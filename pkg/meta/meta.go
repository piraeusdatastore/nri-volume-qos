// Package meta defines the VolumeAttachment metadata keys and types that form
// the contract between a CSI driver and nri-volume-qos.
//
// A CSI driver opts a volume into IO QoS by including the keys defined here in
// the publish_context map returned from ControllerPublishVolume. Kubernetes
// stores that map verbatim in VolumeAttachment.status.attachmentMetadata, where
// nri-volume-qos reads it on every container start.
package meta

import (
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/api/resource"
)

// Metadata keys written by the CSI driver into ControllerPublishVolume
// publish_context (and read back from VolumeAttachment.status.attachmentMetadata).
const (
	KeyDevice = "qos.linbit.com/device"
	KeyRBPS   = "qos.linbit.com/rbps"
	KeyWBPS   = "qos.linbit.com/wbps"
	KeyRIOPS  = "qos.linbit.com/riops"
	KeyWIOPS  = "qos.linbit.com/wiops"
)

// Limits describes the IO constraints for a volume.
type Limits struct {
	// Device is the absolute host path of the block device (e.g. /dev/drbd1000).
	// Required — volumes without this key are ignored by nri-volume-qos.
	Device string
	// RBPS and WBPS cap read/write throughput in bytes per second. Zero means unlimited.
	RBPS, WBPS uint64
	// RIOPS and WIOPS cap read/write IOPS. Zero means unlimited.
	RIOPS, WIOPS uint64
}

// ToMap serialises the limits to a publish_context / attachmentMetadata map.
// Zero-valued bandwidth and IOPS fields are omitted; the caller is responsible
// for ensuring Device is non-empty before publishing.
func (l Limits) ToMap() map[string]string {
	m := map[string]string{KeyDevice: l.Device}
	if l.RBPS > 0 {
		m[KeyRBPS] = strconv.FormatUint(l.RBPS, 10)
	}
	if l.WBPS > 0 {
		m[KeyWBPS] = strconv.FormatUint(l.WBPS, 10)
	}
	if l.RIOPS > 0 {
		m[KeyRIOPS] = strconv.FormatUint(l.RIOPS, 10)
	}
	if l.WIOPS > 0 {
		m[KeyWIOPS] = strconv.FormatUint(l.WIOPS, 10)
	}
	return m
}

// FromMap parses IO limits from a VolumeAttachment attachmentMetadata map.
// Returns (zero, false) when the device key is absent or no bandwidth/IOPS
// limit keys are present. Values may be plain integers ("104857600") or
// Kubernetes quantity strings ("100Mi", "1Gi", "500k"); unparseable values
// are treated as absent.
func FromMap(m map[string]string) (Limits, bool) {
	device, ok := m[KeyDevice]
	if !ok {
		return Limits{}, false
	}
	l := Limits{Device: device}
	var found bool
	if v, ok := parseUint(m[KeyRBPS]); ok {
		l.RBPS = v
		found = true
	}
	if v, ok := parseUint(m[KeyWBPS]); ok {
		l.WBPS = v
		found = true
	}
	if v, ok := parseUint(m[KeyRIOPS]); ok {
		l.RIOPS = v
		found = true
	}
	if v, ok := parseUint(m[KeyWIOPS]); ok {
		l.WIOPS = v
		found = true
	}
	return l, found
}

// parseUint accepts either a plain integer ("104857600") or a Kubernetes
// quantity string ("100Mi", "1Gi", "500k") and returns the value in base units.
func parseUint(s string) (uint64, bool) {
	if s == "" {
		return 0, false
	}
	q, err := resource.ParseQuantity(strings.TrimSpace(s))
	if err != nil {
		return 0, false
	}
	v := q.Value()
	if v < 0 {
		return 0, false
	}
	return uint64(v), true
}
