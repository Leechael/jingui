package refparser

import (
	"fmt"
	"strings"
)

const refPrefix = "jingui://"

// SecretRef represents a parsed jingui:// reference.
type SecretRef struct {
	AppID      string
	SecretName string
	FieldName  string
	Raw        string
}

// IsRef returns true if value starts with "jingui://".
func IsRef(value string) bool {
	return strings.HasPrefix(value, refPrefix)
}

// Parse parses a jingui://<app_id>/<secret_name>/<field_name> reference.
func Parse(ref string) (SecretRef, error) {
	if !IsRef(ref) {
		return SecretRef{}, fmt.Errorf("not a jingui reference: %q", ref)
	}

	body := strings.TrimPrefix(ref, refPrefix)
	parts := strings.SplitN(body, "/", 3)
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return SecretRef{}, fmt.Errorf("invalid jingui reference %q: expected jingui://<app_id>/<secret_name>/<field_name>", ref)
	}

	return SecretRef{
		AppID:      parts[0],
		SecretName: parts[1],
		FieldName:  parts[2],
		Raw:        ref,
	}, nil
}
