## v0.1.3 (2026-02-27)

- fix: restore 403 for vault mismatch in BDD test
- fix: update integration test to expect 404 for wrong item
- docs: add op:// scheme to docs, fix status codes in API docs
- fix: return 404 instead of 403 for item/field/vault not found
- feat: update release command with progress comment lifecycle

## v0.1.3-beta.1 (2026-02-27)

- feat: add op:// URI scheme as alias for jingui://
- Release v0.1.2
- docs: add PUT /v1/instances/:fid to all docs, clarify FID meaning

## v0.1.2 (2026-02-27)

- docs: add PUT /v1/instances/:fid to all docs, clarify FID meaning

## v0.1.2-alpha.9 (2026-02-27)

- feat: add PUT /v1/instances/:fid and fix hex bound_attestation_app_id examples
- docs: fix stale field names and complete OpenAPI schemas

## v0.1.2-alpha.8 (2026-02-27)

- build: remove ratls build tag stub, always link dcap-qvl
- fix: fail early with clear error when ratls strict mode lacks ratls build tag

## v0.1.2-alpha.7 (2026-02-27)

- fix: pin SQLite connection for PRAGMA + migration atomicity
- security: harden RA-TLS attestation identity binding
- refactor: rename user_id → item and user_secrets → vault_items

## Unreleased

- refactor: rename `user_id` → `item`, `user_secrets` → `vault_items`, `/v1/user-secrets` → `/v1/secrets`
- refactor: add optional 4-segment secret reference support (`jingui://vault/item/section/field`)
- refactor: DB migration for existing `user_secrets` → `vault_items`, `bound_user_id` → `bound_item`

## v0.1.2-alpha.6 (2026-02-26)

- logging: print version string on server startup

## v0.1.2-alpha.5 (2026-02-26)

- fix: add libgcc to runtime images for CGO unwind symbols

## v0.1.2-alpha.4 (2026-02-26)

- build: publish ratls-only artifacts and docker images

## v0.1.2-alpha.3 (2026-02-26)

- refactor: rename app namespace to vault and split attestation app binding

## v0.1.2-alpha.2 (2026-02-26)

- logging: add verbose/log-level flags and RA-TLS measurement debug logs

## v0.1.2-alpha.1 (2026-02-25)

- ratls: complete strict flow hardening, policy pinning, tests and docs
- ratls: require attestation app_id in strict challenge mode
- ratls: enforce RA-verified challenge state before fetch
- ratls: wire strict bidirectional attestation into challenge flow
- ratls: scaffold strict attestation handshake types and config
- docs: align with current API and remove init command surface
- docs: translate docs to English
- docs: update app update flow and read metadata behavior
- ux: improve app-duplicate error, add app update endpoint, and hide read metadata by default
- ci: restrict builds/releases to linux-amd64 only

## v0.1.1 (2026-02-25)

- feat: add status, runtime read policy, URL normalization, and openapi endpoint

## v0.1.0 (2026-02-24)

- refactor: start secret-ref migration to service/slug semantics
- docs: align ref semantics with workload identity and add refactor plan
- docs: sync API/PRD status and normalize changelog
- Add end-to-end manual test script

# Changelog

## Unreleased

- docs: update secret-reference semantics to `jingui://<service>/<slug>/<field>`
- docs: record identity-model correction (`app_id` from workload/attestation, not secret ref)
- docs: update PRD roadmap for one-shot migration + CRUD-first + RA-TLS next phase
- docs: add refactor execution tracker in `todo.md`
- docs: add manual test guide warning about pending reference-format migration
- feat(cli): add `jingui status` for instance registration checks
- feat(cli): default appkeys path is `/dstack/.host-shared/.appkeys.json`
- feat(cli): normalize server URLs to handle trailing `/`
- feat(server): add runtime user-level debug-read policy endpoints
- feat(server): tag requests with command type and enforce read policy
- feat(server): add `GET /` -> `ok` and `GET /openapi.json`
- docs: add `docs/openapi.json`
- docs: document `PUT /v1/apps/:app_id` update path and `jingui read --show-meta`
- docs: align README/PRD/manual guide with current `<app_id>/<user_id>/<field>` reference behavior
- docs: refresh OpenAPI paths to match implemented router endpoints
- feat(ratls): scaffold strict challenge attestation flow, dstack info collector, and ratls verifier integration
- chore: normalize changelog structure

## v0.0.3 (2026-02-13)

- Add admin CRUD inspection endpoints for apps, instances, and user-secrets
- Improve RegisterInstance error messages

## v0.0.2 (2026-02-13)

- Improve JINGUI_MASTER_KEY error message for debugging
