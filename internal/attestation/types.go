package attestation

// Bundle carries attestation material exchanged between client and server.
//
// Current source of truth on client side is dstack Info() from /var/run/dstack.sock.
// app_cert is expected to carry RA quote extensions.
type Bundle struct {
	AppCert  string `json:"app_cert"`
	TCBInfo  string `json:"tcb_info"`
	AppID    string `json:"app_id,omitempty"`
	Instance string `json:"instance_id,omitempty"`
	DeviceID string `json:"device_id,omitempty"`
}

// VerifiedIdentity is the normalized identity extracted from a verified
// attestation bundle.
type VerifiedIdentity struct {
	AppID      string
	InstanceID string
	DeviceID   string
}
