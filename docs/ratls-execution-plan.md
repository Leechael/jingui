# RA-TLS Strict Execution Plan

Status: draft-in-progress
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

### W1 - Protocol/Types (in progress)
- Add attestation bundle types
- Extend challenge request/response payloads
- Add strict mode config (`JINGUI_RATLS_STRICT`, default true)

### W2 - Client Collector
- Integrate dstack go SDK `Info()` collector
- Build `client_attestation` payload from `app_cert` + `tcb_info`

### W3 - Server Verifier
- Integrate dcap-qvl and RA cert/quote parser
- Verify identity/policy extraction
- Persist challenge verification state in memory store

### W4 - Client-side Server RA Verify
- Verify `server_attestation` before fetch
- Fail closed in strict mode

### W5 - Tests + Docs
- Unit and integration tests for positive/negative paths
- Update README / OpenAPI / manual guide

## Notes

- `Info` does not expose a dedicated `quote` field; quote is carried in attested certificate extensions (`app_cert`).
- Using `Info` avoids per-request expensive quote generation.
- Challenge ACK provides request-level liveness to complement cached/boot-time attestation materials.
