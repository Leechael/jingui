//go:build !ratls

package attestation

import (
	"context"
	"fmt"
)

// RATLSAvailable reports whether RA-TLS verification is compiled in.
func RATLSAvailable() bool { return false }

// RATLSVerifier stub used when built without `-tags ratls`.
type RATLSVerifier struct{}

func NewRATLSVerifier() *RATLSVerifier {
	return &RATLSVerifier{}
}

func (v *RATLSVerifier) Verify(_ context.Context, _ Bundle) (VerifiedIdentity, error) {
	return VerifiedIdentity{}, fmt.Errorf("RA-TLS verifier unavailable: rebuild with -tags ratls")
}
