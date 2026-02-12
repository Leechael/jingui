# 金匮 (jingui): 产品需求与技术规格文档

---

## 0. 文档说明

**重要提示：** 本文档旨在成为一份完整的、可供开发团队直接使用的技术规格与产品需求文档。它融合了以下三个核心来源的信息：

1.  **凭证保险库产品设计 (PD)**：定义了核心的客户端/服务器架构、数据模型和 API (源自 `pasted_content_2.txt`)。
2.  **加密环境变量技术规范**：定义了底层的 ECIES 加密方案和 TEE 内的密钥管理 (源自 `encrypted-env-spec.md`)。
3.  **1Password CLI 调研**：借鉴了其优秀的开发者体验，特别是 `op run` 命令的设计哲学、秘密引用格式和输出掩蔽机制。

为了确保所有技术细节的完整性和准确性，本文档将**直接引用或完整包含**源文档中的关键技术规格，并在其基础上进行产品化的阐述和扩展。

---

## 1. 产品概述

### 1.1. 愿景与定位

**金匮 (jingui)** 是一个为在**可信执行环境 (TEE)** 中运行的 **AI Agent** 设计的零信任 (Zero-Trust) 秘密管理与注入引擎。它的核心使命是，在确保 AI Agent 能够正常使用所需凭证（如 API 密钥、数据库密码）的同时，从根本上杜绝 Agent 自身或其执行的子进程直接窥探、窃取或泄露这些凭证的可能性。

金匮并非一个供人类使用的传统秘密管理工具，而是一个嵌入在机密计算环境中的、自动化的、非交互式的安全组件。

### 1.2. 核心问题

随着 AI Agent 被赋予越来越强大的能力和越来越高的自主性，它们不可避免地需要访问各种敏感凭证来调用外部服务。这带来了前所未有的安全挑战：

1.  **Agent 可信度问题**：我们如何信任一个复杂的、可能是由大语言模型驱动的 Agent 不会恶意地或无意地泄露它所使用的凭证？
2.  **供应链攻击**：Agent 依赖的第三方库或其调用的外部工具可能存在漏洞，导致凭证被窃取。
3.  **TEE 环境中的秘密注入**：如何在 TEE 这种高度隔离的环境中，安全、高效地为应用程序提供其运行所需的动态凭证？
4.  **输出泄露**：Agent 在生成日志、报告或与用户交互时，可能会无意中将敏感的凭证信息打印到标准输出 (stdout)，造成泄露。

### 1.3. 目标用户与环境

- **核心用户**：**AI Agent** 的应用程序/进程。
- **运行环境**：**机密计算环境**，如 Intel TDX 或 AMD SEV-SNP 所构建的 TEE (Trusted Execution Environment)。
- **部署者**：AI Agent 平台或应用的开发者与运维团队。

---

## 2. 系统架构

本章节的系统架构设计，直接源自原始的《凭证保险库产品设计》文档，并针对 AI Agent 在 TEE 中运行的场景进行了适配。

### 2.1. 整体架构图

```
┌─────────────────────────────────────────────────────────┐
│                      金匮服务器 (Server)                      │
│                    (中心化凭证保险库与分发)                     │
└──────────────────────────┬──────────────────────────────┘
                           │  加密后的秘密 (ECIES Ciphertext)
                           ▼
┌───────────────────────────────────────────────────────────────────┐
│                      可信执行环境 (TEE)                         │
│                                                                   │
│  ┌────────────────────┐      ┌────────────────────────────────┐  │
│  │  金匮客户端 (Client) │      │         AI Agent 应用进程        │  │
│  │ (解密/注入/监视引擎)  │      │                                │  │
│  └──────────┬─────────┘      └───────────────┬────────────┘  │
│             │ 1. 解密秘密并注入环境变量       │ 2. 启动并监视       │
│             └───────────────┬───────────────┘                    │
│                             │                                    │
│                       ┌─────┴─────┐                              │
│                       │ stdout 管道 │ (自动掩蔽秘密)               │
│                       └─────┬─────┘                              │
│                             │                                    │
└─────────────────────────────┼───────────────────────────────────┘
                              ▼
                         安全的输出 (Safe Output)
```

### 2.2. 组件职责

#### 2.2.1. 金匮服务器 (Server)

作为中心化的凭证管理后台，其核心职责包括：
- **应用管理**：管理 App 及其凭证（如 OAuth `credentials.json`）。
- **OAuth 授权网关**：为人类用户提供授权接口，以获取和安全存储 `refresh_token`。
- **TEE 实例注册**：维护一个 TEE 实例身份 (FID) 到其公钥的注册表。
- **按需加密分发**：根据 TEE 客户端的请求，使用其注册的公钥，通过 ECIES 方案实时加密秘密并分发。

#### 2.2.2. 金匮客户端 (Client)

这是一个轻量级的、非交互式的 Go 二进制程序，作为 AI Agent 应用的**安全启动器 (Secure Launcher)**。

- **唯一入口**: `jingui run <agent_command> [args...]`
- **核心职责**:
    1.  **读取配置**: 从 TEE 环境中预置的配置文件（`.appkeys.json`）读取 TEE 实例的私钥。
    2.  **获取秘密**: 连接金匮服务器，拉取加密的秘密包。
    3.  **内存解密**: 在内存中使用实例私钥解密秘密。
    4.  **启动子进程**: 启动 `<agent_command>` 作为其子进程。
    5.  **注入与隔离**: 将解密后的秘密注入到子进程的环境变量中，并阻止子进程读取自身的环境变量内存空间。
    6.  **输出监视与掩蔽**: 拦截子进程的 `stdout` 和 `stderr`，实时过滤和替换输出内容中的秘密值。

---

## 3. 客户端 (Client) 设计：借鉴 1Password 的安全启动器

虽然金匮客户端是为非人类的 AI Agent 设计的，但其核心交互模型深受 **1Password CLI** 的启发，特别是其 `op run` 命令。我们旨在为 TEE 中的 Agent 提供与 `op run` 同样无缝、安全的体验。

### 3.1. 命令行接口 (CLI)

金匮客户端提供一个极其精简的 CLI，其设计直接对标 1Password CLI 的核心功能。

| 命令 | 对标 1Password 命令 | 作用 |
| :--- | :--- | :--- |
| `jingui run -- <cmd>` | `op run -- <cmd>` | **核心功能**。启动一个子进程，并将从服务器获取的秘密作为环境变量注入其中。同时，自动掩蔽子进程的 stdout/stderr 输出。 |
| `jingui read <secret_ref>` | `op read <secret_ref>` | 从服务器读取单个秘密的值并打印到标准输出。主要用于调试或需要将秘密值通过管道传递给其他工具的场景。 |
| `jingui inject` | `op inject` | 读取一个模板文件 (stdin)，将其中的秘密引用替换为真实的秘密值，然后将结果输出到 stdout。 |

### 3.2. 秘密引用格式 (Secret Reference Syntax)

为了在配置文件或代码中引用秘密，我们采用与 1Password 类似的 URI 格式。

**格式**: `jingui://<app_id>/<secret_name>/<field_name>`

- **`jingui://`**: 协议头，标识这是一个金匮秘密引用。
- **`<app_id>`**: 在服务器上注册的应用 ID。
- **`<secret_name>`**: 该应用下的秘密名称（例如，一个特定的用户授权）。
- **`<field_name>`**: 秘密对象中的具体字段（例如，`refresh_token` 或 `api_key`）。

**示例**:

```
# 引用 Google Drive 应用中，用户 a@b.com 的 refresh_token
GOOGLE_REFRESH_TOKEN="jingui://gdrive-app-123/user-a-b-com/refresh_token"
```

### 3.3. `jingui run` 的工作流程与安全机制

这是客户端的核心，它结合了秘密注入和安全隔离两大功能。

1.  **解析命令**: `jingui run` 解析其后的命令和参数。
2.  **扫描环境变量**: 扫描当前进程的环境变量，查找所有 `jingui://` 格式的秘密引用。
3.  **批量获取秘密**: 将所有找到的秘密引用聚合，向服务器发起一次批量获取请求。
4.  **解密与注入**: 在内存中解密所有秘密，并将它们设置到将要创建的子进程的环境变量中。
5.  **启动与隔离**: 
    *   `fork` 出子进程。
    *   在子进程 `execve` **之前**，通过 `ptrace` 和 `seccomp-bpf` 应用安全策略，阻止其访问 `/proc/self/environ` 等敏感资源。
    *   重定向子进程的 `stdout`/`stderr` 到管道。
6.  **输出掩蔽**: 父进程从管道读取子进程的输出，使用 Aho-Corasick 算法高效地将所有明文秘密值替换为 `[REDACTED_BY_JINGUI]`。

### 3.4. 配置文件 (`.appkeys.json`)

客户端的唯一配置来源于一个在 TEE 启动时被安全注入的 JSON 文件。**此文件的格式和来源严格遵循《加密环境变量技术规范》**。

- **位置**: TEE 内部的预定路径，如 `/dstack/.host-shared/.appkeys.json`。
- **核心字段**: `env_crypt_key`，即 TEE 实例的 X25519 私钥（32字节，hex 编码）。
- **来源**: 由 TEE 的启动器 (Launcher) 或编排系统在创建 TEE 实例时，通过与 KMS 的远程证明过程动态生成并注入。

---

## 4. 服务器端 (Server) 设计

金匮服务器是整个系统的中央凭证授权和分发中心。**本章节的设计完全基于原始的《凭证保险库产品设计》文档**，确保所有原始设计意图和技术细节都被完整保留。

### 4.1. 核心职责 (源自原始 PD)

- **应用管理 (App Management)**：负责第三方应用 (App) 的注册和凭证管理。每个 App 都有一个唯一的 `app_id`，并存储其 `credentials.json`（加密存储）。
- **OAuth 授权网关 (OAuth Gateway)**：为人类用户（如开发者、管理员）提供一个统一的 OAuth 授权流程，以安全地获取和存储用于访问第三方服务的 `refresh_token`。
- **TEE 实例注册 (Instance Registration)**：维护一个 TEE 实例身份 (FID) 到其公钥的注册表。此注册过程必须是安全的，通常与 KMS 或远程证明流程集成。
- **按需加密分发 (On-Demand Encrypted Dispatch)**：服务器的核心安全功能。它从不存储明文秘密。当收到来自已验证的 TEE 客户端的请求时，它会：
    1.  在内部解密存储的 `refresh_token`（使用服务器自身的主密钥）。
    2.  使用请求方 TEE 实例的公钥，通过 ECIES 方案**实时加密**该 `refresh_token`。
    3.  将加密后的数据包返回给客户端。
- **静态数据加密 (Encryption at Rest)**：所有持久化存储在数据库中的敏感信息（如 App 的 `credentials.json`、用户的 `refresh_token`）都必须使用服务器自身的主密钥进行加密。

### 4.2. 数据模型 (Database Schema)

**以下数据模型直接采纳自原始 PD 设计**，用于支持上述核心职责。表名和字段名应严格按照此规范实现。

#### 4.2.1. `apps` (应用注册表)

存储所有注册的第三方应用信息。

```sql
-- 应用注册表，存储所有注册的第三方应用信息
CREATE TABLE apps (
    app_id VARCHAR(255) PRIMARY KEY,       -- 应用唯一标识 (e.g., 'gdrive-app-123')
    name VARCHAR(255) NOT NULL,            -- 应用可读名称 (e.g., 'My Company Google Drive')
    service_type VARCHAR(50) NOT NULL,     -- 服务类型: 'google', 'github', 'aws', etc.
    credentials_encrypted BYTEA NOT NULL,  -- 使用服务器主密钥加密后的 credentials.json
    required_scopes TEXT[] NOT NULL,       -- 应用请求的 OAuth 权限范围
    created_by VARCHAR(255) NOT NULL,      -- 创建该应用的管理员 user_id
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

#### 4.2.2. `user_secrets` (用户授权凭证)

存储人类用户对某个 App 的授权凭证，核心是加密的 `refresh_token`。

```sql
-- 用户授权凭证表，存储用户对某个 App 的授权凭证
CREATE TABLE user_secrets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id VARCHAR(255) NOT NULL,         -- 授权用户的 ID
    app_id VARCHAR(255) NOT NULL REFERENCES apps(app_id) ON DELETE CASCADE,
    secret_encrypted BYTEA NOT NULL,         -- 使用服务器主密钥加密后的 Secret 对象 (包含 refresh_token 等)
    granted_scopes TEXT[] NOT NULL,          -- 用户实际授予的权限范围
    token_is_valid BOOLEAN NOT NULL DEFAULT TRUE, -- 该 refresh_token 是否仍然有效
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, app_id)
);
```

#### 4.2.3. `tee_instances` (TEE 实例注册表)

存储所有已注册的、被授权可以获取秘密的 TEE 实例及其公钥。

```sql
-- TEE 实例注册表，系统的核心信任锚点之一
CREATE TABLE tee_instances (
    fid VARCHAR(40) PRIMARY KEY,             -- TEE 实例的指纹 ID (SHA1 hash of public key)
    public_key BYTEA NOT NULL UNIQUE,        -- TEE 实例的 X25519 公钥 (32 bytes)
    bound_app_id VARCHAR(255) NOT NULL,      -- 绑定的应用 ID，决定了此 TEE 能获取哪个 App 的凭证
    bound_user_id VARCHAR(255) NOT NULL,     -- 绑定的用户 ID，决定了使用哪个用户的授权凭证
    label VARCHAR(255),                      -- 可读标签 (e.g., 'prod-agent-vm-1')
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_used_at TIMESTAMPTZ,
    FOREIGN KEY (bound_app_id, bound_user_id) REFERENCES user_secrets(app_id, user_id)
);
```

### 4.3. API 规范 (gRPC & RESTful)

服务器应同时提供 gRPC 和 RESTful API 以满足不同场景的需求。**以下 API 设计直接采纳自原始 PD 设计**。

| Service (gRPC) | Method (RPC) | Path (REST) | 调用者 | 说明 |
| :--- | :--- | :--- | :--- | :--- |
| `AppService` | `RegisterApp` | `POST /v1/apps` | 人类用户 (管理员) | 注册新的第三方 App，上传 `credentials.json`。 |
| `CredentialService` | `GetAuthGateway` | `GET /v1/credentials/gateway/{app_id}` | 人类用户 (开发者) | 获取 OAuth 授权跳转地址，以完成对 App 的授权。 |
| `SecretService` | `FetchSecrets` | `POST /v1/secrets/fetch` | **TEE 客户端** | **核心接口**。TEE 实例通过 FID 和其他证明材料，批量拉取其所需的所有加密后的凭证包。 |
| `InstanceService` | `RegisterInstance` | `POST /v1/instances` | KMS / 运维脚本 | 注册一个新的 TEE 实例及其公钥，并将其绑定到特定的 App 和用户授权。 |

#### 核心 API 流程详解: `FetchSecrets`

1.  **请求 (Request)**: TEE 客户端发起 `POST /v1/secrets/fetch` 请求。请求体包含：
    ```json
    {
      "fid": "...", // TEE 实例的指纹 ID
      "secret_references": ["jingui://...", "jingui://..."] // 需要获取的秘密引用列表
    }
    ```
2.  **验证实例 (Instance Verification)**: 服务器通过 `fid` 在 `tee_instances` 表中查找记录。如果找不到，返回 404 Not Found。
3.  **解析引用 (Reference Parsing)**: 服务器遍历 `secret_references` 列表，解析出每个引用对应的 `app_id`, `user_id` 等信息。
4.  **权限检查 (Authorization)**: 验证该 `fid` 是否有权限访问其请求的每一个 `app_id` 和 `user_id`。这通过 `tee_instances` 表中的 `bound_app_id` 和 `bound_user_id` 来实现。
5.  **批量获取与加密 (Batch Fetch & Encrypt)**:
    a.  对于每一个合法的秘密引用，服务器从 `user_secrets` 表中获取对应的 `secret_encrypted`。
    b.  用服务器主密钥在内存中解密该秘密，得到明文 `refresh_token`。
    c.  使用 `fid` 对应的 `public_key`，通过 **ECIES 方案**将明文 `refresh_token` 加密。
6.  **返回加密包 (Response)**: 将所有加密后的秘密组织成一个 map 返回给客户端。
    ```json
    {
      "secrets": {
        "jingui://gdrive/user1/token": "<ECIES_encrypted_blob_1>",
        "jingui://github/user1/token": "<ECIES_encrypted_blob_2>"
      }
    }
    ```
_The following is a direct and complete inclusion of the encryption specification to ensure all technical details are preserved for developers._

## 5. 加密与安全模型

### 5.1. 加密协议技术规格 (源自 `encrypted-env-spec.md`)

**为了确保实现的绝对准确性，本节完整地、不加修改地包含了《加密环境变量技术规范》的全部内容。**

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

### 5.2. 安全模型与威胁分析

在上述加密协议的基础上，金匮的安全模型依赖于以下核心假设和对策：

| 威胁场景 | 攻击描述 | 金匮的对策 |
| :--- | :--- | :--- |
| **网络窃听** | 攻击者在 TEE 客户端和服务器之间嗅探网络流量。 | 全链路强制 HTTPS/TLS。更重要的是，获取到的秘密数据包本身也是用 TEE 实例的公钥加密的，攻击者没有对应的私钥无法解密。 |
| **服务器被完全攻破** | 攻击者获得了服务器的 root 权限和数据库的完整访问权。 | **核心防御**：服务器不存储 TEE 实例的私钥。攻击者无法伪造 TEE 实例的身份，也无法解密已经分发出去的秘密包。 |
| **TEE 内存泄露** | 攻击者利用 CPU 漏洞（如 Spectre/Meltdown）或 TEE 本身的漏洞，试图读取 TEE 内部内存。 | 这是 TEE 技术的根本挑战。金匮依赖于底层 TEE 实现（如 Intel TDX）提供的内存加密和完整性保护来缓解此风险。金匮自身能做的是确保明文秘密在内存中的停留时间尽可能短。 |
| **AI Agent 恶意行为** | Agent 自身（如一个被污染的 LLM）试图在代码逻辑中将获取到的环境变量值通过网络外传。 | **核心防御**：金匮的**进程级秘密隔离**和**自动化输出掩蔽**机制是为此设计的。Agent 无法直接读取环境变量的明文，其所有 stdout/stderr 输出都会被过滤。 |
| **TEE 启动配置被篡改** | 攻击者在 TEE 启动前，篡改了将被注入的 `.appkeys.json` 文件。 | **核心防御**：依赖于**远程证明**。`.appkeys.json` 的内容是由 KMS 在验证了 TEE 的真实性后颁发的。任何对 TEE 启动镜像或配置的篡改都会改变证明报告的度量值，导致 KMS 拒绝颁发密钥。 |

---

## 6. BDD (行为驱动开发) 场景描述

本章节以 BDD 的形式，描述金匮客户端在 TEE 环境中的核心行为，确保其安全机制和功能符合设计预期。

### 6.1. 功能: `jingui run` - 安全地为 AI Agent 注入并使用秘密

**为了让 AI Agent 能够在不泄露秘密的前提下使用凭证，`jingui run` 必须能够正确地解密、注入秘密，并阻止任何形式的泄露。**

#### 场景 1: 成功的秘密使用

- **假设 (Given)** 一个 TEE 实例已经通过远程证明获得了包含有效私钥的 `.appkeys.json` 文件。
- **并且 (And)** 金匮服务器上存储着一个 API 密钥，其值为 `sk_live_123456789`，对应的秘密引用为 `jingui://app/secret/key`。
- **并且 (And)** 一个 Python 脚本 `agent.py` 被设计为从环境变量 `API_KEY` 中读取密钥并发起 API 调用。
- **当 (When)** TEE 启动器在设置了 `API_KEY="jingui://app/secret/key"` 的环境下执行命令：
  ```bash
  jingui run -- python agent.py
  ```
- **那么 (Then)** `agent.py` 脚本应该能够成功读取到 `API_KEY` 的值 `sk_live_123456789` 并完成 API 调用。
- **并且 (And)** `jingui run` 的标准输出中不应包含任何 `sk_live_123456789` 的明文字符串。

#### 场景 2: 尝试通过标准输出泄露秘密

- **假设 (Given)** 与“成功的秘密使用”场景相同的设置。
- **并且 (And)** `agent.py` 脚本中包含恶意代码，试图打印环境变量：
  ```python
  import os
  api_key = os.getenv("API_KEY")
  print(f"Attempting to leak the key: {api_key}")
  ```
- **当 (When)** TEE 启动器执行命令：
  ```bash
  jingui run -- python agent.py
  ```
- **那么 (Then)** `jingui run` 的最终标准输出应该是被掩蔽后的内容：
  ```
  Attempting to leak the key: [REDACTED_BY_JINGUI]
  ```

#### 场景 3: 尝试通过进程内省泄露秘密

- **假设 (Given)** 与“成功的秘密使用”场景相同的设置。
- **并且 (And)** `agent.py` 脚本中包含更高级的恶意代码，试图读取 `/proc/self/environ` 文件：
  ```python
  with open("/proc/self/environ", "r") as f:
      print(f.read())
  ```
- **当 (When)** TEE 启动器执行命令：
  ```bash
  jingui run -- python agent.py
  ```
- **那么 (Then)** `agent.py` 进程应该被立即终止。
- **并且 (And)** `jingui run` 应该报告一个安全违规错误，例如 `Error: Agent process attempted a forbidden system call (open /proc/self/environ)`。

### 6.2. 功能: `jingui inject` - 安全地渲染模板

**为了支持基于模板的配置文件生成，`jingui inject` 必须能够安全地将秘密引用替换为真实值。**

#### 场景 4: 成功渲染模板

- **假设 (Given)** 与“成功的秘密使用”场景相同的设置。
- **并且 (And)** 一个名为 `config.yaml.tpl` 的模板文件内容如下：
  ```yaml
  database:
    password: {{ jingui://db/prod/password }}
  ```
- **当 (When)** TEE 启动器执行命令：
  ```bash
  cat config.yaml.tpl | jingui inject > config.yaml
  ```
- **那么 (Then)** 生成的 `config.yaml` 文件内容应该是：
  ```yaml
  database:
    password: <the_actual_db_password>
  ```

---

## 7. 部署与运维

### 7.1. 服务器部署

金匮服务器作为一个标准的 Go 后端应用，可以被灵活地部署在企业内部的基础设施上。

- **推荐方式**: 使用官方提供的 Docker 镜像，并通过 Kubernetes 或 Docker Compose 进行编排。
- **数据库**: 生产环境必须使用外部的 PostgreSQL 数据库。
- **配置**: 通过环境变量或配置文件来提供数据库连接字符串、服务器主密钥等。

### 7.2. 客户端部署

金匮客户端不是一个由用户“安装”的软件，而是被**构建在 TEE 基础镜像**中的一个核心组件。

- **镜像构建**: 在构建 TEE 的“黄金镜像”时，`jingui` 的二进制文件必须被包含在内，并放置在系统的 `PATH` 路径下（如 `/usr/local/bin/jingui`）。
- **不可变性**: 一旦镜像被构建和度量，`jingui` 客户端就成为该 TEE 环境不可变的一部分。任何对其的修改都会改变 TEE 的证明报告，导致远程证明失败。
- **入口点**: TEE 镜像的启动脚本或容器的 `ENTRYPOINT` 应被配置为 `jingui run`，由它来启动真正的 AI Agent 应用。

---

## 8. 实现路线图 (Roadmap)

### 第一阶段 (MVP): 核心功能闭环

- **目标**: 验证核心的秘密注入和输出掩蔽流程。
- **内容**:
    - **客户端**: `jingui run` 的基本实现，包括解析秘密引用、从服务器获取加密秘密、使用 Go `crypto` 库解密、通过 `os/exec` 启动子进程并注入环境变量、通过 `io.Pipe` 实现基本的 `stdout`/`stderr` 重定向和过滤。
    - **服务器**: 简化的 API 实现，支持 TEE 实例注册和秘密分发，使用内存或 SQLite 存储。

### 第二阶段 (Alpha): 安全机制与核心功能强化

- **目标**: 实现关键的进程隔离和 `inject` 功能。
- **内容**:
    - **客户端**: 
        - 引入 `seccomp-bpf` 或 `ptrace` 机制，实现对子进程的系统调用过滤。
        - 使用 Aho-Corasick 算法重构输出掩蔽逻辑。
        - 实现 `jingui read` 和 `jingui inject` 命令。
    - **服务器**: 
        - 完整的 OAuth 2.0 授权网关。
        - 引入 PostgreSQL 支持。
        - 为人类管理员提供一个简单的 Web UI 来管理应用和凭证。

### 第三阶段 (Beta): 生产级可用性

- **目标**: 使系统达到生产环境部署的标准。
- **内容**:
    - **服务器**: 
        - 完整的审计日志功能。
        - 与 KMS 集成，实现全自动的远程证明和 TEE 实例注册。
        - 高可用部署方案（多副本、负载均衡）。
    - **文档**: 提供完整的部署、运维和 API 文档。

---

## 9. 开放问题 (源自原始 PD)

**以下开放问题直接采纳自原始 PD 设计，需要在详细设计阶段进行决策。**

| # | 问题 | 影响范围 | 建议答案 |
| :--- | :--- | :--- | :--- |
| 1 | 一个客户端 (FID) 是否可以绑定多个用户，还是严格一对一？ | 数据模型、权限控制 | **建议一对一**。一个 FID 对应一个 TEE 实例，一个实例绑定到一个特定的用户授权。这简化了权限模型。 |
| 2 | 客户端是否需要缓存解密后的秘密？ | 客户端性能与安全 | **不建议持久化缓存**。仅支持在 `jingui run` 单次运行期间的内存缓存。 |
| 3 | 是否需要支持 Google 以外的 OAuth 服务？ | OAuth 网关设计 | **建议支持**。设计一个可插拔的 OAuth Provider 接口。 |
| 4 | Server 端是否需要定期验证 `refresh_token` 的有效性？ | 凭证管理策略 | **建议支持**。实现一个后台任务，定期验证并标记失效的凭证。 |
| 5 | 是否需要支持秘密的版本控制和历史记录？ | 审计与恢复 | **可选**。如果实现，应只保留最近的几个版本，并且历史版本的访问也应被审计。 |
| 6 | 如何处理秘密的分享和协作？ | 权限模型 | **建议基于 App 的权限**。用户对某个 App 有权限，就能访问该 App 下的所有秘密。 |
| 7 | JWT 由谁签发？是金匮服务器自身，还是外部 IdP？ | 认证架构 | **建议金匮服务器自身签发**（MVP 阶段）。 |
| 8 | 是否需要支持 MFA (多因素认证)？ | 安全强度 | **建议支持**（针对人类管理员）。 |

---

## 10. 参考资源

- **加密环境变量技术规范** (源自 `encrypted-env-spec.md`)
- **1Password CLI Documentation**: (https://developer.1password.com/docs/cli/)
- **Linux ptrace, seccomp-bpf Man Pages**
- **Aho-Corasick 算法**: (https://en.wikipedia.org/wiki/Aho%E2%80%93Corasick_algorithm)
- **Intel TDX / AMD SEV-SNP Whitepapers**
