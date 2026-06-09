// Package meta defines the VolumeAttachment metadata keys and types that form
// the contract between a CSI driver and nri-volume-qos.
//
// A CSI driver opts a volume into IO QoS by including the keys defined here in
// the publish_context map returned from ControllerPublishVolume. Kubernetes
// stores that map verbatim in VolumeAttachment.status.attachmentMetadata, where
// nri-volume-qos reads it on every container start.
package meta

import (
	"errors"
	"fmt"
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
	if v, ok, _ := parseLimit(m[KeyRBPS]); ok {
		l.RBPS = v
		found = true
	}
	if v, ok, _ := parseLimit(m[KeyWBPS]); ok {
		l.WBPS = v
		found = true
	}
	if v, ok, _ := parseLimit(m[KeyRIOPS]); ok {
		l.RIOPS = v
		found = true
	}
	if v, ok, _ := parseLimit(m[KeyWIOPS]); ok {
		l.WIOPS = v
		found = true
	}
	return l, found
}

// ParseLimits parses the qos.linbit.com/* IO limits from a parameter map, such
// as a StorageClass's parameters. Unlike FromMap it is strict: a limit that is
// present but cannot be parsed (or is negative) returns an error. The device
// key is set per-node at publish time rather than in the StorageClass, so it is
// ignored here; the returned Limits leaves Device empty for the caller to fill
// in before calling ToMap. Values may be plain integers ("104857600") or
// Kubernetes quantity strings ("100Mi", "1Gi", "500k").
func ParseLimits(params map[string]string) (Limits, error) {
	var l Limits

	for _, f := range []struct {
		key string
		dst *uint64
	}{
		{KeyRBPS, &l.RBPS},
		{KeyWBPS, &l.WBPS},
		{KeyRIOPS, &l.RIOPS},
		{KeyWIOPS, &l.WIOPS},
	} {
		v, _, err := parseLimit(params[f.key])
		if err != nil {
			return Limits{}, fmt.Errorf("invalid value %q for %s: %w", params[f.key], f.key, err)
		}
		*f.dst = v
	}

	return l, nil
}

// parseLimit parses a single qos.linbit.com limit value. ok reports whether a
// usable value was present: an empty string yields (0, false, nil). A non-empty
// value must be a non-negative integer or Kubernetes quantity string ("100Mi",
// "1Gi", "500k"), otherwise (0, false, err) is returned.
func parseLimit(s string) (uint64, bool, error) {
	if s == "" {
		return 0, false, nil
	}
	q, err := resource.ParseQuantity(strings.TrimSpace(s))
	if err != nil {
		return 0, false, err
	}
	v := q.Value()
	if v < 0 {
		return 0, false, errors.New("must not be negative")
	}
	return uint64(v), true, nil
}
