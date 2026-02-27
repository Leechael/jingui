# Jingui Manual Test Guide

This guide splits testing into two parts: **Server side (operator machine)** and **Client side (TDX environment)**.
Values such as keys, FID, and public key are generated dynamically during execution. Record them and reuse them in later steps.

> If you want a quick local end-to-end regression first, run: `scripts/manual-test.sh`.
> The script already covers app / instance / secret admin CRUD checks and cascade-delete scenarios.
>
> Secret reference format: `jingui://<vault>/<item>/<field>` (3-segment) or `jingui://<vault>/<item>/<section>/<field>` (4-segment).
> Examples in this guide follow the current server/client behavior.

---

## 0. Build

On your development machine:

```bash
# Build runtime binaries
make clean build
bin/jingui --version        # → jingui ...
bin/jingui-server -v        # → jingui-server ...

# Build binaries (requires dcap-qvl static library for RA-TLS)
go build -o bin/jingui ./cmd/jingui
go build -o bin/jingui-server ./cmd/jingui-server

# Cross-compile linux/amd64 client for TDX
make build-client-linux-amd64
file bin/linux-amd64/jingui  # → ELF 64-bit LSB executable, x86-64 ...

# Cross-compile linux/amd64 server (if server runs on Linux)
make build-server-linux-amd64
```

Artifacts:

| Target | Path |
|------|------|
| Current platform client | `bin/jingui` |
| Current platform server | `bin/jingui-server` |
| linux/amd64 client | `bin/linux-amd64/jingui` |
| linux/amd64 server | `bin/linux-amd64/jingui-server` |

---

## Part A — Server-side Operations (Operator)

### A1. Generate Master Key and Admin Token

```bash
export JINGUI_MASTER_KEY=$(openssl rand -hex 32)
echo "Master Key: $JINGUI_MASTER_KEY"   # Save this; required when starting server

export JINGUI_ADMIN_TOKEN=$(openssl rand -hex 16)
echo "Admin Token: $JINGUI_ADMIN_TOKEN"  # Save this; required for admin APIs
```

### A2. Start the Server

```bash
export JINGUI_MASTER_KEY="<value from previous step>"
export JINGUI_ADMIN_TOKEN="<value from previous step>"
export JINGUI_DB_PATH="./jingui-test.db"
export JINGUI_LISTEN_ADDR=":8080"
export JINGUI_BASE_URL="http://<SERVER_IP>:8080"   # Must be reachable from TDX
export JINGUI_LOG_LEVEL="debug"                    # Optional: print RA measurements/status

bin/jingui-server
# Output: jingui-server listening on :8080
```

> If server runs on Linux, use `bin/linux-amd64/jingui-server`.
> Note: if `JINGUI_BASE_URL` is not HTTPS, server prints a warning. This is acceptable in test environments.
> For RA-TLS diagnostics, either set `JINGUI_LOG_LEVEL=debug` or start with `bin/jingui-server --verbose`.

### A3. Register App (Upload Google OAuth `credentials.json`)

> If you get `app_id already exists`, use `PUT /v1/apps/:app_id` to update instead of repeating `POST /v1/apps`.

Prepare your downloaded Google Cloud Console `credentials.json`, for example:

```json
{"installed":{"client_id":"xxx.apps.googleusercontent.com","client_secret":"GOCSPX-xxx","redirect_uris":["http://localhost"]}}
```

Register:

```bash
SERVER="http://<SERVER_IP>:8080"
ADMIN_TOKEN="<Admin Token generated in A1>"

curl -s -X POST "$SERVER/v1/apps" \
  -H 'Content-Type: application/json' \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -d "$(jq -n \
    --arg vault "gmail-app" \
    --arg name "Gmail App" \
    --arg service_type "gmail" \
    --arg scopes "https://mail.google.com/" \
    --argjson creds "$(cat credentials.json)" \
    '{vault:$vault, name:$name, service_type:$service_type, required_scopes:$scopes, credentials_json:$creds}'
  )"
```

**Expected response:**

```json
{"vault":"gmail-app","status":"created"}
```

**Check ✓**: HTTP 201, status = `created`

Update an existing app:

```bash
curl -s -X PUT "$SERVER/v1/apps/gmail-app" \
  -H 'Content-Type: application/json' \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -d "$(jq -n \
    --arg vault "gmail-app" \
    --arg name "Gmail App" \
    --arg service_type "gmail" \
    --arg scopes "https://mail.google.com/" \
    --argjson creds "$(cat credentials.json)" \
    '{vault:$vault, name:$name, service_type:$service_type, required_scopes:$scopes, credentials_json:$creds}'
  )"
```

### A4. OAuth Authorization (Get `refresh_token`)

OAuth gateway requires admin token. In browser flows you cannot directly set headers, so use curl to get the redirect URL:

```bash
curl -s -v "$SERVER/v1/credentials/gateway/gmail-app" \
  -H "Authorization: Bearer $ADMIN_TOKEN" 2>&1 | grep -i location
```

Copy the `Location:` Google OAuth URL to your browser.

Or, if server is local, inspect response headers directly:

```bash
# This prints the Google login URL; copy it to your browser
curl -s -D - "$SERVER/v1/credentials/gateway/gmail-app" \
  -H "Authorization: Bearer $ADMIN_TOKEN" | head -20
```

Flow:
1. Browser redirects to Google login page
2. Select account and grant permission
3. Callback goes to `/v1/credentials/callback`
4. Page displays JSON result

**Expected response:**

```json
{"status":"authorized","vault":"gmail-app","email":"user@example.com"}
```

**Check ✓**: `status = authorized`, `email` matches the authorized Google account

> **Record the email**: `bound_item` in TEE instance registration must match this value.

### A5. (Pause) Wait for Client-Side Key Generation

Continue with Part B in TDX to generate keys, then come back with the **public_key** for A6.

---

## Part B — Client-side Operations (TDX)

### B1. Copy Files to TDX

Required file:

```
bin/linux-amd64/jingui      # runtime client binary
```

```bash
# Example: scp to TDX instance
scp bin/linux-amd64/jingui tdx-host:/opt/jingui/jingui

# Make executable inside TDX
ssh tdx-host 'chmod +x /opt/jingui/jingui'
```

### B2. Verify Binary

```bash
# Inside TDX
/opt/jingui/jingui --version
# → jingui ... (linux/amd64)
```

**Check ✓**: version output shows `linux/amd64`

### B3. Use TEE-provisioned `.appkeys.json` and inspect identity

`jingui` does not generate keys. `.appkeys.json` should be provisioned by TEE/KMS flow (default path: `/dstack/.host-shared/.appkeys.json`).

Run status to print local identity (FID/public key):

```bash
/opt/jingui/jingui status \
  --server "http://<SERVER_IP>:8080" \
  --appkeys /dstack/.host-shared/.appkeys.json \
  --insecure
```

Example output (before registration):

```
appkeys_path=/dstack/.host-shared/.appkeys.json
fid=2f4e9d...
public_key=7a8b3c...
server=http://<SERVER_IP>:8080
registered=false
status_error=challenge endpoint returned 404: {"error":"instance not found"}
```

**Record:**
- `Public Key`: **____________________________**
- `FID`: **____________________________**

> `.appkeys.json` contains private key material. Do not copy it out of TDX.

### B4. (Pause) Return to Server Side for Registration

Send the **Public Key** to operator and continue Part A at A6.

---

## Part A (continued) — Register TEE Instance

### A6. Register TEE Instance

Use the TDX-generated public key and the OAuth email from A4.

`bound_attestation_app_id` is a hex string identifying the RA-TLS application identity
(e.g. SHA-1 of the app certificate). In production this comes from the TEE attestation
report; for manual testing use any 40-char hex value:

```bash
SERVER="http://<SERVER_IP>:8080"
ADMIN_TOKEN="<Admin Token generated in A1>"
PUBLIC_KEY="<Public Key from B3>"
EMAIL="<Authorized email from A4>"
ATTESTATION_APP_ID="e2215b69c6f4e3aa0584a60fda044bfe1a133ff9"

curl -s -X POST "$SERVER/v1/instances" \
  -H 'Content-Type: application/json' \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -d "$(jq -n \
    --arg pk "$PUBLIC_KEY" \
    --arg app "gmail-app" \
    --arg attestation "$ATTESTATION_APP_ID" \
    --arg user "$EMAIL" \
    --arg label "tdx-test-1" \
    '{public_key:$pk, bound_vault:$app, bound_attestation_app_id:$attestation, bound_item:$user, label:$label}'
  )"
```

**Expected response:**

```json
{"fid":"2f4e9d...","status":"registered"}
```

**Check ✓**:
- HTTP 201
- Returned `fid` matches the FID shown in B3 status output

### A6b. Update TEE Instance (Optional)

Update `bound_attestation_app_id` or `label` on an existing instance without deleting and re-registering:

```bash
FID="<FID from B3>"
NEW_ATTESTATION_APP_ID="<new 40-char hex value>"

curl -s -X PUT "$SERVER/v1/instances/$FID" \
  -H 'Content-Type: application/json' \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -d "$(jq -n \
    --arg attestation "$NEW_ATTESTATION_APP_ID" \
    --arg label "updated-label" \
    '{bound_attestation_app_id:$attestation, label:$label}'
  )"
```

**Expected response:**

```json
{"fid":"2f4e9d...","status":"updated"}
```

**Check ✓**:
- HTTP 200
- Verify via `GET /v1/instances/$FID` that `bound_attestation_app_id` and `label` reflect the new values

### A7. Challenge Endpoint Smoke Test (Optional)

`/v1/secrets/fetch` now requires challenge-response proof (`challenge_id` + `challenge_response`).
If you only want a quick server-side check, verify challenge issuance:

```bash
FID="<FID from B3>"

curl -s -X POST "$SERVER/v1/secrets/challenge" \
  -H 'Content-Type: application/json' \
  -d "$(jq -n --arg fid "$FID" '{fid:$fid}')" | jq .
```

**Expected response:** includes non-empty `challenge_id` and `challenge` (base64 ECIES blob).

For full end-to-end fetch/decrypt validation, continue with client-side tests in Part B (`jingui read` / `jingui run`).

---

## Part B (continued) — Full Client-Side Tests

### B5. Prepare `.env`

```bash
cat > /opt/jingui/.env << 'EOF'
GOG_ACCOUNT=user@example.com
GOG_CLIENT_ID=jingui://gmail-app/user@example.com/client_id
GOG_CLIENT_SECRET=jingui://gmail-app/user@example.com/client_secret
GOG_REFRESH_TOKEN=jingui://gmail-app/user@example.com/refresh_token
EOF
```

> Replace `user@example.com` with the actual authorized email from A4.

### B6. Test 1 — `jingui read` Single Secret

```bash
cd /opt/jingui
./jingui read \
  --verbose \
  --server "http://<SERVER_IP>:8080" \
  --appkeys .appkeys.json \
  --insecure \
  "jingui://gmail-app/user@example.com/client_id"
```

**Expected**: prints the real `client_id` value to stdout (e.g., `xxx.apps.googleusercontent.com`). With `--verbose`, stderr also includes RA-TLS verification logs (TCB status, MR/RTMR measurements, app_id binding).

To display debug metadata (FID/Public Key):

```bash
./jingui read \
  --show-meta \
  --server "http://<SERVER_IP>:8080" \
  --appkeys .appkeys.json \
  --insecure \
  "jingui://gmail-app/user@example.com/client_id"
```

**Check ✓**: output matches `client_id` in `credentials.json`.

### B7. Test 2 — stdout Redaction

```bash
./jingui run \
  --server "http://<SERVER_IP>:8080" \
  --appkeys .appkeys.json \
  --insecure \
  --env-file .env \
  -- /bin/sh -c 'echo $GOG_CLIENT_ID'
```

**Expected output:**

```
[REDACTED_BY_JINGUI]
```

**Check ✓**: real `client_id` does not appear on stdout.

### B8. Test 3 — stderr Redaction

```bash
./jingui run \
  --server "http://<SERVER_IP>:8080" \
  --appkeys .appkeys.json \
  --insecure \
  --env-file .env \
  -- /bin/sh -c 'echo $GOG_CLIENT_SECRET >&2'
```

**Expected stderr output:**

```
[REDACTED_BY_JINGUI]
```

**Check ✓**: real `client_secret` does not appear on stderr.

### B9. Test 4 — Redact Multiple Secrets in One Line

```bash
./jingui run \
  --server "http://<SERVER_IP>:8080" \
  --appkeys .appkeys.json \
  --insecure \
  --env-file .env \
  -- /bin/sh -c 'echo "id=$GOG_CLIENT_ID secret=$GOG_CLIENT_SECRET token=$GOG_REFRESH_TOKEN"'
```

**Expected output:**

```
id=[REDACTED_BY_JINGUI] secret=[REDACTED_BY_JINGUI] token=[REDACTED_BY_JINGUI]
```

**Check ✓**: all three values are redacted.

### B10. Test 5 — Normal Environment Variables Unchanged

```bash
./jingui run \
  --server "http://<SERVER_IP>:8080" \
  --appkeys .appkeys.json \
  --insecure \
  --env-file .env \
  -- /bin/sh -c 'echo "account=$GOG_ACCOUNT"'
```

**Expected output:**

```
account=user@example.com
```

**Check ✓**: non-`jingui://` values pass through as-is and are not redacted.

### B11. Test 6 — Child Exit Code Propagation

```bash
./jingui run \
  --server "http://<SERVER_IP>:8080" \
  --appkeys .appkeys.json \
  --insecure \
  --env-file .env \
  -- /bin/sh -c 'exit 42'
echo "exit code: $?"
```

**Expected output:**

```
exit code: 42
```

**Check ✓**: child process exit code is propagated correctly.

### B12. Test 7 — Integration with gogcli (Final Validation)

```bash
./jingui run \
  --server "http://<SERVER_IP>:8080" \
  --appkeys .appkeys.json \
  --insecure \
  --env-file .env \
  -- gogcli gmail messages list
```

**Check ✓**:
- gogcli works normally (lists Gmail messages)
- no real `client_id` / `client_secret` / `refresh_token` is leaked in stdout/stderr

---

## Authentication and Access Control Tests (Server Side)

### C0. Admin Token Authentication

```bash
# No token → 401
curl -s -o /dev/null -w "%{http_code}" -X POST "$SERVER/v1/apps" \
  -H 'Content-Type: application/json' \
  -d '{"vault":"x","name":"x","service_type":"x","credentials_json":{"installed":{"client_id":"a","client_secret":"b"}}}'
# Expected: 401

# Wrong token → 401
curl -s -o /dev/null -w "%{http_code}" -X POST "$SERVER/v1/apps" \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer wrong-token' \
  -d '{"vault":"x","name":"x","service_type":"x","credentials_json":{"installed":{"client_id":"a","client_secret":"b"}}}'
# Expected: 401

# secrets/challenge is a TEE caller endpoint (no admin token)
curl -s -o /dev/null -w "%{http_code}" -X POST "$SERVER/v1/secrets/challenge" \
  -H 'Content-Type: application/json' \
  -d '{"fid":"0000000000000000000000000000000000000000"}'
# Expected: 404 (instance not found), not 401
```

**Check ✓**: admin APIs require Bearer token; TEE secret endpoints do not use admin token auth.

### C1. Wrong `app_id` (via client read)

```bash
./jingui read \
  --server "http://<SERVER_IP>:8080" \
  --appkeys .appkeys.json \
  --insecure \
  "jingui://wrong-app/user@example.com/client_id"
```

**Expected**: request fails with HTTP 403 (`vault mismatch ...`).

### C2. Wrong item (via client read)

```bash
./jingui read \
  --server "http://<SERVER_IP>:8080" \
  --appkeys .appkeys.json \
  --insecure \
  "jingui://gmail-app/wrong@example.com/client_id"
```

**Expected**: request fails with HTTP 404 (`item not found ...`).

### C3. Non-existent FID

```bash
# Use a different TEE instance's appkeys file that has NOT been registered on this server
UNREGISTERED_APPKEYS="/path/to/another-instance/.appkeys.json"

./jingui read \
  --server "http://<SERVER_IP>:8080" \
  --appkeys "$UNREGISTERED_APPKEYS" \
  --insecure \
  "jingui://gmail-app/user@example.com/client_id"
```

**Expected**: request fails with HTTP 404 (`instance not found`).

---

## Admin CRUD Supplemental Checks (New)

### D1. Query Endpoints (apps / instances / secrets)

```bash
curl -s "$SERVER/v1/apps" -H "Authorization: Bearer $ADMIN_TOKEN" | jq .
curl -s "$SERVER/v1/instances" -H "Authorization: Bearer $ADMIN_TOKEN" | jq .
curl -s "$SERVER/v1/secrets" -H "Authorization: Bearer $ADMIN_TOKEN" | jq .
```

**Check ✓**:
- all return 200
- `apps` does not leak `credentials_encrypted`
- `secrets` list does not leak `secret_encrypted`

### D2. Non-cascade Delete Should Be Blocked

```bash
# If vault_items/instances still exist under app, delete should fail
curl -s -o /dev/null -w "%{http_code}" -X DELETE \
  "$SERVER/v1/apps/gmail-app" \
  -H "Authorization: Bearer $ADMIN_TOKEN"
```

**Expected**: 409 (dependent records exist)

### D3. Cascade Delete

```bash
# Delete app and dependent records
curl -s -X DELETE \
  "$SERVER/v1/apps/gmail-app?cascade=true" \
  -H "Authorization: Bearer $ADMIN_TOKEN"
```

**Expected**: 200; corresponding records should be removed from apps / secrets / instances.

---

## Checklist

| # | Test Item | Expected | Pass |
|---|--------|------|------|
| A1 | Generate Master Key + Admin Token | both values recorded | ☐ |
| A3 | Register App (with admin token) | 201 created | ☐ |
| A4 | OAuth authorization | authorized + email | ☐ |
| A6 | Register TEE Instance | 201 registered, FID matches | ☐ |
| A6b | Update TEE Instance | 200 updated, GET reflects new values | ☐ |
| A7 | challenge endpoint smoke test | returns challenge_id + challenge | ☐ |
| B2 | Binary version | linux/amd64 | ☐ |
| B3 | `jingui status` | FID/public_key printed from provisioned `.appkeys.json` | ☐ |
| B6 | `jingui read` | real `client_id` printed | ☐ |
| B7 | stdout redaction | `[REDACTED_BY_JINGUI]` | ☐ |
| B8 | stderr redaction | `[REDACTED_BY_JINGUI]` | ☐ |
| B9 | Multi-secret redaction | all three redacted | ☐ |
| B10 | Normal env passthrough | `account=user@example.com` | ☐ |
| B11 | Exit code propagation | `exit code: 42` | ☐ |
| B12 | gogcli integration | works + no leaks | ☐ |
| C0 | Admin token auth | no/wrong token → 401, TEE secret endpoints are not admin-token protected | ☐ |
| C1 | Wrong app_id | 403 | ☐ |
| C2 | Wrong item | 404 | ☐ |
| C3 | Non-existent FID | 404 | ☐ |
