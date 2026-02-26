//go:build ratls

package attestation

import (
	"context"
	"crypto/x509"
	"encoding/asn1"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"strings"

	dstackratls "github.com/Dstack-TEE/dstack/sdk/go/ratls"
	"github.com/aspect-build/jingui/internal/logx"
)

// RATLSVerifier verifies attestation bundles using RA-TLS certificate extensions.
type RATLSVerifier struct{}

var oidRATLSAppID = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 62397, 1, 3}

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

	result, err := dstackratls.VerifyCert(cert)
	if err != nil {
		return VerifiedIdentity{}, fmt.Errorf("RA-TLS certificate verification failed: %w", err)
	}

	extractedAppID := extractAppIDFromCert(cert)
	if extractedAppID != "" && strings.TrimSpace(b.AppID) != "" && extractedAppID != strings.TrimSpace(b.AppID) {
		return VerifiedIdentity{}, fmt.Errorf("attestation app_id mismatch between certificate and bundle")
	}
	if extractedAppID == "" {
		extractedAppID = strings.TrimSpace(b.AppID)
	}

	if result != nil {
		logRATLSMeasurements(result)
	}
	logx.Debugf("ratls.identity cert_app_id=%q bundle_app_id=%q resolved_app_id=%q instance_id=%q device_id=%q", extractAppIDFromCert(cert), strings.TrimSpace(b.AppID), extractedAppID, b.Instance, b.DeviceID)

	return VerifiedIdentity{
		AppID:      extractedAppID,
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

func logRATLSMeasurements(result *dstackratls.VerifyResult) {
	report := result.Report
	if report == nil {
		return
	}
	qr := report.Report
	logx.Debugf("ratls.verify status=%s qe_status=%s platform_status=%s advisory_ids=%v", report.Status, report.QEStatus.Status, report.PlatformStatus.Status, report.AdvisoryIDs)
	logx.Debugf("ratls.measurements type=%s mr_td=%s mr_config_id=%s mr_owner=%s mr_owner_config=%s", qr.Type, fmtHex(qr.MrTD), fmtHex(qr.MrConfigID), fmtHex(qr.MrOwner), fmtHex(qr.MrOwnerConfig))
	logx.Debugf("ratls.measurements rtmr0=%s rtmr1=%s rtmr2=%s rtmr3=%s tee_tcb_svn=%s td_attributes=%s", fmtHex(qr.RTMR0), fmtHex(qr.RTMR1), fmtHex(qr.RTMR2), fmtHex(qr.RTMR3), fmtHex(qr.TeeTCBSVN), fmtHex(qr.TdAttributes))
}

func fmtHex(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	if logx.IsDebug() {
		return hex.EncodeToString(b)
	}
	x := hex.EncodeToString(b)
	if len(x) <= 32 {
		return x
	}
	return x[:32] + "..."
}
