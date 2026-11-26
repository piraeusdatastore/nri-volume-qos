# Contributing to nri-volume-qos

The [Piraeus contributing guide](https://github.com/piraeusdatastore/piraeus/blob/master/CONTRIBUTING.md) describes how to report issues, write commits and changelog entries, and open pull requests.
The sections that follow describe what is specific to this repository.

## Target branch

Open pull requests against the `main` branch.

## Build and test

Install [Go](https://go.dev/) in the version declared in `go.mod`.
The Makefile has targets for building, testing, and vetting the code:

```
$ make build
$ make test
$ make vet
```

On every pull request, CI runs `go test ./...`, [`golangci-lint`](https://golangci-lint.run/) on new issues, and a build of the container image.
To run the linter locally:

```
$ golangci-lint run
```

## Deployment manifests

The repository has two deployment methods: the Helm chart in `charts/nri-volume-qos/` and the plain manifest in `deploy/daemonset.yaml`.
Both define the ServiceAccount, the RBAC rules, and the DaemonSet.
If you change one of these resources, for example to add a command-line option or a volume, change both deployment methods in the same pull request.

## Testing on a cluster

To try a change on a cluster, build an image and push it to your registry:

```
$ make push REGISTRY=registry.example.com/myname TAG=dev
```

By default, the image is built for `linux/amd64` and `linux/arm64`.
To build for a single platform, set `PLATFORMS`, for example `PLATFORMS=linux/amd64`.
Replace the image in your deployment with the new image.
