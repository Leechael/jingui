# TODO

## Refactor: fix identity model and secret reference semantics

### Decisions (locked)
- [x] `app_id` means CVM/Agent workload identity.
- [x] Secret refs do **not** include `app_id`.
- [x] New ref format: `jingui://<vault>/<item>/<field>` (with optional 4-segment `<section>` support).
- [x] CRUD-first execution strategy.
- [x] One-shot migration (no legacy-field compatibility, no `/v2` API split).
- [x] RA-TLS integration in next phase, but current refactor must avoid future breaking changes.

## Phase 1 (complete): schema + CRUD + server-client flow
- [x] Redesign schema: `user_secrets` → `vault_items`, `user_id` → `item`, with migration.
- [x] Update DB access layer (`apps`, `vault_items`, `tee_instances`) to new semantics.
- [x] Update ref parser to `vault/item/field` naming with optional 4-segment section support.
- [x] Update `fetch` authorization flow to use bound workload identity + parsed ref namespace.
- [x] Update admin CRUD handlers and validation/error messages.
- [x] Update integration tests / BDD fixtures to new reference format.
- [x] Update `scripts/manual-test.sh` and `docs/manual-test-guide.md` examples.

## Phase 2 (next): RA-TLS identity binding
- [x] Add verifier abstraction (`Noop` now, `RATLS` impl next).
- [x] Add attestation metadata fields/structures needed by future enforcement.
- [x] Integrate RA-TLS cert verification (dstack PR #512).
- [x] Integrate DCAP Go bindings (dcap-qvl PR #113).
- [x] Bind resolved workload identity (app_id) from attestation to policy checks.

## Phase 3: productionization
- [ ] PostgreSQL backend.
- [ ] Audit log for challenge/fetch/admin destructive operations.
- [ ] Secret lifecycle ops (rotation/invalid state handling) and policy docs.
