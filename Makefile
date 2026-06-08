PROJECT  ?= nri-volume-qos
REGISTRY ?= quay.io/piraeusdatastore
PLATFORMS ?= linux/amd64,linux/arm64
VERSION  ?= $(shell git describe --tags --match "v*.*" HEAD 2>/dev/null || echo "0.0.0-dev")
TAG      ?= $(VERSION)
NOCACHE  ?= false

.PHONY: help
help:
	@echo "Targets: build, image, push, test, vet"

.PHONY: build
build:
	CGO_ENABLED=0 go build \
		-ldflags="-X github.com/piraeusdatastore/nri-volume-qos/pkg/metadata.Version=$(VERSION)" \
		-o bin/nri-volume-qos \
		./cmd/nri-volume-qos

.PHONY: test
test:
	go test ./...

.PHONY: vet
vet:
	go vet ./...

.PHONY: image
image:
	docker buildx build $(_EXTRA_ARGS) \
		--build-arg=VERSION=$(VERSION) \
		--platform=$(PLATFORMS) \
		--no-cache=$(NOCACHE) \
		--pull=$(NOCACHE) \
		--tag $(REGISTRY)/$(PROJECT):$(TAG) \
		.

.PHONY: push
push:
	$(MAKE) image _EXTRA_ARGS=--push
