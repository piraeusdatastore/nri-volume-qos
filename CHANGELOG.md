# Changelog
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Helm chart under `charts/nri-volume-qos` for deploying the plugin as a DaemonSet, with configurable image, host paths, RBAC, scheduling, and NRI plugin settings.

## [0.1.1] - 2026-06-09

## [0.1.0] - 2026-06-09

### Added

- Initial implementation: an NRI plugin that enforces per-volume IO limits (read/write bandwidth and IOPS) on Kubernetes containers via the Linux cgroup v2 `io.max` interface, driven by QoS metadata on CSI `VolumeAttachment`s. Supports both filesystem and raw block volumes, and runs as a DaemonSet on containerd 1.7+ and CRI-O 1.26+.
- Multi-arch (linux/amd64, linux/arm64) container images, signed with [cosign](https://docs.sigstore.dev/) keyless signing and published from GitHub Actions.

[Unreleased]: https://github.com/piraeusdatastore/nri-volume-qos/compare/v0.1.1...HEAD
[0.1.1]: https://github.com/piraeusdatastore/nri-volume-qos/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/piraeusdatastore/nri-volume-qos/releases/tag/v0.1.0
