package refparser

import (
	"fmt"
	"strings"
)

var refPrefixes = []string{"jingui://", "op://"}

// SecretRef represents a parsed jingui:// (or op://) reference.
//
// Canonical semantics:
//
//	jingui://<vault>/<item>/<field_name>
//	jingui://<vault>/<item>/<section>/<field_name>
//	op://<vault>/<item>/<field_name>
//	op://<vault>/<item>/<section>/<field_name>
type SecretRef struct {
	Vault     string
	Item      string
	Section   string // optional, empty if 3-segment URI
	FieldName string
	Raw       string
}

// IsRef returns true if value starts with "jingui://" or "op://".
func IsRef(value string) bool {
	for _, p := range refPrefixes {
		if strings.HasPrefix(value, p) {
			return true
		}
	}
	return false
}

// matchedPrefix returns the prefix that ref starts with, or "" if none match.
func matchedPrefix(ref string) string {
	for _, p := range refPrefixes {
		if strings.HasPrefix(ref, p) {
			return p
		}
	}
	return ""
}

// Parse parses a jingui://<vault>/<item>/<field_name> (or op://…) or
// jingui://<vault>/<item>/<section>/<field_name> (or op://…) reference.
func Parse(ref string) (SecretRef, error) {
	prefix := matchedPrefix(ref)
	if prefix == "" {
		return SecretRef{}, fmt.Errorf("not a secret reference: %q", ref)
	}

	body := strings.TrimPrefix(ref, prefix)
	parts := strings.Split(body, "/")

	switch len(parts) {
	case 3:
		if parts[0] == "" || parts[1] == "" || parts[2] == "" {
			return SecretRef{}, fmt.Errorf("invalid reference %q: expected %s<vault>/<item>/<field_name>", ref, prefix)
		}
		return SecretRef{
			Vault:     parts[0],
			Item:      parts[1],
			FieldName: parts[2],
			Raw:       ref,
		}, nil
	case 4:
		if parts[0] == "" || parts[1] == "" || parts[2] == "" || parts[3] == "" {
			return SecretRef{}, fmt.Errorf("invalid reference %q: expected %s<vault>/<item>/<section>/<field_name>", ref, prefix)
		}
		return SecretRef{
			Vault:     parts[0],
			Item:      parts[1],
			Section:   parts[2],
			FieldName: parts[3],
			Raw:       ref,
		}, nil
	default:
		return SecretRef{}, fmt.Errorf("invalid reference %q: expected 3 or 4 path segments", ref)
	}
}
