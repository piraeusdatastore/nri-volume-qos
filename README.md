# nri-volume-qos

An [NRI] (Node Resource Interface) plugin that enforces per-volume IO limits on Kubernetes containers using the
Linux cgroupv2 `io.max` interface.

## Overview

`nri-volume-qos` runs as a DaemonSet on every node and hooks into the container runtime (containerd 1.7+ or CRI-O 1.26+)
via NRI. When a container starts, the plugin checks whether any of its CSI volumes — both filesystem mounts and raw
block devices — carry IO limit metadata in their `VolumeAttachment`. If they do, it writes the limits to the
container's cgroupv2 `io.max` file after the cgroup has been created but before the container process starts, so
there is no window where the limits are absent.

### CSI driver interface

`nri-volume-qos` defines a cooperative interface with the CSI driver: when the driver's `ControllerPublishVolume` returns,
its `publish_context` map is stored by Kubernetes in `VolumeAttachment.status.attachmentMetadata`. The plugin reads the
following keys from that map:

| Key | Required | Description |
|---|---|---|
| `qos.linbit.com/device` | yes | Absolute path to the block device on the node (e.g. `/dev/drbd1000`) |
| `qos.linbit.com/rbps` | no | Maximum read bandwidth in bytes/s |
| `qos.linbit.com/wbps` | no | Maximum write bandwidth in bytes/s |
| `qos.linbit.com/riops` | no | Maximum read IOPS |
| `qos.linbit.com/wiops` | no | Maximum write IOPS |

If `qos.linbit.com/device` is absent the volume is ignored. Omitting a limit key leaves that dimension unlimited.
Limits take effect the next time a container mounting the volume is (re)started.

The CSI driver is responsible for computing the limit values — from PVC annotations, StorageClass parameters, or any
other source. `nri-volume-qos` has no opinion on where they come from.

## Requirements

| Component | Minimum version |
|---|---|
| Kubernetes | 1.26 |
| containerd | 1.7 (NRI must be enabled in `config.toml`) |
| CRI-O | 1.26 |
| Linux kernel | 5.14 (cgroupv2 IO controller) |

NRI support must be explicitly enabled in containerd. Add the following to `/etc/containerd/config.toml`:

```toml
[plugins."io.containerd.nri.v1.nri"]
  disable = false
```

## Deployment

### Helm

Install the chart published to the GitHub Container Registry:

```
helm install nri-volume-qos oci://ghcr.io/piraeusdatastore/nri-volume-qos \
  --version <X.Y.Z> --namespace kube-system
```

Or from a checkout of this repository:

```
helm install nri-volume-qos ./charts/nri-volume-qos --namespace kube-system
```

See [`charts/nri-volume-qos/README.md`](charts/nri-volume-qos/README.md) for the full list of values, including how to
adjust the host kubelet directory for distributions that don't use `/var/lib/kubelet/pods`.

### Plain manifest

```
kubectl apply -f deploy/daemonset.yaml
```

Both methods create a `ServiceAccount`, `ClusterRole`, `ClusterRoleBinding`, and a `DaemonSet` in the `kube-system`
namespace. The DaemonSet requires read access to `volumeattachments` and mounts three host paths:

| Host path | Mount | Purpose |
|---|---|---|
| `/var/lib/kubelet/pods` | read-only | Discover CSI volumes and read `vol_data.json` |
| `/dev` | read-only | Stat block devices to resolve `major:minor` numbers |
| `/var/run/nri` | read-write | NRI socket exposed by the container runtime |

## Configuration

The plugin binary accepts the following flags:

```
--node-name string          Name of the node this instance runs on (defaults to $NODE_NAME)
--kubelet-pods-dir string   Path to the kubelet pods directory on the host (default "/var/lib/kubelet/pods")
--nri-plugin-name string    NRI plugin name (default "qos.linbit.com")
--nri-plugin-idx string     NRI plugin index; controls ordering relative to other NRI plugins (default "90")
--kubeconfig string         Path to a kubeconfig file; defaults to in-cluster config when empty
```

## How it works

1. At startup the plugin establishes a watch on `VolumeAttachment` objects and waits for the local cache to sync
   before accepting NRI events. All subsequent lookups are served from this in-process cache with no API server
   round-trips.

2. On every `CreateContainer` event the plugin walks the kubelet pod directory for the pod's UID:
   - `.../pods/<uid>/volumes/kubernetes.io~csi/` — filesystem CSI volumes. Each subdirectory contains a
     `vol_data.json` file written by the kubelet with the `attachmentID` of the corresponding `VolumeAttachment`.
   - `.../pods/<uid>/volumeDevices/kubernetes.io~csi/` — raw block-device CSI volumes. Each entry is a symlink
     whose name is the PV name; the matching `VolumeAttachment` is found by scanning the cache for a VA on this
     node with that PV name.

3. For each CSI volume found, the plugin reads IO limit metadata from the cached `VolumeAttachment`, resolves the
   block device `major:minor` by `stat`-ing the device path, and accumulates `io.max` entries.

4. The plugin returns a `ContainerAdjustment` with `Unified["io.max"]` set to the collected entries. The container
   runtime applies them to the container cgroup at creation time.

## Building

```
make build          # produces bin/nri-volume-qos
make image          # builds a multi-arch container image
make push           # builds and pushes the image
```

[NRI]: https://github.com/containerd/nri
