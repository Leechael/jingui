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
| `JINGUI_LOG_LEVEL` | No | `info` | Log level (`debug`,`info`,`warn`,`error`) for RA-TLS handshake diagnostics |

### Client

Create a `.env` file with secret references:

```env
GMAIL_CLIENT_ID=jingui://my-gmail/alice@example.com/client_id
GMAIL_CLIENT_SECRET=jingui://my-gmail/alice@example.com/client_secret
GMAIL_REFRESH_TOKEN=jingui://my-gmail/alice@example.com/refresh_token
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
jingui read --server https://jingui.example.com 'jingui://my-gmail/alice@example.com/client_id'
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
| `--verbose` | `false` | Enable verbose debug logs (same as `--log-level debug`) |
| `--log-level` | `JINGUI_LOG_LEVEL` / `info` | Log level (`debug`,`info`,`warn`,`error`) |

`jingui read` also supports `--show-meta` to print FID/Public Key to stderr when debugging.

RA-TLS strict client knobs:
- `JINGUI_RATLS_STRICT` (default `true`)
- `JINGUI_RATLS_EXPECT_SERVER_APP_ID` (optional pin; when set, server attestation app_id must match)
- `JINGUI_LOG_LEVEL=debug` (or `--verbose`) to print RA verification measurements (MR/RTMR/TCB status)

## Secret Reference Format

```
jingui://<vault>/<item>/<field_name>
jingui://<vault>/<item>/<section>/<field_name>
```

- `<vault>` — app/service namespace (e.g. `my-gmail`)
- `<item>` — item within the vault (e.g. `alice@example.com`)
- `<section>` — optional subsection (e.g. `oauth`)
- `<field_name>` — field within the secret object (e.g. `client_id`)

Examples:

- `jingui://my-gmail/alice@example.com/client_id`
- `jingui://my-gmail/alice@example.com/client_secret`
- `jingui://my-gmail/alice@example.com/refresh_token`
- `jingui://my-gmail/alice@example.com/oauth/access_token` (4-segment)

## Security Model

- **In transit** — ECIES (X25519 + AES-256-GCM). Secrets are encrypted to the TEE instance's public key.
- **At rest** — AES-256-GCM with the server master key.
- **Proof of possession** — before returning secrets, the server issues a nonce encrypted to the TEE's public key. Only the holder of the matching private key can decrypt and respond.
- **Process isolation** — seccomp BPF blocks ptrace/process_vm_readv; `PR_SET_DUMPABLE=0` prevents core dumps.
- **Output redaction** — Aho-Corasick streaming replacement masks leaked values in stdout/stderr.

## Building

RA-TLS attestation verification requires the dcap-qvl static library (built from Rust). CI builds it automatically; for local development see `scripts/build-dcap-qvl.sh` or the Dockerfile.

```bash
make build          # current platform (requires dcap-qvl)
make build-all      # cross-compile all 8 binaries (2 × 4 platforms)
make ci             # lint + test + bdd
```

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

### Secret management

| Method | Path | Description |
|--------|------|-------------|
| GET | `/v1/secrets` | List secret metadata (supports `?vault=` filter) |
| GET | `/v1/secrets/:vault/:item` | Get one secret metadata record |
| DELETE | `/v1/secrets/:vault/:item` | Delete secret (`?cascade=true` deletes dependent instances) |

### Debug policy APIs (runtime item-level read control)

| Method | Path | Description |
|--------|------|-------------|
| GET | `/v1/debug-policy/:vault/:item` | Get whether `jingui read` is allowed for this item |
| PUT | `/v1/debug-policy/:vault/:item` | Update `allow_read_debug` at runtime |

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

## License

See [LICENSE](LICENSE) for details.
