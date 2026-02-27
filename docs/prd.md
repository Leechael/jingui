# Jingui: Product Requirements and Technical Specification

---

## 0. Document Notes

**Important:** This document is intended to be a complete technical and product specification that can be used directly by the engineering team. It combines information from three core sources:

1. **Credential Vault Product Design (PD)**: core client/server architecture, data model, and APIs.
2. **Encrypted Environment Variables Specification**: ECIES encryption and TEE key management.
3. **1Password CLI research**: developer experience ideas, especially `op run`, secret references, and output redaction.

To preserve technical accuracy, this document directly includes key technical specs from source materials and extends them for productization.

---

## Current Implementation Status (as of 2026-02-27)

To avoid ambiguity between target design and shipped behavior, this snapshot is authoritative for the current codebase:

- Secret reference format: `jingui://<vault>/<item>/<field>` (3-segment) or `jingui://<vault>/<item>/<section>/<field>` (4-segment)
- `vault_items` keyspace: `(app_id, item)`
- `jingui inject` is planned, not implemented in current CLI
- Challenge-response (`/v1/secrets/challenge` + `/v1/secrets/fetch`) is implemented and required for fetch
- RA-TLS strict mode with bidirectional attestation is implemented

## 1. Product Overview

### 1.1 Vision and Positioning

**Jingui** is a zero-trust secret management and injection engine designed for **AI Agents** running inside **Trusted Execution Environments (TEE)**.
Its mission is to let AI agents use required credentials (API keys, DB passwords, etc.) while fundamentally reducing the chance that the agent itself—or any subprocess it runs—can inspect, steal, or leak those credentials.

Jingui is not a traditional human-operated secrets tool. It is an automated, non-interactive security component embedded into confidential-computing environments.

### 1.2 Core Problem

As AI agents become more powerful and autonomous, they must access more sensitive credentials. This creates several security challenges:

1. **Agent trustworthiness**: how do we trust an LLM-driven agent not to intentionally or accidentally leak secrets?
2. **Supply-chain risk**: third-party dependencies and external tools used by agents may contain vulnerabilities.
3. **TEE secret delivery**: how to securely and efficiently provide runtime secrets inside strongly isolated TEE environments.
4. **Output leakage**: agents may accidentally print secrets to stdout/stderr in logs, reports, or user-facing output.

### 1.3 Target Users and Environment

- **Primary user**: AI agent application/process.
- **Runtime environment**: confidential computing environments such as Intel TDX or AMD SEV-SNP.
- **Operators/deployers**: AI platform or application engineering/ops teams.

---

## 2. System Architecture

This architecture is derived from the original Credential Vault PD and adapted for AI agents in TEE.

### 2.1 High-Level Architecture

```
┌─────────────────────────────────────────────────────────┐
│                    Jingui Server                        │
│          (Central credential vault and dispatcher)      │
└──────────────────────────┬──────────────────────────────┘
                           │  Encrypted secrets (ECIES ciphertext)
                           ▼
┌───────────────────────────────────────────────────────────────────┐
│                     Trusted Execution Environment (TEE)           │
│                                                                   │
│  ┌────────────────────┐      ┌────────────────────────────────┐  │
│  │  Jingui Client     │      │      AI Agent Process         │  │
│  │ (decrypt/inject/   │      │                                │  │
│  │  monitor engine)   │      │                                │  │
│  └──────────┬─────────┘      └───────────────┬────────────┘  │
│             │ 1. decrypt and inject env vars  │ 2. launch/monitor│
│             └───────────────┬─────────────────┘                │
│                             │                                  │
│                       ┌─────┴─────┐                            │
│                       │ stdout pipe │ (automatic redaction)    │
│                       └─────┬─────┘                            │
│                             │                                  │
└─────────────────────────────┼───────────────────────────────────┘
                              ▼
                        Safe Output
```

### 2.2 Component Responsibilities

#### 2.2.1 Jingui Server

- **App management**: manage apps and app credentials (for example OAuth `credentials.json`).
- **OAuth gateway**: provide authorization flow for human users to obtain/store `refresh_token`.
- **TEE instance registration**: maintain registry mapping FID to public key.
- **On-demand encrypted dispatch**: encrypt requested secrets using registered TEE public key (ECIES) and return.

#### 2.2.2 Jingui Client

A lightweight non-interactive Go binary acting as the agent’s **secure launcher**.

- **Single entrypoint**: `jingui run <agent_command> [args...]`
- **Core responsibilities**:
  1. Read TEE-provisioned config (`.appkeys.json`) to get instance private key.
  2. Fetch encrypted secret bundle from server.
  3. Decrypt secrets in memory.
  4. Launch `<agent_command>` as subprocess.
  5. Inject secrets into subprocess env and block subprocess from reading sensitive process env sources.
  6. Intercept stdout/stderr and redact secret values in real time.

---

## 3. Client Design (Inspired by 1Password)

Although Jingui targets non-human AI agents, its UX model is inspired by **1Password CLI**, especially `op run`, to provide seamless and secure execution in TEE.

### 3.1 CLI Surface

| Command | 1Password Equivalent | Purpose |
| :--- | :--- | :--- |
| `jingui run -- <cmd>` | `op run -- <cmd>` | **Core function**. Launch subprocess, inject fetched secrets as env vars, and redact subprocess stdout/stderr. |
| `jingui read <secret_ref>` | `op read <secret_ref>` | Read a single secret and print it. Metadata hidden by default; `--show-meta` enables debug FID/Public Key output. |
| `jingui status` | — | Print current instance info (FID/Public Key) and registration state for troubleshooting. |
| `jingui inject` | `op inject` | Planned command (not in current released CLI). |

### 3.2 Secret Reference Syntax

Use URI-style references:

`jingui://<vault>/<item>/<field_name>` or `jingui://<vault>/<item>/<section>/<field_name>`

- `jingui://`: protocol prefix.
- `<vault>`: vault namespace (e.g. app/service name).
- `<item>`: item within the vault (e.g. authorized email).
- `<section>`: optional subsection (e.g. `oauth`).
- `<field_name>`: field within secret object (`refresh_token`, `client_id`, etc).

Examples:

```
GOG_CLIENT_ID="jingui://gmail-app/user@example.com/client_id"
GOG_CLIENT_SECRET="jingui://gmail-app/user@example.com/client_secret"
GOG_REFRESH_TOKEN="jingui://gmail-app/user@example.com/refresh_token"
```

### 3.3 `jingui run` Workflow and Security

1. Parse command and args after `jingui run`.
2. Scan current env for `jingui://` references.
3. Batch-fetch all references in one server request.
4. Decrypt in memory and inject resolved values into child env.
5. Launch child process:
   - fork child;
   - before `execve`, apply `ptrace` + `seccomp-bpf` policy to block sensitive sources like `/proc/self/environ`;
   - redirect child stdout/stderr to pipes.
6. Redact output in parent process using Aho-Corasick; replace matched secret plaintext with `[REDACTED_BY_JINGUI]`.

### 3.4 Configuration File (`.appkeys.json`)

The client reads one configuration source injected at TEE boot. **Format and source follow `encrypted-env-spec.md` strictly.**

- **Path**: predetermined TEE path, e.g. `/dstack/.host-shared/.appkeys.json`
- **Key field**: `env_crypt_key` (X25519 private key, 32 bytes, hex-encoded)
- **Source**: produced/injected by TEE launcher or orchestrator during instance creation via KMS + remote attestation

---

## 4. Server Design

Jingui Server is the central authority for credential authorization and distribution. This section keeps original PD intent while aligning terminology.

### 4.1 Core Responsibilities (Revised)

- **Workload app management**: manage CVM/Agent workload identity. Each workload has unique `app_id` (dstack semantics), which is the authorization boundary key.
- **Vault item management**: manage per-item secrets for external services (e.g. Gmail), organized by `(app_id, item)`.
- **TEE instance registration**: maintain instance identity and key material. Current phase uses FID+public key binding; next phase introduces RA-TLS attestation.
- **On-demand encrypted dispatch**: resolve secret references under instance-bound identity; decrypt at rest data and re-encrypt to requester public key via ECIES.
- **Encryption at rest**: all persisted sensitive data uses server master key encryption.

### 4.2 Data Model (Database Schema)

> Current implementation uses `apps(app_id, service_type, required_scopes, credentials_encrypted)` and `vault_items(app_id, item, secret_encrypted)`.

#### 4.2.1 `apps` (workload applications)

```sql
CREATE TABLE apps (
    app_id TEXT PRIMARY KEY,                -- CVM/Agent app identity
    owner_user_id TEXT NOT NULL,
    name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

#### 4.2.2 `vault_items` (service credentials)

```sql
CREATE TABLE vault_items (
    app_id TEXT NOT NULL REFERENCES apps(app_id) ON DELETE CASCADE,
    item TEXT NOT NULL,                     -- e.g. foo@example.com / work
    secret_encrypted BYTEA NOT NULL,        -- encrypted JSON object
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (app_id, item)
);
```

#### 4.2.3 `tee_instances` (TEE instance registry)

```sql
CREATE TABLE tee_instances (
    fid TEXT PRIMARY KEY,
    public_key BYTEA NOT NULL UNIQUE,
    bound_app_id TEXT NOT NULL REFERENCES apps(app_id),
    bound_attestation_app_id TEXT NOT NULL DEFAULT '',
    bound_item TEXT NOT NULL,
    label TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_used_at TIMESTAMPTZ,

    -- Reserved for next phase: RA-TLS / attestation metadata
    attestation_mode TEXT,
    tee_type TEXT,
    mrtd BYTEA,
    rtmr0 BYTEA,
    rtmr1 BYTEA,
    rtmr2 BYTEA,
    rtmr3 BYTEA,
    quote_hash BYTEA,
    cert_fingerprint BYTEA,
    verified_at TIMESTAMPTZ
);
```

### 4.3 API Specification (gRPC & REST)

The server should expose both gRPC and REST APIs.

| Service (gRPC) | Method (RPC) | Path (REST) | Caller | Description |
| :--- | :--- | :--- | :--- | :--- |
| `AppService` | `RegisterApp` | `POST /v1/apps` | Human admin | Register workload app (`app_id`). |
| `AppService` | `UpdateApp` | `PUT /v1/apps/{app_id}` | Human admin | Update existing app metadata/credentials. |
| `CredentialService` | `PutCredential` | `PUT /v1/credentials/{app_id}` | Human admin | Write vault item secret (`vault`/`item`). |
| `SecretService` | `FetchSecrets` | `POST /v1/secrets/fetch` | **TEE client** | **Core API**. TEE instance batch-fetches encrypted credentials based on bound identity. |
| `InstanceService` | `RegisterInstance` | `POST /v1/instances` | KMS / ops script | Register TEE instance and bind vault/item identity. |
| `InstanceService` | `UpdateInstance` | `PUT /v1/instances/{fid}` | Human admin | Update `bound_attestation_app_id` and `label` on an existing instance. |

#### Core API flow: `FetchSecrets`

1. **Request**: TEE client sends:
   ```json
   {
     "fid": "...",
     "secret_references": ["jingui://gmail-app/user@example.com/client_id", "jingui://gmail-app/user@example.com/refresh_token"]
   }
   ```
2. **Instance verification**: lookup `fid` in `tee_instances`; return 404 if not found.
3. **Reference parsing**: parse each secret reference into `vault`, `item`, `field_name`.
4. **Authorization**: resolve bound identity `(bound_app_id, bound_item)` by `fid`; authorize references under that namespace.
5. **Batch fetch and encrypt**:
   - read `secret_encrypted` from `vault_items` using `(app_id, item)`;
   - decrypt using server master key and extract `field_name`;
   - ECIES-encrypt with instance public key.
6. **Response**:
   ```json
   {
     "secrets": {
       "jingui://gmail-app/user@example.com/client_id": "<ECIES_encrypted_blob_1>",
       "jingui://gmail-app/user@example.com/refresh_token": "<ECIES_encrypted_blob_2>"
     }
   }
   ```

_The following is directly included to preserve full encryption details for implementers._

## 5. Encryption and Security Model

### 5.1 Encryption Protocol Specification (from `encrypted-env-spec.md`)

**To ensure implementation accuracy, this section includes the full technical content without modifications.**

---

> # Encrypted Environment Variables — Technical Specification
> 
> ## 1. Overview
> 
> dstack uses an ECIES (Elliptic Curve Integrated Encryption Scheme) variant to protect application environment variables. The client encrypts environment variables using an X25519 public key, and the ciphertext is deployed alongside the application into a CVM (Confidential Virtual Machine). At boot time, the CVM obtains the corresponding private key from the KMS via TDX remote attestation and decrypts the variables inside the TEE.
> 
> ## 2. Encryption Public Key Source
> 
> ### 2.1 Key Derivation Chain
> 
> The KMS deterministically derives a per-application encrypt/decrypt key pair from its root CA key:
> 
> ```
> KMS root CA key (P-256 KeyPair)
>   │
>   └─ derive_dh_secret(context = [app_id, "env-encrypt-key"])
>        → SHA256(derived_P256_key_DER) → 32 bytes
>        → X25519 StaticSecret (private key = env_crypt_key, delivered to TEE)
>        → X25519 PublicKey (public key, exposed to client for encryption)
> ```
> 
> The same `app_id` always derives the same key pair. Keys are deterministic, not randomly generated.
> 
> ### 2.2 Computing `app_id`
> 
> ```
> app_id = SHA256(app-compose.json)[0..20]    // first 20 bytes, 40 hex characters
> ```
> 
> ### 2.3 RPC Interface for Obtaining the Public Key
> 
> The public key is exposed through a two-level RPC chain:
> 
> ```
> Client/UI  ──→  VMM (GetAppEnvEncryptPubKey)  ──→  KMS (GetAppEnvEncryptPubKey)
>                        pass-through proxy              actual key derivation
> ```
> 
> **Request**:
> 
> ```protobuf
> message AppId {
>   bytes app_id = 1;    // 20-byte app_id
> }
> ```
> 
> **Response**:
> 
> ```protobuf
> message PublicKeyResponse {
>   bytes public_key = 1;     // 32-byte X25519 public key
>   bytes signature = 2;      // Legacy k256 signature (no timestamp)
>   uint64 timestamp = 3;     // Unix timestamp in seconds when response was generated
>   bytes signature_v1 = 4;   // New k256 signature (with timestamp, replay-resistant)
> }
> ```
> 
> ### 2.4 Public Key Signature Verification
> 
> The response includes signatures generated by the KMS k256 (secp256k1) root key, allowing clients to verify the public key authenticity:
> 
> - **signature** (legacy): `sign(Keccak256("dstack-env-encrypt-pubkey" + ":" + app_id + public_key))`
> - **signature_v1** (new): `sign(Keccak256("dstack-env-encrypt-pubkey" + ":" + app_id + timestamp_be_bytes + public_key))`
> 
> ## 3. Encrypt/Decrypt Protocol
> 
> ### 3.1 Ciphertext Binary Format
> 
> ```
> Offset   Length      Content
> ───────────────────────────────────
> 0        32 bytes    ephemeral_public_key  (sender's ephemeral X25519 public key)
> 32       12 bytes    iv                    (AES-GCM nonce)
> 44       N+16 bytes  ciphertext + auth_tag (AES-GCM ciphertext + authentication tag)
> ```
> 
> ### 3.2 Plaintext Format
> 
> UTF-8 encoded JSON:
> 
> ```json
> {"env": [{"key": "FOO", "value": "bar"}, {"key": "SECRET", "value": "123"}]}
> ```
> 
> ### 3.3 Encryption Flow (Client-Side, at Deploy Time)
> 
> ```
> 1. plaintext     = JSON.encode({"env": [{"key": k, "value": v}, ...]})
> 2. ephemeral_sk  = X25519.random_private_key()               // 32 bytes
> 3. ephemeral_pk  = X25519.public_key(ephemeral_sk)            // 32 bytes
> 4. shared_secret = X25519.dh(ephemeral_sk, remote_public_key) // 32 bytes
> 5. iv            = random(12)                                  // 12 bytes
> 6. ciphertext    = AES-256-GCM.encrypt(
>                        key       = shared_secret,  // DH output used directly as AES key, no KDF
>                        nonce     = iv,
>                        plaintext = plaintext,
>                        aad       = None            // no associated data
>                    )
> 7. output        = ephemeral_pk || iv || ciphertext
> ```
> 
> ### 3.4 Decryption Flow (Inside TEE)
> 
> ```
> 1. ephemeral_pk   = data[0..32]
> 2. iv             = data[32..44]
> 3. ciphertext     = data[44..]       // includes 16-byte GCM auth tag
> 4. shared_secret  = X25519.dh(env_crypt_key, ephemeral_pk)  // 32 bytes
> 5. plaintext      = AES-256-GCM.decrypt(
>                         key        = shared_secret,
>                         nonce      = iv,
>                         ciphertext = ciphertext,
>                         aad        = None
>                     )
> 6. result         = JSON.decode(plaintext)  // → {"env": [...]} 
> ```
> 
> ### 3.5 Algorithm Parameters Summary
> 
> | Parameter | Value |
> |-----------|-------|
> | Key agreement | X25519 (RFC 7748), **not** ECDH P-256 |
> | Symmetric encryption | AES-256-GCM |
> | KDF | None — shared secret is used directly as the AES key |
> | IV / Nonce | 12 bytes, randomly generated |
> | AAD | None (no associated data) |
> | Auth tag | 16 bytes (GCM default), appended to ciphertext |
> | Key format | Raw 32 bytes, not PEM/DER |
> 
> ## 4. `.appkeys.json` File Specification
> 
> ### 4.1 File Location
> 
> Path inside TEE: `/dstack/.host-shared/.appkeys.json`
> 
> ### 4.2 JSON Structure
> 
> ```json
> {
>   "disk_crypt_key": "aabbccdd...",
>   "env_crypt_key": "0123456789abcdef...(64 hex chars)...",
>   "k256_key": "...",
>   "k256_signature": "...",
>   "gateway_app_id": "some-app-id",
>   "ca_cert": "-----BEGIN CERTIFICATE-----\n...",
>   "key_provider": {
>     "Kms": {
>       "url": "https://kms.example.com/prpc",
>       "pubkey": "...",
>       "tmp_ca_key": "-----BEGIN PRIVATE KEY-----\n...",
>       "tmp_ca_cert": "-----BEGIN CERTIFICATE-----\n..."
>     }
>   }
> }
> ```
> 
> ### 4.3 Field Descriptions
> 
> | Field | Rust Type | JSON Serialization | Description |
> |-------|-----------|-------------------|-------------|
> | `disk_crypt_key` | `Vec<u8>` | hex string | Disk encryption key |
> | `env_crypt_key` | `Vec<u8>` | hex string | **X25519 private key (32 bytes = 64 hex chars)**, may be absent |
> | `k256_key` | `Vec<u8>` | hex string | secp256k1 signing private key |
> | `k256_signature` | `Vec<u8>` | hex string | KMS signature of the k256 key |
> | `gateway_app_id` | `String` | plain string | Gateway application ID |
> | `ca_cert` | `String` | PEM string | CA certificate |
> | `key_provider` | tagged enum | see below | Key provider information |
> 
> **All `Vec<u8>` fields are serialized as hex strings in JSON** (via the `serde-human-bytes` crate using `hex::encode` / `hex::decode`, **not** base64).
> 
> ### 4.4 `key_provider` Field
> 
> This is a Rust tagged enum. Serde uses externally tagged format — an object with exactly one key:
> 
> ```json
> {"None":  {"key": "<PEM>"}}
> {"Local": {"key": "<PEM>", "mr": "<hex>"}}
> {"Tpm":   {"key": "<PEM>", "pubkey": "<hex>"}}
> {"Kms":   {"url": "...", "pubkey": "<hex>", "tmp_ca_key": "<PEM>", "tmp_ca_cert": "<PEM>"}}
> ```
> 
> ## 5. Language Implementation Guides
> 
> ### 5.1 Parsing `.appkeys.json`
> 
> **Go**:
> 
> ```go
> type AppKeys struct {
>     DiskCryptKey  string          `json:"disk_crypt_key"`   // hex string
>     EnvCryptKey   string          `json:"env_crypt_key"`    // hex string, may be empty
>     K256Key       string          `json:"k256_key"`         // hex string
>     K256Signature string          `json:"k256_signature"`   // hex string
>     GatewayAppId  string          `json:"gateway_app_id"`
>     CaCert        string          `json:"ca_cert"`
>     KeyProvider   json.RawMessage `json:"key_provider"`
> }
> 
> // Reading env_crypt_key:
> keyBytes, err := hex.DecodeString(appKeys.EnvCryptKey) // → []byte, len=32
> ```
> 
> ### 5.2 Decryption
> 
> **Go**:
> 
> ```go
> import (
>     "crypto/aes"
>     "crypto/cipher"
>     "fmt"
> 
>     "golang.org/x/crypto/curve25519"
> )
> 
> func Decrypt(envCryptKey [32]byte, data []byte) ([]byte, error) {
>     if len(data) < 44 {
>         return nil, fmt.Errorf("ciphertext too short")
>     }
>     ephPk := data[:32]
>     iv := data[32:44]
>     ct := data[44:]
> 
>     shared, err := curve25519.X25519(envCryptKey[:], ephPk)
>     if err != nil {
>         return nil, err
>     }
> 
>     block, err := aes.NewCipher(shared)
>     if err != nil {
>         return nil, err
>     }
>     gcm, err := cipher.NewGCM(block)
>     if err != nil {
>         return nil, err
>     }
>     return gcm.Open(nil, iv, ct, nil)
> }
> ```

---

### 5.2 Security Model and Threat Analysis

On top of the encryption protocol above, Jingui’s security posture depends on these assumptions and mitigations:

| Threat Scenario | Description | Jingui Mitigation |
| :--- | :--- | :--- |
| **Network sniffing** | Attacker intercepts traffic between TEE client and server. | Force HTTPS/TLS end-to-end. Secret bundles are additionally encrypted to TEE instance public key; without private key attacker cannot decrypt. |
| **Full server compromise** | Attacker gets root access to server and full database dump. | **Core defense**: server does not store TEE private keys. Attacker cannot impersonate TEE instance or decrypt already dispatched bundles. |
| **TEE memory exposure** | CPU/TEE vulnerabilities (e.g. Spectre/Meltdown class) attempt memory extraction. | This is a fundamental TEE challenge. Jingui relies on platform protections (e.g. Intel TDX memory encryption/integrity) and minimizes plaintext secret residency in memory. |
| **Malicious AI agent behavior** | Agent (or poisoned model logic) tries to exfiltrate env values over network/logs. | **Core defense**: process-level isolation + automatic stdout/stderr redaction. Agent cannot trivially read raw env sources; output is filtered. |
| **TEE boot config tampering** | Attacker tampers with `.appkeys.json` before TEE boot. | **Core defense**: remote attestation. `.appkeys.json` is issued by KMS only after TEE authenticity verification. Tampering changes measured state and causes key issuance refusal. |

---

## 6. BDD Scenarios

### 6.1 Feature: `jingui run` — Safe Secret Injection for AI Agent

`jingui run` must decrypt/inject secrets correctly and block common leakage paths.

#### Scenario 1: Successful Secret Usage

- **Given** a TEE instance has valid `.appkeys.json` from attestation flow.
- **And** server stores a credential field for reference `jingui://gmail-app/user@example.com/refresh_token`.
- **And** a Python script `agent.py` reads `API_KEY` and calls external API.
- **When** launcher runs:
  ```bash
  jingui run -- python agent.py
  ```
- **Then** `agent.py` can use `API_KEY=sk_live_123456789` to complete API call.
- **And** `jingui run` stdout does not reveal plaintext `sk_live_123456789`.

#### Scenario 2: Attempted stdout Leak

- **Given** same setup as Scenario 1.
- **And** `agent.py` includes malicious print logic:
  ```python
  import os
  api_key = os.getenv("API_KEY")
  print(f"Attempting to leak the key: {api_key}")
  ```
- **When** launcher runs:
  ```bash
  jingui run -- python agent.py
  ```
- **Then** final stdout is redacted:
  ```
  Attempting to leak the key: [REDACTED_BY_JINGUI]
  ```

#### Scenario 3: Attempted Process Introspection Leak

- **Given** same setup as Scenario 1.
- **And** `agent.py` tries to read `/proc/self/environ`:
  ```python
  with open("/proc/self/environ", "r") as f:
      print(f.read())
  ```
- **When** launcher runs:
  ```bash
  jingui run -- python agent.py
  ```
- **Then** process is terminated immediately.
- **And** `jingui run` reports a security violation, e.g. `forbidden system call (open /proc/self/environ)`.

### 6.2 Feature: `jingui inject` — Safe Template Rendering

#### Scenario 4: Successful Template Render

- **Given** same setup as Scenario 1.
- **And** template file `config.yaml.tpl`:
  ```yaml
  database:
    password: {{ jingui://db/prod/password }}
  ```
- **When** launcher runs:
  ```bash
  cat config.yaml.tpl | jingui inject > config.yaml
  ```
- **Then** generated `config.yaml` is:
  ```yaml
  database:
    password: <the_actual_db_password>
  ```

---

## 7. Deployment and Operations

### 7.1 Server Deployment

Jingui server is a standard Go backend service.

- **Recommended**: official Docker image with Kubernetes or Docker Compose orchestration.
- **Database**: production must use external PostgreSQL.
- **Configuration**: provide DB DSN, master key, and related settings via env vars or config file.

### 7.2 Client Deployment

Jingui client is not typically “installed by users”; it should be baked into the TEE base image.

- **Image build**: include `jingui` binary in golden image under PATH (e.g. `/usr/local/bin/jingui`).
- **Immutability**: once image is measured, client becomes part of attested state. Any modification changes measurement and should fail attestation/key issuance.
- **Entrypoint**: image startup script / container `ENTRYPOINT` should call `jingui run`, which then starts the real agent command.

### 7.3 Current Status and Correction Decisions (2026-02-24)

This section captures agreed baseline corrections.

**Confirmed issues**
- Current implementation incorrectly treats `app_id` as external service app identifier (e.g. gmail-app).
- Target semantics: `app_id` should represent CVM/Agent workload identity from TEE attestation chain.

**Completed design changes**
- Secret reference format: `jingui://<vault>/<item>/<field>` (with optional 4-segment `<section>` support).
- DB table renamed `user_secrets` → `vault_items`, column `user_id` → `item`.
- API route `/v1/user-secrets` → `/v1/secrets`, params `:app_id/:user_id` → `:vault/:item`.
- `app_id` remains as server-side authorization boundary (determined by instance binding and attestation).

---

## 8. Roadmap

### Phase 1 (complete): Data Model Refactor + CRUD

- **Goal**: correct data model semantics and rename `user_id` → `item`.
- **Scope**:
  - one-shot schema migration (no compatibility layer, no v2 API),
  - secret reference: `jingui://<vault>/<item>/<field>` (with optional 4-segment `<section>` support),
  - renamed DB table `user_secrets` → `vault_items`, route `/v1/user-secrets` → `/v1/secrets`,
  - aligned admin CRUD, fetch/read/run, and all tests.

### Phase 2: RA-TLS Identity Binding

- **Goal**: bind workload identity to attestation chain and replace pure FID trust.
- **Scope**:
  - RA-TLS certificate verification (dstack/go-ratls + dcap-qvl Go bindings),
  - parse workload identity (`app_id`) from attestation context for authorization,
  - keep secret-reference syntax unchanged to avoid client-config breakage.

### Phase 3: Production Hardening

- **Goal**: complete reliability/operations/security controls on top of stable identity model.
- **Scope**:
  - audit logs, PostgreSQL production rollout, HA deployment,
  - KMS + remote-attestation auto-registration pipeline,
  - complete policy documentation (rotation/revocation/compliance).

---

## 9. Open Questions (from original PD)

| # | Question | Impact | Suggested Answer |
| :--- | :--- | :--- | :--- |
| 1 | Can one client (FID) bind multiple users, or strictly one-to-one? | Data model, auth | **Recommend one-to-one**: one FID per TEE instance bound to one user authorization context. |
| 2 | Should client cache decrypted secrets? | Performance vs security | **No persistent cache**. Only in-memory cache per `jingui run` invocation. |
| 3 | Should OAuth providers beyond Google be supported? | OAuth gateway design | **Yes**. Define pluggable OAuth provider interface. |
| 4 | Should server periodically validate refresh-token validity? | Credential lifecycle | **Yes**. Add background validation and invalidation tagging. |
| 5 | Need secret versioning/history? | Audit/recovery | **Optional**. If enabled, keep limited recent versions and audit access. |
| 6 | How to support sharing/collaboration? | Permission model | **App-based permission model**. Access granted by app-level authorization. |
| 7 | Who issues JWTs, Jingui or external IdP? | Auth architecture | **Jingui-issued in MVP**. |
| 8 | Is MFA required? | Security posture | **Yes**, for human administrators. |

---

## 10. References

- Encrypted Environment Variables specification (`encrypted-env-spec.md`)
- 1Password CLI docs: https://developer.1password.com/docs/cli/
- Linux `ptrace`, `seccomp-bpf` man pages
- Aho-Corasick algorithm: https://en.wikipedia.org/wiki/Aho%E2%80%93Corasick_algorithm
- Intel TDX / AMD SEV-SNP whitepapers
