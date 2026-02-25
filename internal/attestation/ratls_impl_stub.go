//go:build !ratls

package attestation

import (
	"context"
	"fmt"
)

// RATLSVerifier stub used when built without `-tags ratls`.
type RATLSVerifier struct{}

func NewRATLSVerifier() *RATLSVerifier {
	return &RATLSVerifier{}
}

func (v *RATLSVerifier) Verify(_ context.Context, _ Bundle) (VerifiedIdentity, error) {
	return VerifiedIdentity{}, fmt.Errorf("RA-TLS verifier unavailable: rebuild with -tags ratls")
}
