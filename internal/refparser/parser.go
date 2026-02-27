package refparser

import (
	"fmt"
	"strings"
)

const refPrefix = "jingui://"

// SecretRef represents a parsed jingui:// reference.
//
// Canonical semantics:
//
//	jingui://<vault>/<item>/<field_name>
//	jingui://<vault>/<item>/<section>/<field_name>
type SecretRef struct {
	Vault     string
	Item      string
	Section   string // optional, empty if 3-segment URI
	FieldName string
	Raw       string
}

// IsRef returns true if value starts with "jingui://".
func IsRef(value string) bool {
	return strings.HasPrefix(value, refPrefix)
}

// Parse parses a jingui://<vault>/<item>/<field_name> or
// jingui://<vault>/<item>/<section>/<field_name> reference.
func Parse(ref string) (SecretRef, error) {
	if !IsRef(ref) {
		return SecretRef{}, fmt.Errorf("not a jingui reference: %q", ref)
	}

	body := strings.TrimPrefix(ref, refPrefix)
	parts := strings.Split(body, "/")

	switch len(parts) {
	case 3:
		if parts[0] == "" || parts[1] == "" || parts[2] == "" {
			return SecretRef{}, fmt.Errorf("invalid jingui reference %q: expected jingui://<vault>/<item>/<field_name>", ref)
		}
		return SecretRef{
			Vault:     parts[0],
			Item:      parts[1],
			FieldName: parts[2],
			Raw:       ref,
		}, nil
	case 4:
		if parts[0] == "" || parts[1] == "" || parts[2] == "" || parts[3] == "" {
			return SecretRef{}, fmt.Errorf("invalid jingui reference %q: expected jingui://<vault>/<item>/<section>/<field_name>", ref)
		}
		return SecretRef{
			Vault:     parts[0],
			Item:      parts[1],
			Section:   parts[2],
			FieldName: parts[3],
			Raw:       ref,
		}, nil
	default:
		return SecretRef{}, fmt.Errorf("invalid jingui reference %q: expected 3 or 4 path segments", ref)
	}
}
