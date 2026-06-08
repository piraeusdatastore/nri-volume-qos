FROM --platform=$BUILDPLATFORM golang:1 AS builder
ARG VERSION=0.0.0-dev
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG TARGETOS
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build \
    -ldflags="-X github.com/piraeusdatastore/nri-volume-qos/pkg/metadata.Version=${VERSION}" \
    -o /nri-volume-qos \
    ./cmd/nri-volume-qos

FROM registry.access.redhat.com/ubi9/ubi-micro:latest
COPY --from=builder /nri-volume-qos /nri-volume-qos
ENTRYPOINT ["/nri-volume-qos"]
