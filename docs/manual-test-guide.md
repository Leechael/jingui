# Jingui Manual Test Guide

This guide splits testing into two parts: **Server side (operator machine)** and **Client side (TDX environment)**.
Values such as keys, FID, and public key are generated dynamically during execution. Record them and reuse them in later steps.

> If you want a quick local end-to-end regression first, run: `scripts/manual-test.sh`.
> The script already covers app / instance / user-secret admin CRUD checks and cascade-delete scenarios.
>
> ⚠️ The design has been confirmed to move to `jingui://<service>/<slug>/<field>` semantics.
> `app_id` will no longer appear in secret references.
> Some examples below still use the old format and will be updated after schema/handler refactor is complete.

---

## 0. Build

On your development machine:

```bash
# Verify versions
make clean build
bin/jingui --version        # → jingui dev (commit=..., go=..., darwin/arm64)
bin/jingui-server -v        # → jingui-server dev (commit=..., go=..., darwin/arm64)

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

bin/jingui-server
# Output: jingui-server listening on :8080
```

> If server runs on Linux, use `bin/linux-amd64/jingui-server`.
> Note: if `JINGUI_BASE_URL` is not HTTPS, server prints a warning. This is acceptable in test environments.

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
    --arg app_id "gmail-app" \
    --arg name "Gmail App" \
    --arg service_type "gmail" \
    --arg scopes "https://mail.google.com/" \
    --argjson creds "$(cat credentials.json)" \
    '{app_id:$app_id, name:$name, service_type:$service_type, required_scopes:$scopes, credentials_json:$creds}'
  )"
```

**Expected response:**

```json
{"app_id":"gmail-app","status":"created"}
```

**Check ✓**: HTTP 201, status = `created`

Update an existing app:

```bash
curl -s -X PUT "$SERVER/v1/apps/gmail-app" \
  -H 'Content-Type: application/json' \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -d "$(jq -n \
    --arg app_id "gmail-app" \
    --arg name "Gmail App" \
    --arg service_type "gmail" \
    --arg scopes "https://mail.google.com/" \
    --argjson creds "$(cat credentials.json)" \
    '{app_id:$app_id, name:$name, service_type:$service_type, required_scopes:$scopes, credentials_json:$creds}'
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
{"status":"authorized","app_id":"gmail-app","email":"user@example.com"}
```

**Check ✓**: `status = authorized`, `email` matches the authorized Google account

> **Record the email**: `bound_user_id` in TEE instance registration must match this value.

### A5. (Pause) Wait for Client-Side Key Generation

Continue with Part B in TDX to generate keys, then come back with the **public_key** for A6.

---

## Part B — Client-side Operations (TDX)

### B1. Copy Files to TDX

Required file:

```
bin/linux-amd64/jingui     # client binary
```

```bash
# Example: scp to TDX instance
scp bin/linux-amd64/jingui  tdx-host:/opt/jingui/jingui

# Make executable inside TDX
ssh tdx-host 'chmod +x /opt/jingui/jingui'
```

### B2. Verify Binary

```bash
# Inside TDX
/opt/jingui/jingui --version
# → jingui dev (commit=..., go=..., linux/amd64)

/opt/jingui/jingui --help
```

**Check ✓**: version output shows `linux/amd64`

### B3. Generate Key Pair (`.appkeys.json`)

```bash
cd /opt/jingui
./jingui init -o .appkeys.json
```

Example output:

```
Wrote .appkeys.json

Public Key : 7a8b3c...  (64 hex chars)
FID        : 2f4e9d...  (40 hex chars)

Use the public key to register this instance:
  curl -X POST $SERVER/v1/instances \
    -H 'Content-Type: application/json' \
    -d '{"public_key":"7a8b3c...","bound_app_id":"<APP_ID>","bound_user_id":"<EMAIL>"}'
```

**Record:**
- `Public Key`: **____________________________**
- `FID`: **____________________________**

> `.appkeys.json` contains private key material and is set to permission `0600`. Do not copy it out of TDX.

### B4. (Pause) Return to Server Side for Registration

Send the **Public Key** to operator and continue Part A at A6.

---

## Part A (continued) — Register TEE Instance

### A6. Register TEE Instance

Use the TDX-generated public key and the OAuth email from A4:

```bash
SERVER="http://<SERVER_IP>:8080"
ADMIN_TOKEN="<Admin Token generated in A1>"
PUBLIC_KEY="<Public Key from B3>"
EMAIL="<Authorized email from A4>"

curl -s -X POST "$SERVER/v1/instances" \
  -H 'Content-Type: application/json' \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -d "$(jq -n \
    --arg pk "$PUBLIC_KEY" \
    --arg app "gmail-app" \
    --arg user "$EMAIL" \
    --arg label "tdx-test-1" \
    '{public_key:$pk, bound_app_id:$app, bound_user_id:$user, label:$label}'
  )"
```

**Expected response:**

```json
{"fid":"2f4e9d...","status":"registered"}
```

**Check ✓**:
- HTTP 201
- Returned `fid` matches the FID output from B3 `jingui init`

### A7. Direct `secrets/fetch` Test with curl (Optional)

```bash
FID="<FID from B3>"

curl -s -X POST "$SERVER/v1/secrets/fetch" \
  -H 'Content-Type: application/json' \
  -d "$(jq -n \
    --arg fid "$FID" \
    '{fid:$fid, secret_references:["jingui://gmail-app/'"$EMAIL"'/client_id","jingui://gmail-app/'"$EMAIL"'/client_secret","jingui://gmail-app/'"$EMAIL"'/refresh_token"]}'
  )" | jq .
```

**Expected response:**

```json
{
  "secrets": {
    "jingui://gmail-app/user@example.com/client_id": "<base64 ECIES blob>",
    "jingui://gmail-app/user@example.com/client_secret": "<base64 ECIES blob>",
    "jingui://gmail-app/user@example.com/refresh_token": "<base64 ECIES blob>"
  }
}
```

**Check ✓**: all 3 keys are returned, each value is a base64-encoded ECIES ciphertext.

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
  --server "http://<SERVER_IP>:8080" \
  --appkeys .appkeys.json \
  --insecure \
  "jingui://gmail-app/user@example.com/client_id"
```

**Expected**: by default prints only the real `client_id` value to stdout (e.g., `xxx.apps.googleusercontent.com`).

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
  -d '{"app_id":"x","name":"x","service_type":"x","credentials_json":{"installed":{"client_id":"a","client_secret":"b"}}}'
# Expected: 401

# Wrong token → 401
curl -s -o /dev/null -w "%{http_code}" -X POST "$SERVER/v1/apps" \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer wrong-token' \
  -d '{"app_id":"x","name":"x","service_type":"x","credentials_json":{"installed":{"client_id":"a","client_secret":"b"}}}'
# Expected: 401

# secrets/fetch does not require admin token (for TEE callers)
curl -s -o /dev/null -w "%{http_code}" -X POST "$SERVER/v1/secrets/fetch" \
  -H 'Content-Type: application/json' \
  -d '{"fid":"nonexistent","secret_references":[]}'
# Expected: not 401 (should be 404)
```

**Check ✓**: admin APIs (apps, instances, gateway) require correct Bearer token; `secrets/fetch` does not.

### C1. Wrong `app_id`

```bash
curl -s -X POST "$SERVER/v1/secrets/fetch" \
  -H 'Content-Type: application/json' \
  -d '{"fid":"'"$FID"'","secret_references":["jingui://wrong-app/user@example.com/client_id"]}'
```

**Expected**: HTTP 403 `{"error":"app_id mismatch ..."}`

### C2. Wrong `user_id`

```bash
curl -s -X POST "$SERVER/v1/secrets/fetch" \
  -H 'Content-Type: application/json' \
  -d '{"fid":"'"$FID"'","secret_references":["jingui://gmail-app/wrong@example.com/client_id"]}'
```

**Expected**: HTTP 403 `{"error":"user_id mismatch ..."}`

### C3. Non-existent FID

```bash
curl -s -X POST "$SERVER/v1/secrets/fetch" \
  -H 'Content-Type: application/json' \
  -d '{"fid":"0000000000000000000000000000000000000000","secret_references":["jingui://gmail-app/user@example.com/client_id"]}'
```

**Expected**: HTTP 404 `{"error":"instance not found"}`

---

## Admin CRUD Supplemental Checks (New)

### D1. Query Endpoints (apps / instances / user-secrets)

```bash
curl -s "$SERVER/v1/apps" -H "Authorization: Bearer $ADMIN_TOKEN" | jq .
curl -s "$SERVER/v1/instances" -H "Authorization: Bearer $ADMIN_TOKEN" | jq .
curl -s "$SERVER/v1/user-secrets" -H "Authorization: Bearer $ADMIN_TOKEN" | jq .
```

**Check ✓**:
- all return 200
- `apps` does not leak `credentials_encrypted`
- `user-secrets` list does not leak `secret_encrypted`

### D2. Non-cascade Delete Should Be Blocked

```bash
# If user_secrets/instances still exist under app, delete should fail
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

**Expected**: 200; corresponding records should be removed from apps / user-secrets / instances.

---

## Checklist

| # | Test Item | Expected | Pass |
|---|--------|------|------|
| A1 | Generate Master Key + Admin Token | both values recorded | ☐ |
| A3 | Register App (with admin token) | 201 created | ☐ |
| A4 | OAuth authorization | authorized + email | ☐ |
| A6 | Register TEE Instance | 201 registered, FID matches | ☐ |
| A7 | curl fetch secrets | 3 base64 blobs returned | ☐ |
| B2 | Binary version | linux/amd64 | ☐ |
| B3 | `jingui init` | `.appkeys.json` generated + pubkey/FID printed | ☐ |
| B6 | `jingui read` | real `client_id` printed | ☐ |
| B7 | stdout redaction | `[REDACTED_BY_JINGUI]` | ☐ |
| B8 | stderr redaction | `[REDACTED_BY_JINGUI]` | ☐ |
| B9 | Multi-secret redaction | all three redacted | ☐ |
| B10 | Normal env passthrough | `account=user@example.com` | ☐ |
| B11 | Exit code propagation | `exit code: 42` | ☐ |
| B12 | gogcli integration | works + no leaks | ☐ |
| C0 | Admin token auth | no/wrong token → 401, `secrets/fetch` no token required | ☐ |
| C1 | Wrong app_id | 403 | ☐ |
| C2 | Wrong user_id | 403 | ☐ |
| C3 | Non-existent FID | 404 | ☐ |
