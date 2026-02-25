//go:build ratls

package attestation

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"

	dstackratls "github.com/Dstack-TEE/dstack/sdk/go/ratls"
)

// RATLSVerifier verifies attestation bundles using RA-TLS certificate extensions.
type RATLSVerifier struct{}

func NewRATLSVerifier() *RATLSVerifier {
	return &RATLSVerifier{}
}

func (v *RATLSVerifier) Verify(_ context.Context, b Bundle) (VerifiedIdentity, error) {
	if b.AppCert == "" {
		return VerifiedIdentity{}, fmt.Errorf("missing app_cert in attestation bundle")
	}

	cert, err := parseFirstPEMCertificate(b.AppCert)
	if err != nil {
		return VerifiedIdentity{}, err
	}

	if _, err := dstackratls.VerifyCert(cert); err != nil {
		return VerifiedIdentity{}, fmt.Errorf("RA-TLS certificate verification failed: %w", err)
	}

	return VerifiedIdentity{
		AppID:      b.AppID,
		InstanceID: b.Instance,
		DeviceID:   b.DeviceID,
	}, nil
}

func parseFirstPEMCertificate(pemChain string) (*x509.Certificate, error) {
	block, _ := pem.Decode([]byte(pemChain))
	if block == nil {
		return nil, fmt.Errorf("failed to decode certificate PEM")
	}
	if block.Type != "CERTIFICATE" {
		return nil, fmt.Errorf("unexpected PEM block type %q (want CERTIFICATE)", block.Type)
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse certificate: %w", err)
	}
	return cert, nil
}
