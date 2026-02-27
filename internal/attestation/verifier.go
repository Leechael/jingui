package attestation

import "context"

// Verifier validates attestation bundles in strict mode.
//
// Concrete implementation will use dcap-qvl + dstack RA-TLS parsing.
type Verifier interface {
	Verify(ctx context.Context, b Bundle) (VerifiedIdentity, error)
}

// Collector fetches local attestation bundle from the TEE runtime.
//
// Concrete implementation will use dstack go SDK Info() against
// /var/run/dstack.sock.
type Collector interface {
	Collect(ctx context.Context) (Bundle, error)
}
