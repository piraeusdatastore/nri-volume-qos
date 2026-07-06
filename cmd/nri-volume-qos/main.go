package main

import (
	"fmt"
	"os"
	"os/signal"

	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	k8scli "k8s.io/component-base/cli"
	"k8s.io/klog/v2"

	"github.com/piraeusdatastore/nri-volume-qos/pkg/metadata"
	"github.com/piraeusdatastore/nri-volume-qos/pkg/plugin"
)

func newCommand() *cobra.Command {
	var kubeconfig, nodeName, pluginName, pluginIdx, kubeletPodsDir, hostPrefixDir string

	cmd := &cobra.Command{
		Use:     "nri-volume-qos",
		Short:   "NRI plugin that enforces per-volume IO limits via cgroupv2 io.max",
		Version: metadata.Version,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := signal.NotifyContext(cmd.Context(), os.Interrupt)
			defer cancel()

			if nodeName == "" {
				return fmt.Errorf("--node-name is required (or set NODE_NAME)")
			}

			restCfg, err := buildRESTConfig(kubeconfig)
			if err != nil {
				return err
			}

			k8s, err := kubernetes.NewForConfig(restCfg)
			if err != nil {
				return err
			}

			p, err := plugin.New(plugin.Options{
				K8s:            k8s,
				NodeName:       nodeName,
				PluginName:     pluginName,
				PluginIdx:      pluginIdx,
				KubeletPodsDir: kubeletPodsDir,
				HostPrefixDir:  hostPrefixDir,
			})
			if err != nil {
				return err
			}

			klog.InfoS("Starting nri-volume-qos", "version", metadata.Version, "node", nodeName)
			return p.Run(ctx)
		},
	}

	cmd.Flags().StringVar(&kubeconfig, "kubeconfig", "",
		"Path to a kubeconfig file; defaults to in-cluster config when empty")
	cmd.Flags().StringVar(&nodeName, "node-name", os.Getenv("NODE_NAME"),
		"Name of the node this instance runs on (defaults to $NODE_NAME)")
	cmd.Flags().StringVar(&pluginName, "nri-plugin-name", "qos.linbit.com",
		"NRI plugin name")
	cmd.Flags().StringVar(&pluginIdx, "nri-plugin-idx", "90",
		"NRI plugin index; controls ordering relative to other NRI plugins")
	cmd.Flags().StringVar(&kubeletPodsDir, "kubelet-pods-dir", "/var/lib/kubelet/pods",
		"Path to the kubelet pods directory on the host; must match the hostPath volume mount")
	cmd.Flags().StringVar(&hostPrefixDir, "host-prefix-dir", "/host",
		"All lookups of the hosts /dev/... directory are prefixed with this path.")

	return cmd
}

func buildRESTConfig(kubeconfig string) (*rest.Config, error) {
	if kubeconfig != "" {
		return clientcmd.BuildConfigFromFlags("", kubeconfig)
	}
	cfg, err := rest.InClusterConfig()
	if err == nil {
		return cfg, nil
	}
	// Fall back to default kubeconfig for local development.
	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		clientcmd.NewDefaultClientConfigLoadingRules(),
		nil,
	).ClientConfig()
}

func main() {
	os.Exit(k8scli.Run(newCommand()))
}
