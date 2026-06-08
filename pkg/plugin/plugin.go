package plugin

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/containerd/nri/pkg/api"
	"github.com/containerd/nri/pkg/stub"
	"golang.org/x/sys/unix"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

	"github.com/piraeusdatastore/nri-volume-qos/pkg/meta"
)

const vaByNodeAndPV = "vaByNodeAndPV"

// Options configures the Plugin.
type Options struct {
	K8s            kubernetes.Interface
	NodeName       string
	PluginName     string
	PluginIdx      string
	KubeletPodsDir string
	HostPrefixDir  string
}

// Plugin is an NRI plugin that applies cgroupv2 io.max limits to containers
// whose CSI volumes carry IO limit metadata in their VolumeAttachment.
type Plugin struct {
	stub            stub.Stub
	nodeName        string
	kubeletPodsDir  string
	hostPrefixDir   string
	vaIndexer       cache.Indexer
	vasSynced       cache.InformerSynced
	informerFactory informers.SharedInformerFactory
}

// ioLimit holds a resolved per-device IO limit ready to be written to io.max.
type ioLimit struct {
	major, minor uint32
	rbps, wbps   uint64
	riops, wiops uint64
}

// New creates and registers the NRI plugin stub.
func New(opts Options) (*Plugin, error) {
	factory := informers.NewSharedInformerFactory(opts.K8s, 0)
	vaInformer := factory.Storage().V1().VolumeAttachments()
	if err := vaInformer.Informer().AddIndexers(cache.Indexers{vaByNodeAndPV: indexVAByNodeAndPV}); err != nil {
		return nil, fmt.Errorf("add VA indexer: %w", err)
	}

	p := &Plugin{
		nodeName:        opts.NodeName,
		kubeletPodsDir:  opts.KubeletPodsDir,
		hostPrefixDir:   opts.HostPrefixDir,
		vaIndexer:       vaInformer.Informer().GetIndexer(),
		vasSynced:       vaInformer.Informer().HasSynced,
		informerFactory: factory,
	}

	s, err := stub.New(p,
		stub.WithPluginName(opts.PluginName),
		stub.WithPluginIdx(opts.PluginIdx),
	)
	if err != nil {
		return nil, fmt.Errorf("create NRI stub: %w", err)
	}
	p.stub = s
	return p, nil
}

// Run starts the informer cache, waits for it to sync, then starts the NRI
// plugin and blocks until ctx is cancelled or the runtime disconnects.
func (p *Plugin) Run(ctx context.Context) error {
	p.informerFactory.Start(ctx.Done())

	klog.InfoS("Waiting for VolumeAttachment informer cache to sync")
	if !cache.WaitForCacheSync(ctx.Done(), p.vasSynced) {
		return fmt.Errorf("VolumeAttachment informer cache failed to sync")
	}
	klog.InfoS("VolumeAttachment informer cache synced")

	if err := p.stub.Run(ctx); err != nil && ctx.Err() == nil {
		return err
	}
	return nil
}

// CreateContainer implements stub.CreateContainerInterface.
//
// The returned ContainerAdjustment carries io.max entries in the cgroupv2
// Unified map; the runtime writes them to the container cgroup at creation
// time, so we never touch the cgroup filesystem directly.
func (p *Plugin) CreateContainer(ctx context.Context, pod *api.PodSandbox, container *api.Container) (*api.ContainerAdjustment, []*api.ContainerUpdate, error) {
	limits := p.resolveIOLimits(ctx, pod, container)
	if len(limits) == 0 {
		return nil, nil, nil
	}

	var lines []string
	for _, l := range limits {
		lines = append(lines, formatIOMaxEntry(l))
	}

	adj := &api.ContainerAdjustment{}
	adj.AddLinuxUnified("io.max", strings.Join(lines, "\n"))

	klog.V(4).InfoS("Adjusting io.max",
		"pod", klog.KRef(pod.GetNamespace(), pod.GetName()),
		"container", container.GetName(),
		"entries", len(limits))

	return adj, nil, nil
}

// resolveIOLimits walks the kubelet pod volume directories to discover all CSI
// volumes for the pod — both filesystem mounts and raw block devices — and
// resolves IO limits from the corresponding VolumeAttachment for each.
//
// Walking the kubelet directory rather than inspecting container.GetMounts()
// is necessary because raw block-device volumes appear only in the NRI device
// section without any CSI-identifying information.
func (p *Plugin) resolveIOLimits(ctx context.Context, pod *api.PodSandbox, container *api.Container) []ioLimit {
	log := klog.LoggerWithValues(klog.Background(),
		"pod", klog.KRef(pod.GetNamespace(), pod.GetName()),
		"container", container.GetName())

	podDir := filepath.Join(p.kubeletPodsDir, pod.GetUid())
	log.V(4).Info("Scanning kubelet pod directory", "podDir", podDir)

	var limits []ioLimit
	for _, csiDir := range []string{
		filepath.Join(podDir, "volumes", "kubernetes.io~csi"),
		filepath.Join(podDir, "volumeDevices", "kubernetes.io~csi"),
	} {
		limits = append(limits, p.scanCSIDir(log, csiDir)...)
	}

	log.V(4).Info("Finished resolving IO limits", "count", len(limits))
	return limits
}

// scanCSIDir reads entries from a kubernetes.io~csi volume directory and
// resolves IO limits for each entry found. The kubelet names both filesystem
// mount directories (.../volumes/kubernetes.io~csi/<pvName>/) and block-device
// symlinks (.../volumeDevices/kubernetes.io~csi/<pvName>) after the PV name,
// so we can look up the VolumeAttachment directly from the index.
func (p *Plugin) scanCSIDir(log klog.Logger, dir string) []ioLimit {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if !os.IsNotExist(err) {
			log.V(4).Error(err, "Failed to read CSI directory", "dir", dir)
		}
		return nil
	}

	var limits []ioLimit
	for _, entry := range entries {
		pvName := entry.Name()
		log.V(4).Info("Found CSI volume entry", "pvName", pvName)

		va, err := p.lookupVAByPVName(pvName)
		if err != nil {
			log.V(4).Error(err, "Could not look up VolumeAttachment", "pvName", pvName)
			continue
		}
		log.V(4).Info("Resolved VolumeAttachment", "pvName", pvName, "attachmentID", va.Name)

		ml, ok := meta.FromMap(va.Status.AttachmentMetadata)
		if !ok {
			log.V(4).Info("VolumeAttachment has no QoS metadata, skipping", "attachmentID", va.Name)
			continue
		}
		log.V(4).Info("Found QoS metadata in VolumeAttachment", "attachmentID", va.Name, "device", ml.Device)

		major, minor, err := deviceMajorMinor(path.Join(p.hostPrefixDir, ml.Device))
		if err != nil {
			log.Error(err, "Failed to stat device", "device", ml.Device, "attachmentID", va.Name)
			continue
		}

		limit := ioLimit{
			major: major, minor: minor,
			rbps: ml.RBPS, wbps: ml.WBPS,
			riops: ml.RIOPS, wiops: ml.WIOPS,
		}
		limits = append(limits, limit)

		log.V(4).Info("Resolved IO limit",
			"attachmentID", va.Name,
			"device", fmt.Sprintf("%d:%d", major, minor),
			"rbps", ml.RBPS, "wbps", ml.WBPS,
			"riops", ml.RIOPS, "wiops", ml.WIOPS)
	}

	return limits
}

// lookupVAByPVName finds the VolumeAttachment for pvName on the current node
// using the vaByNodeAndPV index for O(1) lookup.
func (p *Plugin) lookupVAByPVName(pvName string) (*storagev1.VolumeAttachment, error) {
	objs, err := p.vaIndexer.ByIndex(vaByNodeAndPV, p.nodeName+"/"+pvName)
	if err != nil {
		return nil, fmt.Errorf("VA index lookup: %w", err)
	}
	if len(objs) == 0 {
		return nil, fmt.Errorf("no VolumeAttachment for PV %q on node %q", pvName, p.nodeName)
	}
	return objs[0].(*storagev1.VolumeAttachment), nil
}

// indexVAByNodeAndPV is a cache.IndexFunc that keys VolumeAttachments by
// "nodeName/pvName" for O(1) lookup.
func indexVAByNodeAndPV(obj interface{}) ([]string, error) {
	va, ok := obj.(*storagev1.VolumeAttachment)
	if !ok {
		return nil, fmt.Errorf("unexpected type %T", obj)
	}
	if va.Spec.Source.PersistentVolumeName == nil {
		return nil, nil
	}
	return []string{va.Spec.NodeName + "/" + *va.Spec.Source.PersistentVolumeName}, nil
}

// deviceMajorMinor returns the major and minor device numbers for path.
func deviceMajorMinor(path string) (uint32, uint32, error) {
	var st unix.Stat_t
	if err := unix.Stat(path, &st); err != nil {
		return 0, 0, err
	}
	return unix.Major(st.Rdev), unix.Minor(st.Rdev), nil
}

// formatIOMaxEntry builds a single line suitable for writing to io.max, e.g.:
//
//	"8:0 rbps=104857600 wbps=52428800 riops=1000 wiops=500"
//
// Dimensions with a zero value are omitted (kernel treats absent fields as
// unchanged, preserving any existing limit or "max").
func formatIOMaxEntry(l ioLimit) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%d:%d", l.major, l.minor)
	if l.rbps > 0 {
		fmt.Fprintf(&b, " rbps=%d", l.rbps)
	}
	if l.wbps > 0 {
		fmt.Fprintf(&b, " wbps=%d", l.wbps)
	}
	if l.riops > 0 {
		fmt.Fprintf(&b, " riops=%d", l.riops)
	}
	if l.wiops > 0 {
		fmt.Fprintf(&b, " wiops=%d", l.wiops)
	}
	return b.String()
}
