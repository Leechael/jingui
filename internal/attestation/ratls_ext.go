package attestation

import (
	"crypto/x509"
	"encoding/asn1"
	"encoding/hex"
	"strings"
)

func extractAppIDFromCert(cert *x509.Certificate) string {
	for _, ext := range cert.Extensions {
		if !ext.Id.Equal(oidRATLSAppID) {
			continue
		}
		var raw []byte
		if _, err := asn1.Unmarshal(ext.Value, &raw); err != nil {
			continue
		}
		if len(raw) == 0 {
			continue
		}
		if isPrintableASCII(raw) {
			return strings.TrimSpace(string(raw))
		}
		return hex.EncodeToString(raw)
	}
	return ""
}

func isPrintableASCII(b []byte) bool {
	for _, c := range b {
		if c < 0x20 || c > 0x7e {
			return false
		}
	}
	return true
}
