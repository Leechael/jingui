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

1. **Server** stores secrets in vaults and manages TEE instance access.
2. **Client** runs inside the TEE, proves possession of its private key via a challenge-response protocol, receives secrets encrypted to its public key, decrypts them, and injects them as environment variables into the target process.
3. **Lockdown** — on Linux/amd64, the child process is hardened with seccomp filters that block `ptrace` and `process_vm_readv`, plus `PR_SET_DUMPABLE=0`.
4. **Output masking** — all secret values are redacted from stdout/stderr using Aho-Corasick multi-pattern matching.

## Quick Start

### Server

```bash
export JINGUI_ADMIN_TOKEN="$(openssl rand -hex 16)"   # ≥16 chars
jingui-server
```

Or with Docker:

```bash
docker run -d \
  -e JINGUI_ADMIN_TOKEN="..." \
  -v jingui-data:/data \
  -p 8080:8080 \
  ghcr.io/<owner>/jingui-server:latest
```

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `JINGUI_ADMIN_TOKEN` | Yes | — | Bearer token for admin APIs (min 16 chars) |
| `JINGUI_DB_PATH` | No | `jingui.db` | SQLite database path |
| `JINGUI_LISTEN_ADDR` | No | `:8080` | Listen address |
| `JINGUI_CORS_ORIGINS` | No | — | Comma-separated allowed CORS origins (for admin panel dev) |
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

Lines with `jingui://` (or `op://`) URIs are fetched and decrypted; plain values pass through unchanged.

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

`op://` is also accepted as an alias (1Password CLI compatible):

```
op://<vault>/<item>/<field_name>
op://<vault>/<item>/<section>/<field_name>
```

- `<vault>` — app/service namespace (e.g. `my-gmail`)
- `<item>` — item within the vault (e.g. `alice@example.com`)
- `<section>` — optional subsection (e.g. `oauth`)
- `<field_name>` — field within the secret object (e.g. `client_id`)

Examples:

- `jingui://my-gmail/alice@example.com/client_id`
- `op://my-gmail/alice@example.com/client_secret`
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
- Database schema: `docs/schema.md`

**Admin endpoints** (require `Authorization: Bearer <ADMIN_TOKEN>`):

### Vault management

| Method | Path | Description |
|--------|------|-------------|
| POST | `/v1/vaults` | Create a vault |
| GET | `/v1/vaults` | List vaults |
| GET | `/v1/vaults/:id` | Get vault |
| PUT | `/v1/vaults/:id` | Update vault name |
| DELETE | `/v1/vaults/:id` | Delete vault (`?cascade=true` to delete items + access grants) |

### Vault items

| Method | Path | Description |
|--------|------|-------------|
| GET | `/v1/vaults/:id/items` | List sections in a vault |
| GET | `/v1/vaults/:id/items/:section` | Get field keys for a section |
| PUT | `/v1/vaults/:id/items/:section` | Upsert/delete fields (`{fields: {k:v}, delete: [k]}`) |
| DELETE | `/v1/vaults/:id/items/:section` | Delete all fields in a section |

### Vault ↔ Instance access

| Method | Path | Description |
|--------|------|-------------|
| GET | `/v1/vaults/:id/instances` | List instances with access to this vault |
| POST | `/v1/vaults/:id/instances/:fid` | Grant instance access to vault |
| DELETE | `/v1/vaults/:id/instances/:fid` | Revoke instance access to vault |

### Instance management

FID (Fingerprint ID) = `hex(SHA1(public_key))` — a 40-char hex identifier derived from the instance's X25519 public key.

| Method | Path | Description |
|--------|------|-------------|
| POST | `/v1/instances` | Register a TEE instance (public key + dstack_app_id) |
| GET | `/v1/instances` | List all instances |
| GET | `/v1/instances/:fid` | Get instance details |
| PUT | `/v1/instances/:fid` | Update `dstack_app_id` and `label` |
| DELETE | `/v1/instances/:fid` | Delete an instance |

### Debug policy

Per vault+instance pair control over `jingui read`.

| Method | Path | Description |
|--------|------|-------------|
| GET | `/v1/debug-policy/:vault/:fid` | Get debug-read policy (defaults to allow) |
| PUT | `/v1/debug-policy/:vault/:fid` | Set `allow_read` for vault+instance |

**Client endpoints** (no admin auth):

| Method | Path | Description |
|--------|------|-------------|
| POST | `/v1/secrets/challenge` | Request proof-of-possession challenge |
| POST | `/v1/secrets/fetch` | Fetch encrypted secrets (after challenge) |

## dstack Platform Constraints

- **App keys path**: `/dstack/.host-shared/.appkeys.json` is the default location for the X25519 private key file, determined by the dstack runtime environment.
- **Key format**: X25519 (Curve25519) key pairs; ECIES encryption uses X25519 + AES-256-GCM.
- **`dstack_app_id`**: Application identity from the dstack attestation chain, used for RA-TLS verification during the challenge/fetch flow.

## Web Admin Panel

Jingui includes a single-page admin panel (`web/`) for managing vaults, items, and instances through the browser. It is built separately and served as static files. Set `JINGUI_CORS_ORIGINS` to allow cross-origin requests during development.

## License

See [LICENSE](LICENSE) for details.
