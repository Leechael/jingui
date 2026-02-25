# Jingui (金匮)

Zero-trust secret injection engine for AI Agents running in Trusted Execution Environments (TEEs).

Jingui ensures that secrets (API keys, OAuth tokens, credentials) are delivered to TEE workloads without the application—or any of its dependencies—ever having direct access to plaintext values outside the encrypted channel.

## How It Works

```
┌─────────────┐         challenge/response         ┌─────────────────┐
│  jingui      │◄────────── ECIES (X25519) ────────►│  jingui-server   │
│  (TEE client)│                                    │  (management)    │
└──────┬───────┘                                    └─────────────────┘
       │
       │  env vars + stdout/stderr masking
       ▼
┌─────────────┐
│  your app   │  ← secrets injected, ptrace blocked
└─────────────┘
```

1. **Server** stores encrypted credentials and manages OAuth flows.
2. **Client** runs inside the TEE, proves possession of its private key via a challenge-response protocol, receives secrets encrypted to its public key, decrypts them, and injects them as environment variables into the target process.
3. **Lockdown** — on Linux/amd64, the child process is hardened with seccomp filters that block `ptrace` and `process_vm_readv`, plus `PR_SET_DUMPABLE=0`.
4. **Output masking** — all secret values are redacted from stdout/stderr using Aho-Corasick multi-pattern matching.

## Quick Start

### Server

```bash
export JINGUI_MASTER_KEY="$(openssl rand -hex 32)"   # 64 hex chars
export JINGUI_ADMIN_TOKEN="$(openssl rand -hex 16)"   # ≥16 chars
jingui-server
```

Or with Docker:

```bash
docker run -d \
  -e JINGUI_MASTER_KEY="..." \
  -e JINGUI_ADMIN_TOKEN="..." \
  -v jingui-data:/data \
  -p 8080:8080 \
  ghcr.io/<owner>/jingui-server:latest
```

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `JINGUI_MASTER_KEY` | Yes | — | 64 hex characters (32-byte AES key for at-rest encryption) |
| `JINGUI_ADMIN_TOKEN` | Yes | — | Bearer token for admin APIs (min 16 chars) |
| `JINGUI_DB_PATH` | No | `jingui.db` | SQLite database path |
| `JINGUI_LISTEN_ADDR` | No | `:8080` | Listen address |
| `JINGUI_BASE_URL` | No | `http://localhost:8080` | Public URL for OAuth callbacks |
| `JINGUI_RATLS_STRICT` | No | `true` | Require client/server attestation exchange in challenge/fetch flow |

### Client

Create a `.env` file with secret references:

```env
GMAIL_CLIENT_ID=jingui://my-gmail/user@example.com/client_id
GMAIL_CLIENT_SECRET=jingui://my-gmail/user@example.com/client_secret
GMAIL_REFRESH_TOKEN=jingui://my-gmail/user@example.com/refresh_token
DATABASE_URL=postgres://localhost/mydb
```

Run your application:

```bash
jingui run --server https://jingui.example.com -- python app.py
```

Check local instance status and registration:

```bash
jingui status --server https://jingui.example.com
```

Read one secret (metadata is hidden by default):

```bash
jingui read --server https://jingui.example.com 'jingui://my-gmail/user@example.com/client_id'
# use --show-meta to print FID/Public Key to stderr for debugging
```

Lines with `jingui://` URIs are fetched and decrypted; plain values pass through unchanged.

| Flag | Default | Description |
|------|---------|-------------|
| `--server` | `JINGUI_SERVER_URL` env | Server URL (required) |
| `--appkeys` | `/dstack/.host-shared/.appkeys.json` | Path to X25519 private key file |
| `--env-file` | `.env` | Environment file with secret refs |
| `--insecure` | `false` | Allow plaintext HTTP |
| `--no-lockdown` | `false` | Disable seccomp hardening |

`jingui read` also supports `--show-meta` to print FID/Public Key to stderr when debugging.

RA-TLS strict client knobs:
- `JINGUI_RATLS_STRICT` (default `true`)
- `JINGUI_RATLS_EXPECT_SERVER_APP_ID` (optional pin; when set, server attestation app_id must match)

## Secret Reference Format

```
jingui://<app_id>/<user_id>/<field_name>
```

Examples:

- `jingui://my-gmail/user@example.com/client_id`
- `jingui://my-gmail/user@example.com/client_secret`
- `jingui://my-gmail/user@example.com/refresh_token`

> Note: migration to `jingui://<service>/<slug>/<field>` is planned, but current stable implementation and tests use `<app_id>/<user_id>/<field>`.

## Security Model

- **In transit** — ECIES (X25519 + AES-256-GCM). Secrets are encrypted to the TEE instance's public key.
- **At rest** — AES-256-GCM with the server master key.
- **Proof of possession** — before returning secrets, the server issues a nonce encrypted to the TEE's public key. Only the holder of the matching private key can decrypt and respond.
- **Process isolation** — seccomp BPF blocks ptrace/process_vm_readv; `PR_SET_DUMPABLE=0` prevents core dumps.
- **Output redaction** — Aho-Corasick streaming replacement masks leaked values in stdout/stderr.

## Building

```bash
make build          # current platform
make build-all      # cross-compile all 8 binaries (2 × 4 platforms)
make ci             # lint + test + bdd
```

RA-TLS verifier path (dcap-qvl linked build):

```bash
go build -tags ratls -o bin/jingui-server ./cmd/jingui-server
go build -tags ratls -o bin/jingui ./cmd/jingui
```

> Without `-tags ratls`, attestation verifier runs in stub mode and strict RA-TLS requests will fail closed.

Docker:

```bash
docker build --target server -t jingui-server .
docker build --target client -t jingui .
```

## API Overview

- OpenAPI JSON: `/openapi.json` (also committed as `docs/openapi.json`)

**Admin endpoints** (require `Authorization: Bearer <ADMIN_TOKEN>`):

### App management

| Method | Path | Description |
|--------|------|-------------|
| POST | `/v1/apps` | Register a workload app (CVM/agent app identity) |
| PUT | `/v1/apps/:app_id` | Update app metadata/credentials |
| GET | `/v1/apps` | List workload apps (metadata only) |
| GET | `/v1/apps/:app_id` | Get workload app metadata |
| DELETE | `/v1/apps/:app_id` | Delete workload app (`?cascade=true` to delete dependent secrets/instances) |

### Instance management

| Method | Path | Description |
|--------|------|-------------|
| POST | `/v1/instances` | Register a TEE instance (public key + binding) |
| GET | `/v1/instances` | List registered TEE instances |
| GET | `/v1/instances/:fid` | Get instance details |
| DELETE | `/v1/instances/:fid` | Delete an instance |

### User-secret management

| Method | Path | Description |
|--------|------|-------------|
| GET | `/v1/user-secrets` | List user-secret metadata (supports `?app_id=` filter) |
| GET | `/v1/user-secrets/:app_id/:user_id` | Get one user-secret metadata record |
| DELETE | `/v1/user-secrets/:app_id/:user_id` | Delete user secret (`?cascade=true` deletes dependent instances) |

### Debug policy APIs (runtime user-level read control)

| Method | Path | Description |
|--------|------|-------------|
| GET | `/v1/debug-policy/:app_id/:user_id` | Get whether `jingui read` is allowed for this user |
| PUT | `/v1/debug-policy/:app_id/:user_id` | Update `allow_read_debug` at runtime |

### Credential APIs

| Method | Path | Description |
|--------|------|-------------|
| PUT | `/v1/credentials/:app_id` | Store secrets directly |
| GET | `/v1/credentials/gateway/:app_id` | Start OAuth authorization flow |
| POST | `/v1/credentials/device/:app_id` | Start OAuth device flow |
| GET | `/v1/credentials/callback` | OAuth callback endpoint |

**Client endpoints**:

| Method | Path | Description |
|--------|------|-------------|
| POST | `/v1/secrets/challenge` | Request proof-of-possession challenge |
| POST | `/v1/secrets/fetch` | Fetch encrypted secrets (after challenge) |

## Manual verification

- Full end-to-end script: `scripts/manual-test.sh`
- Step-by-step guide: `docs/manual-test-guide.md`

## Planned refactor (in progress)

- Correct data model semantics:
  - `app_id` is workload identity (CVM/agent app), not provider/service name.
  - Secret references use `jingui://<service>/<slug>/<field>` and do not carry `app_id`.
- Execution plan:
  1. Refactor DB schema and CRUD first (single-step migration; no backward-compat layer).
  2. Keep server-client flow working with challenge-response during refactor.
  3. Introduce RA-TLS-based identity binding in next phase without changing ref syntax.

## License

See [LICENSE](LICENSE) for details.
