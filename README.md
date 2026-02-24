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

### Client

Create a `.env` file with secret references:

```env
GMAIL_TOKEN=jingui://gmail/user@example.com/token
GMAIL_WORK_TOKEN=jingui://gmail/work/token
DATABASE_URL=postgres://localhost/mydb
```

Run your application:

```bash
jingui run --server https://jingui.example.com -- python app.py
```

Lines with `jingui://` URIs are fetched and decrypted; plain values pass through unchanged.

| Flag | Default | Description |
|------|---------|-------------|
| `--server` | `JINGUI_SERVER_URL` env | Server URL (required) |
| `--appkeys` | `.appkeys.json` | Path to X25519 private key file |
| `--env-file` | `.env` | Environment file with secret refs |
| `--insecure` | `false` | Allow plaintext HTTP |
| `--no-lockdown` | `false` | Disable seccomp hardening |

## Secret Reference Format

```
jingui://<service>/<slug_or_email>/<field_name>
```

Examples:

- `jingui://gmail/foo@example.com/token`
- `jingui://gmail/work/token`

`app_id` is **not** encoded in secret references. In the target design, workload identity (`app_id`) comes from TEE attestation (RA-TLS), while the reference only selects a secret namespace inside that workload.

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

Docker:

```bash
docker build --target server -t jingui-server .
docker build --target client -t jingui .
```

## API Overview

**Admin endpoints** (require `Authorization: Bearer <ADMIN_TOKEN>`):

### App management

| Method | Path | Description |
|--------|------|-------------|
| POST | `/v1/apps` | Register a workload app (CVM/agent app identity) |
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
