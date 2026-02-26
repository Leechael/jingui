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
