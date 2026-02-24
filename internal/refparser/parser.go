package refparser

import (
	"fmt"
	"strings"
)

const refPrefix = "jingui://"

// SecretRef represents a parsed jingui:// reference.
//
// Canonical semantics:
//   jingui://<service>/<slug_or_email>/<field_name>
//
// Transitional aliases (AppID/SecretName) are kept temporarily to reduce
// refactor churn while server/client handlers migrate to Service/Slug naming.
type SecretRef struct {
	Service   string
	Slug      string
	FieldName string
	Raw       string

	// Deprecated aliases; kept during refactor.
	AppID      string
	SecretName string
}

// IsRef returns true if value starts with "jingui://".
func IsRef(value string) bool {
	return strings.HasPrefix(value, refPrefix)
}

// Parse parses a jingui://<service>/<slug_or_email>/<field_name> reference.
func Parse(ref string) (SecretRef, error) {
	if !IsRef(ref) {
		return SecretRef{}, fmt.Errorf("not a jingui reference: %q", ref)
	}

	body := strings.TrimPrefix(ref, refPrefix)
	parts := strings.SplitN(body, "/", 3)
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return SecretRef{}, fmt.Errorf("invalid jingui reference %q: expected jingui://<service>/<slug_or_email>/<field_name>", ref)
	}

	return SecretRef{
		Service:    parts[0],
		Slug:       parts[1],
		FieldName:  parts[2],
		Raw:        ref,
		AppID:      parts[0],
		SecretName: parts[1],
	}, nil
}
