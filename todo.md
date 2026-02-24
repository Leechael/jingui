# TODO

## Refactor: fix identity model and secret reference semantics

### Decisions (locked)
- [x] `app_id` means CVM/Agent workload identity.
- [x] Secret refs do **not** include `app_id`.
- [x] New ref format: `jingui://<service>/<slug_or_email>/<field>`.
- [x] CRUD-first execution strategy.
- [x] One-shot migration (no legacy-field compatibility, no `/v2` API split).
- [x] RA-TLS integration in next phase, but current refactor must avoid future breaking changes.

## Phase 1 (now): schema + CRUD + server-client flow
- [ ] Redesign schema around `(app_id, user_id, service, slug)` secret keyspace.
- [ ] Update DB access layer (`apps`, `user_secrets`, `tee_instances`) to new semantics.
- [ ] Update ref parser to `service/slug/field` naming.
- [ ] Update `fetch` authorization flow to use bound workload identity + parsed ref namespace.
- [ ] Update admin CRUD handlers and validation/error messages.
- [ ] Update integration tests / BDD fixtures to new reference format.
- [ ] Update `scripts/manual-test.sh` and `docs/manual-test-guide.md` examples.

## Phase 2 (next): RA-TLS identity binding
- [ ] Add verifier abstraction (`Noop` now, `RATLS` impl next).
- [ ] Add attestation metadata fields/structures needed by future enforcement.
- [ ] Integrate RA-TLS cert verification (dstack PR #512).
- [ ] Integrate DCAP Go bindings (dcap-qvl PR #113).
- [ ] Bind resolved workload identity (app_id) from attestation to policy checks.

## Phase 3: productionization
- [ ] PostgreSQL backend.
- [ ] Audit log for challenge/fetch/admin destructive operations.
- [ ] Secret lifecycle ops (rotation/invalid state handling) and policy docs.
