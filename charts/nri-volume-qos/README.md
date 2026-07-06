# nri-volume-qos Helm chart

Deploys [`nri-volume-qos`](https://github.com/piraeusdatastore/nri-volume-qos) — an
NRI plugin that enforces per-volume IO limits on Kubernetes containers via the
Linux cgroup v2 `io.max` interface — as a DaemonSet.

## Prerequisites

| Component | Minimum version |
|---|---|
| Kubernetes | 1.26 |
| containerd | 1.7 (NRI enabled in `config.toml`) |
| CRI-O | 1.26 |
| Linux kernel | 5.14 (cgroup v2 IO controller) |

NRI must be enabled in the runtime. For containerd, add to `/etc/containerd/config.toml`:

```toml
[plugins."io.containerd.nri.v1.nri"]
  disable = false
```

## Installing

Install the published chart from the GitHub Container Registry (OCI), pinning
the version you want:

```console
helm install nri-volume-qos \
  oci://ghcr.io/piraeusdatastore/nri-volume-qos \
  --version <X.Y.Z> \
  --namespace kube-system
```

Or install from a checkout of this repository:

```console
helm install nri-volume-qos ./charts/nri-volume-qos \
  --namespace kube-system
```

## Uninstalling

```console
helm uninstall nri-volume-qos --namespace kube-system
```

## How limits are supplied

The plugin only *enforces* limits; it does not compute them. Your CSI driver
publishes the limit values as `qos.linbit.com/*` keys in
`VolumeAttachment.status.attachmentMetadata`:

| Key | Required | Description |
|---|---|---|
| `qos.linbit.com/device` | yes | Absolute path to the block device on the node |
| `qos.linbit.com/rbps` | no | Maximum read bandwidth in bytes/s |
| `qos.linbit.com/wbps` | no | Maximum write bandwidth in bytes/s |
| `qos.linbit.com/riops` | no | Maximum read IOPS |
| `qos.linbit.com/wiops` | no | Maximum write IOPS |

## Values

| Key | Default | Description |
|---|---|---|
| `image.registry` | `quay.io` | Image registry. Set empty to omit. |
| `image.repository` | `piraeusdatastore/nri-volume-qos` | Image repository. |
| `image.tag` | `""` | Image tag; defaults to the chart `appVersion`. |
| `image.pullPolicy` | `IfNotPresent` | Image pull policy. |
| `imagePullSecrets` | `[]` | Secrets for pulling from a private registry. |
| `nameOverride` | `""` | Override the chart name. |
| `fullnameOverride` | `""` | Override the fully qualified resource name. |
| `serviceAccount.create` | `true` | Create a ServiceAccount. |
| `serviceAccount.name` | `""` | ServiceAccount name; generated when empty. |
| `serviceAccount.annotations` | `{}` | Annotations for the ServiceAccount. |
| `rbac.create` | `true` | Create the ClusterRole and ClusterRoleBinding. |
| `plugin.name` | `qos.linbit.com` | NRI plugin name (`--nri-plugin-name`). |
| `plugin.index` | `"90"` | NRI plugin index (`--nri-plugin-idx`). |
| `hostPaths.kubeletPodsDir` | `/var/lib/kubelet/pods` | Host kubelet pods directory. |
| `hostPaths.dev` | `/dev` | Host `/dev` directory. |
| `hostPaths.nriSocketDir` | `/var/run/nri` | Host NRI socket directory. |
| `hostPrefixDir` | `/host` | In-container prefix for host `/dev` (`--host-prefix-dir`). |
| `extraArgs` | `[]` | Extra CLI flags appended to the plugin command. |
| `updateStrategy` | `{type: RollingUpdate}` | DaemonSet update strategy. |
| `priorityClassName` | `system-node-critical` | Pod priority class. |
| `tolerations` | tolerate all taints | Pod tolerations. |
| `nodeSelector` | `{}` | Pod node selector. |
| `affinity` | `{}` | Pod affinity. |
| `resources` | `{}` | Container resource requests/limits. |
| `podSecurityContext` | `{}` | Pod security context. |
| `securityContext` | `{privileged: true}` | Container security context. |
| `podAnnotations` | `{}` | Pod annotations. |
| `podLabels` | `{}` | Extra pod labels. |
| `commonLabels` | `{}` | Labels applied to all resources. |
| `extraEnv` | `[]` | Extra environment variables. |
| `extraVolumes` | `[]` | Extra volumes. |
| `extraVolumeMounts` | `[]` | Extra volume mounts. |

### Non-default kubelet directory

On distributions that place the kubelet directory elsewhere, override the host
path — it is used both for the hostPath volume and the `--kubelet-pods-dir` flag:

```yaml
hostPaths:
  kubeletPodsDir: /var/lib/k0s/kubelet/pods
```
