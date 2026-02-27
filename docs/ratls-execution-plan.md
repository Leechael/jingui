# RA-TLS Strict Execution Plan

Status: implemented (strict path)
Branch: `feat-ratls-strict`

## Goal

Enable strict, bidirectional RA validation in Jingui secret handshake flow while keeping transport as standard HTTPS.

- HTTPS remains LE/TLS
- Attestation material sourced via dstack guest-agent socket (`/var/run/dstack.sock`)
- Server verifies client RA (dcap-qvl)
- Client verifies server RA (attested cert from challenge response)
- Existing challenge-response remains as key-possession/liveness ACK

## Agreed Flow

1. Client requests `/v1/secrets/challenge` with:
   - `fid`
   - `client_attestation` (`app_cert`, `tcb_info`, optional identity fields)
2. Server verifies client attestation and policy.
3. Server returns challenge plus `server_attestation`.
4. Client verifies server attestation.
5. Client calls `/v1/secrets/fetch` with challenge response.
6. Server requires both RA verification + challenge ACK before release.

## Incremental Work Items

### W1 - Protocol/Types ✅
- [x] Add attestation bundle types
- [x] Extend challenge request/response payloads
- [x] Add strict mode config (`JINGUI_RATLS_STRICT`, default true)

### W2 - Client Collector ✅
- [x] Integrate dstack go SDK `Info()` collector
- [x] Build `client_attestation` payload from `app_cert` + `tcb_info`

### W3 - Server Verifier ✅
- [x] Integrate dcap-qvl and RA cert/quote parser
- [x] Verify strict flow gates and challenge RA state
- [x] Persist challenge verification state in memory store
- [x] Extract/bind app_id from verified attestation certificate extensions (with bundle consistency checks)

### W4 - Client-side Server RA Verify ✅
- [x] Verify `server_attestation` before fetch
- [x] Fail closed in strict mode
- [x] Optional app_id pin via `JINGUI_RATLS_EXPECT_SERVER_APP_ID`

### W5 - Tests + Docs ✅
- [x] Add strict negative tests (missing/mismatch attestation)
- [x] Add strict flow state test (challenge->fetch gate)
- [x] Update README / OpenAPI / manual guide
- [x] CI builds dcap-qvl and runs all tests with RA-TLS linked

## Notes

- `Info` does not expose a dedicated `quote` field; quote is carried in attested certificate extensions (`app_cert`).
- Using `Info` avoids per-request expensive quote generation.
- Challenge ACK provides request-level liveness to complement cached/boot-time attestation materials.
