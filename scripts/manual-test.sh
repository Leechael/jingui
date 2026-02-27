#!/usr/bin/env bash
#
# End-to-end manual test: server setup → admin curl → client fetch secrets
#
# Prerequisites: go, curl, python3 (for JSON parsing)
# Usage:  bash scripts/manual-test.sh
#
set -uo pipefail

WORKDIR=$(mktemp -d)
trap 'kill "$SERVER_PID" 2>/dev/null; rm -rf "$WORKDIR"' EXIT
echo "Working directory: $WORKDIR"

REPO_ROOT="$(cd "$(dirname "$0")/.."; pwd)"
PORT=18199
BASE="http://localhost:$PORT"
MASTER_KEY=$(openssl rand -hex 32)
ADMIN_TOKEN="manual-test-admin-token-$(openssl rand -hex 8)"
AUTH="Authorization: Bearer $ADMIN_TOKEN"

pass=0
fail=0
check() {
  local desc="$1" expected="$2" actual="$3"
  if [ "$actual" = "$expected" ]; then
    printf "  \033[32mPASS\033[0m  %s\n" "$desc"
    ((pass++))
  else
    printf "  \033[31mFAIL\033[0m  %s (expected %s, got %s)\n" "$desc" "$expected" "$actual"
    ((fail++))
  fi
}

json_val() { python3 -c "import sys,json; print(json.load(open('$1'))$2)"; }
json_len() { python3 -c "import json; d=json.load(open('$1')); print(len(d) if d else 0)"; }
json_has() { python3 -c "import json; print('$2' in json.load(open('$1')))"; }
json_any_has() { python3 -c "import json; print(any('$2' in x for x in json.load(open('$1'))))"; }

########################################################################
echo ""
echo "=============================================="
echo "  Phase 1: Build server & client"
echo "=============================================="

echo "Building server..."
(cd "$REPO_ROOT" && go build -o "$WORKDIR/jingui-server" ./cmd/jingui-server) || { echo "FATAL: server build failed"; exit 1; }
echo "  Built: $WORKDIR/jingui-server"

echo "Building client..."
(cd "$REPO_ROOT" && go build -o "$WORKDIR/jingui" ./cmd/jingui) || { echo "FATAL: client build failed"; exit 1; }
echo "  Built: $WORKDIR/jingui"

########################################################################
echo ""
echo "=============================================="
echo "  Phase 2: Start server"
echo "=============================================="

export JINGUI_MASTER_KEY="$MASTER_KEY"
export JINGUI_ADMIN_TOKEN="$ADMIN_TOKEN"
export JINGUI_DB_PATH="$WORKDIR/test.db"
export JINGUI_LISTEN_ADDR=":$PORT"
export JINGUI_BASE_URL="$BASE"
export GIN_MODE=release

"$WORKDIR/jingui-server" > "$WORKDIR/server.log" 2>&1 &
SERVER_PID=$!
echo "  Server PID: $SERVER_PID (log: $WORKDIR/server.log)"

# Wait for server to be ready
for i in $(seq 1 30); do
  if curl -sf "$BASE/v1/apps" -H "$AUTH" > /dev/null 2>&1; then
    echo "  Server ready (took ${i}00ms)"
    break
  fi
  sleep 0.1
done

########################################################################
echo ""
echo "=============================================="
echo "  Phase 3: Admin setup via curl"
echo "=============================================="

echo ""
echo "--- 3.1 Create app ---"
HTTP_CODE=$(curl -s -o "$WORKDIR/resp.json" -w "%{http_code}" \
  -X POST "$BASE/v1/apps" -H "$AUTH" -H "Content-Type: application/json" \
  -d '{
    "app_id": "my-gmail",
    "name": "My Gmail App",
    "service_type": "gmail",
    "required_scopes": "https://mail.google.com/",
    "credentials_json": {
      "installed": {
        "client_id": "test-client-id.apps.googleusercontent.com",
        "client_secret": "test-client-secret-value",
        "redirect_uris": ["http://localhost"]
      }
    }
  }')
check "POST /v1/apps → 201" "201" "$HTTP_CODE"
VAL=$(json_val "$WORKDIR/resp.json" "['status']")
check "  status = created" "created" "$VAL"

echo ""
echo "--- 3.2 List apps ---"
HTTP_CODE=$(curl -s -o "$WORKDIR/resp.json" -w "%{http_code}" \
  "$BASE/v1/apps" -H "$AUTH")
check "GET /v1/apps → 200" "200" "$HTTP_CODE"
COUNT=$(json_len "$WORKDIR/resp.json")
check "  app count = 1" "1" "$COUNT"

echo ""
echo "--- 3.3 Get app detail ---"
HTTP_CODE=$(curl -s -o "$WORKDIR/resp.json" -w "%{http_code}" \
  "$BASE/v1/apps/my-gmail" -H "$AUTH")
check "GET /v1/apps/my-gmail → 200" "200" "$HTTP_CODE"
VAL=$(json_val "$WORKDIR/resp.json" "['has_credentials']")
check "  has_credentials = True" "True" "$VAL"
HAS_ENCRYPTED=$(json_has "$WORKDIR/resp.json" "credentials_encrypted")
check "  credentials_encrypted not leaked" "False" "$HAS_ENCRYPTED"

echo ""
echo "--- 3.4 Get nonexistent app → 404 ---"
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" \
  "$BASE/v1/apps/nonexistent" -H "$AUTH")
check "GET /v1/apps/nonexistent → 404" "404" "$HTTP_CODE"

echo ""
echo "--- 3.5 Store user credentials (PUT /v1/credentials) ---"
HTTP_CODE=$(curl -s -o "$WORKDIR/resp.json" -w "%{http_code}" \
  -X PUT "$BASE/v1/credentials/my-gmail" -H "$AUTH" -H "Content-Type: application/json" \
  -d '{
    "item": "alice@example.com",
    "secrets": {
      "refresh_token": "ya29.super-secret-refresh-token-for-alice"
    }
  }')
check "PUT /v1/credentials/my-gmail → 200" "200" "$HTTP_CODE"

echo ""
echo "--- 3.6 List secrets ---"
HTTP_CODE=$(curl -s -o "$WORKDIR/resp.json" -w "%{http_code}" \
  "$BASE/v1/secrets" -H "$AUTH")
check "GET /v1/secrets → 200" "200" "$HTTP_CODE"
COUNT=$(json_len "$WORKDIR/resp.json")
check "  secret count = 1" "1" "$COUNT"
HAS_ENCRYPTED=$(json_any_has "$WORKDIR/resp.json" "secret_encrypted")
check "  secret_encrypted not leaked" "False" "$HAS_ENCRYPTED"

echo ""
echo "--- 3.7 List secrets filtered by vault ---"
HTTP_CODE=$(curl -s -o "$WORKDIR/resp.json" -w "%{http_code}" \
  "$BASE/v1/secrets?vault=my-gmail" -H "$AUTH")
check "GET /v1/secrets?vault=my-gmail → 200" "200" "$HTTP_CODE"
COUNT=$(json_len "$WORKDIR/resp.json")
check "  filtered count = 1" "1" "$COUNT"

echo ""
echo "--- 3.8 Get secret detail ---"
HTTP_CODE=$(curl -s -o "$WORKDIR/resp.json" -w "%{http_code}" \
  "$BASE/v1/secrets/my-gmail/alice@example.com" -H "$AUTH")
check "GET /v1/secrets/.../alice → 200" "200" "$HTTP_CODE"
VAL=$(json_val "$WORKDIR/resp.json" "['has_secret']")
check "  has_secret = True" "True" "$VAL"

########################################################################
echo ""
echo "=============================================="
echo "  Phase 4: Client setup"
echo "=============================================="

echo ""
echo "--- 4.1 Prepare .appkeys.json ---"
cat > "$WORKDIR/gen_appkeys.go" <<'EOF'
package main

import (
	"crypto/rand"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"

	"golang.org/x/crypto/curve25519"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: gen_appkeys <output>")
		os.Exit(2)
	}
	out := os.Args[1]

	var priv [32]byte
	if _, err := rand.Read(priv[:]); err != nil {
		panic(err)
	}
	pub, err := curve25519.X25519(priv[:], curve25519.Basepoint)
	if err != nil {
		panic(err)
	}

	data, err := json.MarshalIndent(map[string]string{"env_crypt_key": hex.EncodeToString(priv[:])}, "", "  ")
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile(out, data, 0600); err != nil {
		panic(err)
	}

	h := sha1.Sum(pub)
	fmt.Printf("Public Key : %s\n", hex.EncodeToString(pub))
	fmt.Printf("FID        : %s\n", hex.EncodeToString(h[:]))
}
EOF

(cd "$REPO_ROOT" && go run "$WORKDIR/gen_appkeys.go" "$WORKDIR/appkeys.json") >"$WORKDIR/keygen.txt"
echo "  Generated: $WORKDIR/appkeys.json"
cat "$WORKDIR/keygen.txt"

PUB_KEY=$(grep "Public Key" "$WORKDIR/keygen.txt" | awk '{print $NF}')
FID=$(grep "FID" "$WORKDIR/keygen.txt" | head -1 | awk '{print $NF}')
echo "  Public Key: $PUB_KEY"
echo "  FID: $FID"
check "  Public key is 64 hex chars" "64" "${#PUB_KEY}"
check "  FID is 40 hex chars" "40" "${#FID}"

echo ""
echo "--- 4.2 Register TEE instance ---"
HTTP_CODE=$(curl -s -o "$WORKDIR/resp.json" -w "%{http_code}" \
  -X POST "$BASE/v1/instances" -H "$AUTH" -H "Content-Type: application/json" \
  -d "{
    \"public_key\": \"$PUB_KEY\",
    \"bound_vault\": \"my-gmail\",
    \"bound_item\": \"alice@example.com\",
    \"label\": \"manual-test\"
  }")
check "POST /v1/instances → 201" "201" "$HTTP_CODE"
REGISTERED_FID=$(json_val "$WORKDIR/resp.json" "['fid']")
check "  FID matches" "$FID" "$REGISTERED_FID"

echo ""
echo "--- 4.3 List instances ---"
HTTP_CODE=$(curl -s -o "$WORKDIR/resp.json" -w "%{http_code}" \
  "$BASE/v1/instances" -H "$AUTH")
check "GET /v1/instances → 200" "200" "$HTTP_CODE"
COUNT=$(json_len "$WORKDIR/resp.json")
check "  instance count = 1" "1" "$COUNT"
RESP_PK=$(json_val "$WORKDIR/resp.json" "[0]['public_key']")
check "  public_key is hex (not base64)" "$PUB_KEY" "$RESP_PK"

echo ""
echo "--- 4.4 Get instance detail ---"
HTTP_CODE=$(curl -s -o "$WORKDIR/resp.json" -w "%{http_code}" \
  "$BASE/v1/instances/$FID" -H "$AUTH")
check "GET /v1/instances/$FID → 200" "200" "$HTTP_CODE"

########################################################################
echo ""
echo "=============================================="
echo "  Phase 5: Client fetches secrets"
echo "=============================================="

echo ""
echo "--- 5.1 jingui read (single secret) ---"
CLIENT_ID=$("$WORKDIR/jingui" read \
  --server "$BASE" --insecure \
  --appkeys "$WORKDIR/appkeys.json" \
  "jingui://my-gmail/alice@example.com/client_id" 2>/dev/null)
check "read client_id" "test-client-id.apps.googleusercontent.com" "$CLIENT_ID"

echo ""
echo "--- 5.2 jingui read (client_secret) ---"
CLIENT_SECRET=$("$WORKDIR/jingui" read \
  --server "$BASE" --insecure \
  --appkeys "$WORKDIR/appkeys.json" \
  "jingui://my-gmail/alice@example.com/client_secret" 2>/dev/null)
check "read client_secret" "test-client-secret-value" "$CLIENT_SECRET"

echo ""
echo "--- 5.3 jingui read (refresh_token) ---"
REFRESH=$("$WORKDIR/jingui" read \
  --server "$BASE" --insecure \
  --appkeys "$WORKDIR/appkeys.json" \
  "jingui://my-gmail/alice@example.com/refresh_token" 2>/dev/null)
check "read refresh_token" "ya29.super-secret-refresh-token-for-alice" "$REFRESH"

echo ""
echo "--- 5.4 jingui run (env injection + masking) ---"
cat > "$WORKDIR/test.env" <<'ENVEOF'
GMAIL_CLIENT_ID=jingui://my-gmail/alice@example.com/client_id
GMAIL_SECRET=jingui://my-gmail/alice@example.com/client_secret
GMAIL_REFRESH=jingui://my-gmail/alice@example.com/refresh_token
PLAIN_VAR=hello-world
ENVEOF

OUTPUT=$("$WORKDIR/jingui" run \
  --server "$BASE" --insecure \
  --appkeys "$WORKDIR/appkeys.json" \
  --env-file "$WORKDIR/test.env" \
  --no-lockdown \
  -- env 2>/dev/null)

# Check that the secrets were injected
echo "$OUTPUT" | grep -q "GMAIL_CLIENT_ID=" && check "env has GMAIL_CLIENT_ID" "yes" "yes" || check "env has GMAIL_CLIENT_ID" "yes" "no"
echo "$OUTPUT" | grep -q "GMAIL_SECRET=" && check "env has GMAIL_SECRET" "yes" "yes" || check "env has GMAIL_SECRET" "yes" "no"
echo "$OUTPUT" | grep -q "GMAIL_REFRESH=" && check "env has GMAIL_REFRESH" "yes" "yes" || check "env has GMAIL_REFRESH" "yes" "no"
echo "$OUTPUT" | grep -q "PLAIN_VAR=hello-world" && check "env has PLAIN_VAR" "yes" "yes" || check "env has PLAIN_VAR" "yes" "no"

echo ""
echo "--- 5.5 jingui run (secret masking in output) ---"
MASKED=$("$WORKDIR/jingui" run \
  --server "$BASE" --insecure \
  --appkeys "$WORKDIR/appkeys.json" \
  --env-file "$WORKDIR/test.env" \
  --no-lockdown \
  -- sh -c 'echo "token=$GMAIL_REFRESH"' 2>/dev/null)

if echo "$MASKED" | grep -q "REDACTED_BY_JINGUI"; then
  check "secret value masked in stdout" "yes" "yes"
else
  check "secret value masked in stdout" "yes" "no"
  echo "  actual output: $MASKED"
fi

if echo "$MASKED" | grep -q "ya29.super-secret"; then
  check "plaintext NOT in stdout" "no" "yes"
else
  check "plaintext NOT in stdout" "no" "no"
fi

########################################################################
echo ""
echo "=============================================="
echo "  Phase 6: Admin CRUD - delete operations"
echo "=============================================="

echo ""
echo "--- 6.1 DELETE app blocked by FK (no cascade) ---"
HTTP_CODE=$(curl -s -o "$WORKDIR/resp.json" -w "%{http_code}" \
  -X DELETE "$BASE/v1/apps/my-gmail" -H "$AUTH")
check "DELETE /v1/apps/my-gmail → 409" "409" "$HTTP_CODE"

echo ""
echo "--- 6.2 DELETE instance ---"
HTTP_CODE=$(curl -s -o "$WORKDIR/resp.json" -w "%{http_code}" \
  -X DELETE "$BASE/v1/instances/$FID" -H "$AUTH")
check "DELETE /v1/instances/$FID → 200" "200" "$HTTP_CODE"

echo ""
echo "--- 6.3 DELETE secret (no deps after instance removed) ---"
HTTP_CODE=$(curl -s -o "$WORKDIR/resp.json" -w "%{http_code}" \
  -X DELETE "$BASE/v1/secrets/my-gmail/alice@example.com" -H "$AUTH")
check "DELETE secret → 200" "200" "$HTTP_CODE"

echo ""
echo "--- 6.4 DELETE app (no deps remaining) ---"
HTTP_CODE=$(curl -s -o "$WORKDIR/resp.json" -w "%{http_code}" \
  -X DELETE "$BASE/v1/apps/my-gmail" -H "$AUTH")
check "DELETE /v1/apps/my-gmail → 200" "200" "$HTTP_CODE"

echo ""
echo "--- 6.5 Verify everything is gone ---"
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" "$BASE/v1/apps/my-gmail" -H "$AUTH")
check "app gone → 404" "404" "$HTTP_CODE"
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" "$BASE/v1/instances/$FID" -H "$AUTH")
check "instance gone → 404" "404" "$HTTP_CODE"
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" "$BASE/v1/secrets/my-gmail/alice@example.com" -H "$AUTH")
check "secret gone → 404" "404" "$HTTP_CODE"

########################################################################
echo ""
echo "=============================================="
echo "  Phase 7: Cascade delete"
echo "=============================================="

echo ""
echo "--- 7.1 Recreate full chain: app → secret → instance ---"
curl -s -o /dev/null -X POST "$BASE/v1/apps" -H "$AUTH" -H "Content-Type: application/json" \
  -d '{"app_id":"cascade-app","name":"Cascade Test","service_type":"gmail",
       "credentials_json":{"installed":{"client_id":"cid","client_secret":"cs","redirect_uris":["http://localhost"]}}}'
curl -s -o /dev/null -X PUT "$BASE/v1/credentials/cascade-app" -H "$AUTH" -H "Content-Type: application/json" \
  -d '{"item":"bob@example.com","secrets":{"refresh_token":"tok"}}'
# Reuse same client key
curl -s -o /dev/null -X POST "$BASE/v1/instances" -H "$AUTH" -H "Content-Type: application/json" \
  -d "{\"public_key\":\"$PUB_KEY\",\"bound_vault\":\"cascade-app\",\"bound_item\":\"bob@example.com\"}"
echo "  Created cascade-app → bob@example.com → instance"

echo ""
echo "--- 7.2 DELETE app with cascade ---"
HTTP_CODE=$(curl -s -o "$WORKDIR/resp.json" -w "%{http_code}" \
  -X DELETE "$BASE/v1/apps/cascade-app?cascade=true" -H "$AUTH")
check "DELETE cascade-app?cascade=true → 200" "200" "$HTTP_CODE"

echo ""
echo "--- 7.3 Verify cascade cleaned everything ---"
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" "$BASE/v1/apps/cascade-app" -H "$AUTH")
check "app gone → 404" "404" "$HTTP_CODE"
curl -s -o "$WORKDIR/resp.json" "$BASE/v1/secrets?vault=cascade-app" -H "$AUTH"
SECRETS_COUNT=$(json_len "$WORKDIR/resp.json")
check "no secrets remain" "0" "$SECRETS_COUNT"
curl -s -o "$WORKDIR/resp.json" "$BASE/v1/instances" -H "$AUTH"
INST_COUNT=$(json_len "$WORKDIR/resp.json")
check "no instances remain" "0" "$INST_COUNT"

########################################################################
echo ""
echo "=============================================="
echo "  Phase 8: Auth enforcement"
echo "=============================================="

echo ""
echo "--- 8.1 All admin endpoints reject unauthenticated requests ---"
for EP in \
  "GET /v1/apps" \
  "GET /v1/apps/x" \
  "DELETE /v1/apps/x" \
  "GET /v1/instances" \
  "GET /v1/instances/x" \
  "DELETE /v1/instances/x" \
  "GET /v1/secrets" \
  "GET /v1/secrets/x/y" \
  "DELETE /v1/secrets/x/y"; do
  METHOD="${EP%% *}"
  PATH_="${EP#* }"
  HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" -X "$METHOD" "$BASE$PATH_")
  check "$METHOD $PATH_ → 401" "401" "$HTTP_CODE"
done

########################################################################
echo ""
echo "=============================================="
printf "  Results: \033[32m%d passed\033[0m, \033[31m%d failed\033[0m\n" "$pass" "$fail"
echo "=============================================="
[ "$fail" -eq 0 ] || exit 1
