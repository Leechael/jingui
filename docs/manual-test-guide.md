# Jingui Manual Test Guide

本指引将测试拆分为 **Server 端 (operator 机器)** 和 **Client 端 (TDX 环境)** 两部分。
涉及的所有值（密钥、FID、公钥等）在执行过程中会动态生成，请随时记录并在后续步骤中替换。

> 如果你要先做一轮本地快速全链路回归，优先运行：`scripts/manual-test.sh`。
> 该脚本已覆盖 app / instance / user-secret 的新增 admin CRUD 检查与级联删除场景。
>
> ⚠️ 设计已确认将重构为 `jingui://<service>/<slug>/<field>` 语义，`app_id` 不再出现在 ref 中。
> 本文部分示例仍使用旧格式，待 schema/handler 重构完成后会统一替换。

---

## 0. Build

在开发机上：

```bash
# 确认版本
make clean build
bin/jingui --version        # → jingui dev (commit=..., go=..., darwin/arm64)
bin/jingui-server -v        # → jingui-server dev (commit=..., go=..., darwin/arm64)

# 交叉编译 TDX 用的 linux/amd64 client
make build-client-linux-amd64
file bin/linux-amd64/jingui  # → ELF 64-bit LSB executable, x86-64 ...

# 交叉编译 linux/amd64 server（如果 server 也跑在 linux 上）
make build-server-linux-amd64
```

产物路径：

| 目标 | 路径 |
|------|------|
| 当前平台 client | `bin/jingui` |
| 当前平台 server | `bin/jingui-server` |
| linux/amd64 client | `bin/linux-amd64/jingui` |
| linux/amd64 server | `bin/linux-amd64/jingui-server` |

---

## Part A — Server 端操作 (Operator)

### A1. 生成 Master Key 和 Admin Token

```bash
export JINGUI_MASTER_KEY=$(openssl rand -hex 32)
echo "Master Key: $JINGUI_MASTER_KEY"   # 记录下来，后续启动 server 需要

export JINGUI_ADMIN_TOKEN=$(openssl rand -hex 16)
echo "Admin Token: $JINGUI_ADMIN_TOKEN"  # 记录下来，管理 API 需要
```

### A2. 启动 Server

```bash
export JINGUI_MASTER_KEY="<上一步的值>"
export JINGUI_ADMIN_TOKEN="<上一步的值>"
export JINGUI_DB_PATH="./jingui-test.db"
export JINGUI_LISTEN_ADDR=":8080"
export JINGUI_BASE_URL="http://<SERVER_IP>:8080"   # TDX 能访问到的地址

bin/jingui-server
# 输出: jingui-server listening on :8080
```

> 如果 server 也在 linux 上跑，用 `bin/linux-amd64/jingui-server`。
> 注意：如果 `JINGUI_BASE_URL` 不是 HTTPS，server 会输出警告。测试环境可忽略。

### A3. 注册 App (上传 Google OAuth credentials.json)

> 若返回 `app_id already exists`，请改用 `PUT /v1/apps/:app_id` 更新，而不是重复 `POST /v1/apps`。

准备好你的 Google Cloud Console 下载的 `credentials.json`，内容形如：

```json
{"installed":{"client_id":"xxx.apps.googleusercontent.com","client_secret":"GOCSPX-xxx","redirect_uris":["http://localhost"]}}
```

注册：

```bash
SERVER="http://<SERVER_IP>:8080"
ADMIN_TOKEN="<A1 中生成的 Admin Token>"

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

**预期响应：**

```json
{"app_id":"gmail-app","status":"created"}
```

**验证点 ✓**: HTTP 201, status = "created"

如需更新已有 app：

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

### A4. OAuth 授权 (获取 refresh_token)

OAuth gateway 需要 admin token。在浏览器中无法直接传 header，用 curl 获取重定向 URL：

```bash
curl -s -v "$SERVER/v1/credentials/gateway/gmail-app" \
  -H "Authorization: Bearer $ADMIN_TOKEN" 2>&1 | grep -i location
```

复制 `Location:` 后面的 Google OAuth URL 到浏览器中打开。

或者，如果 server 在本地，可以直接用浏览器访问（但需要另一种方式传 token）。简便做法是临时用 curl 跟随重定向：

```bash
# 这会输出 Google 登录 URL，复制到浏览器
curl -s -D - "$SERVER/v1/credentials/gateway/gmail-app" \
  -H "Authorization: Bearer $ADMIN_TOKEN" | head -20
```

流程：
1. 浏览器重定向到 Google 登录页
2. 选择 Google 账号并授权
3. 回调到 `/v1/credentials/callback`
4. 页面显示 JSON 结果

**预期响应：**

```json
{"status":"authorized","app_id":"gmail-app","email":"user@example.com"}
```

**验证点 ✓**: status = "authorized"，email 是你授权的 Google 账号

> **记录 email**: 后续注册 TEE instance 时 `bound_user_id` 必须与此一致。

### A5. (暂停) 等待 Client 端生成密钥

继续 Part B 在 TDX 里生成密钥，拿到 **public_key** 后再回来完成 A6。

---

## Part B — Client 端操作 (TDX 环境)

### B1. 拷贝文件到 TDX

需要传入 TDX 的文件：

```
bin/linux-amd64/jingui     # client 二进制
```

```bash
# 示例：scp 到 TDX 实例
scp bin/linux-amd64/jingui  tdx-host:/opt/jingui/jingui

# 在 TDX 里给执行权限
ssh tdx-host 'chmod +x /opt/jingui/jingui'
```

### B2. 验证二进制

```bash
# 在 TDX 内
/opt/jingui/jingui --version
# → jingui dev (commit=..., go=..., linux/amd64)

/opt/jingui/jingui --help
```

**验证点 ✓**: 版本输出 `linux/amd64`

### B3. 生成密钥 (.appkeys.json)

```bash
cd /opt/jingui
./jingui init -o .appkeys.json
```

输出示例：

```
Wrote .appkeys.json

Public Key : 7a8b3c...  (64 hex chars)
FID        : 2f4e9d...  (40 hex chars)

Use the public key to register this instance:
  curl -X POST $SERVER/v1/instances \
    -H 'Content-Type: application/json' \
    -d '{"public_key":"7a8b3c...","bound_app_id":"<APP_ID>","bound_user_id":"<EMAIL>"}'
```

**记录下来：**
- `Public Key`: **____________________________**
- `FID`: **____________________________**

> `.appkeys.json` 包含私钥，权限已设为 0600，不要传出 TDX。

### B4. (暂停) 回到 Server 端完成注册

把上面的 **Public Key** 给到 operator，继续 Part A 的 A6。

---

## Part A (续) — 注册 TEE Instance

### A6. 注册 TEE Instance

用 TDX 里生成的 public_key 和 A4 中 OAuth 授权的 email：

```bash
SERVER="http://<SERVER_IP>:8080"
ADMIN_TOKEN="<A1 中生成的 Admin Token>"
PUBLIC_KEY="<B3 中获得的 Public Key>"
EMAIL="<A4 中授权的 email>"

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

**预期响应：**

```json
{"fid":"2f4e9d...","status":"registered"}
```

**验证点 ✓**:
- HTTP 201
- 返回的 `fid` 应与 B3 中 `jingui init` 输出的 FID 一致

### A7. 直接 curl 测试 secrets/fetch (可选)

```bash
FID="<B3 的 FID>"

curl -s -X POST "$SERVER/v1/secrets/fetch" \
  -H 'Content-Type: application/json' \
  -d "$(jq -n \
    --arg fid "$FID" \
    '{fid:$fid, secret_references:["jingui://gmail-app/'"$EMAIL"'/client_id","jingui://gmail-app/'"$EMAIL"'/client_secret","jingui://gmail-app/'"$EMAIL"'/refresh_token"]}'
  )" | jq .
```

**预期响应：**

```json
{
  "secrets": {
    "jingui://gmail-app/user@example.com/client_id": "<base64 ECIES blob>",
    "jingui://gmail-app/user@example.com/client_secret": "<base64 ECIES blob>",
    "jingui://gmail-app/user@example.com/refresh_token": "<base64 ECIES blob>"
  }
}
```

**验证点 ✓**: 3 个 key 全部返回，值为 base64 编码的 ECIES 密文

---

## Part B (续) — Client 端完整测试

### B5. 准备 .env 文件

```bash
cat > /opt/jingui/.env << 'EOF'
GOG_ACCOUNT=user@example.com
GOG_CLIENT_ID=jingui://gmail-app/user@example.com/client_id
GOG_CLIENT_SECRET=jingui://gmail-app/user@example.com/client_secret
GOG_REFRESH_TOKEN=jingui://gmail-app/user@example.com/refresh_token
EOF
```

> 把 `user@example.com` 替换为 A4 中实际授权的 email。

### B6. Test 1 — `jingui read` 读取单个 secret

```bash
cd /opt/jingui
./jingui read \
  --server "http://<SERVER_IP>:8080" \
  --appkeys .appkeys.json \
  --insecure \
  "jingui://gmail-app/user@example.com/client_id"
```

**预期**: 默认仅 stdout 输出真实的 client_id 值（如 `xxx.apps.googleusercontent.com`）。

如需显示调试元信息（FID/Public Key）：

```bash
./jingui read \
  --show-meta \
  --server "http://<SERVER_IP>:8080" \
  --appkeys .appkeys.json \
  --insecure \
  "jingui://gmail-app/user@example.com/client_id"
```

**验证点 ✓**: 输出与 credentials.json 中的 `client_id` 一致

### B7. Test 2 — stdout 掩蔽

```bash
./jingui run \
  --server "http://<SERVER_IP>:8080" \
  --appkeys .appkeys.json \
  --insecure \
  --env-file .env \
  -- /bin/sh -c 'echo $GOG_CLIENT_ID'
```

**预期输出：**

```
[REDACTED_BY_JINGUI]
```

**验证点 ✓**: 真实的 client_id 值不出现在 stdout

### B8. Test 3 — stderr 掩蔽

```bash
./jingui run \
  --server "http://<SERVER_IP>:8080" \
  --appkeys .appkeys.json \
  --insecure \
  --env-file .env \
  -- /bin/sh -c 'echo $GOG_CLIENT_SECRET >&2'
```

**预期 stderr 输出：**

```
[REDACTED_BY_JINGUI]
```

**验证点 ✓**: 真实的 client_secret 值不出现在 stderr

### B9. Test 4 — 多个 secret 同时掩蔽

```bash
./jingui run \
  --server "http://<SERVER_IP>:8080" \
  --appkeys .appkeys.json \
  --insecure \
  --env-file .env \
  -- /bin/sh -c 'echo "id=$GOG_CLIENT_ID secret=$GOG_CLIENT_SECRET token=$GOG_REFRESH_TOKEN"'
```

**预期输出：**

```
id=[REDACTED_BY_JINGUI] secret=[REDACTED_BY_JINGUI] token=[REDACTED_BY_JINGUI]
```

**验证点 ✓**: 三个值全部被掩蔽

### B10. Test 5 — 普通环境变量不受影响

```bash
./jingui run \
  --server "http://<SERVER_IP>:8080" \
  --appkeys .appkeys.json \
  --insecure \
  --env-file .env \
  -- /bin/sh -c 'echo "account=$GOG_ACCOUNT"'
```

**预期输出：**

```
account=user@example.com
```

**验证点 ✓**: 非 jingui:// 的普通值正常传递，不被掩蔽

### B11. Test 6 — 子进程退出码传递

```bash
./jingui run \
  --server "http://<SERVER_IP>:8080" \
  --appkeys .appkeys.json \
  --insecure \
  --env-file .env \
  -- /bin/sh -c 'exit 42'
echo "exit code: $?"
```

**预期输出：**

```
exit code: 42
```

**验证点 ✓**: 子进程退出码正确传递

### B12. Test 7 — 与 gogcli 集成 (最终验证)

```bash
./jingui run \
  --server "http://<SERVER_IP>:8080" \
  --appkeys .appkeys.json \
  --insecure \
  --env-file .env \
  -- gogcli gmail messages list
```

**验证点 ✓**:
- gogcli 正常工作（列出 Gmail 消息）
- stdout/stderr 中不出现任何真实的 client_id / client_secret / refresh_token

---

## 认证与访问控制测试 (Server 端)

### C0. Admin Token 认证

```bash
# 无 token → 401
curl -s -o /dev/null -w "%{http_code}" -X POST "$SERVER/v1/apps" \
  -H 'Content-Type: application/json' \
  -d '{"app_id":"x","name":"x","service_type":"x","credentials_json":{"installed":{"client_id":"a","client_secret":"b"}}}'
# 预期: 401

# 错误 token → 401
curl -s -o /dev/null -w "%{http_code}" -X POST "$SERVER/v1/apps" \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer wrong-token' \
  -d '{"app_id":"x","name":"x","service_type":"x","credentials_json":{"installed":{"client_id":"a","client_secret":"b"}}}'
# 预期: 401

# secrets/fetch 不需要 admin token（给 TEE 调用）
curl -s -o /dev/null -w "%{http_code}" -X POST "$SERVER/v1/secrets/fetch" \
  -H 'Content-Type: application/json' \
  -d '{"fid":"nonexistent","secret_references":[]}'
# 预期: 非 401 (应为 404)
```

**验证点 ✓**: 管理 API (apps, instances, gateway) 需要正确的 Bearer token；secrets/fetch 不需要

### C1. 错误的 app_id

```bash
curl -s -X POST "$SERVER/v1/secrets/fetch" \
  -H 'Content-Type: application/json' \
  -d '{"fid":"'"$FID"'","secret_references":["jingui://wrong-app/user@example.com/client_id"]}'
```

**预期**: HTTP 403 `{"error":"app_id mismatch ..."}`

### C2. 错误的 user_id

```bash
curl -s -X POST "$SERVER/v1/secrets/fetch" \
  -H 'Content-Type: application/json' \
  -d '{"fid":"'"$FID"'","secret_references":["jingui://gmail-app/wrong@example.com/client_id"]}'
```

**预期**: HTTP 403 `{"error":"user_id mismatch ..."}`

### C3. 不存在的 FID

```bash
curl -s -X POST "$SERVER/v1/secrets/fetch" \
  -H 'Content-Type: application/json' \
  -d '{"fid":"0000000000000000000000000000000000000000","secret_references":["jingui://gmail-app/user@example.com/client_id"]}'
```

**预期**: HTTP 404 `{"error":"instance not found"}`

---

## Admin CRUD 补充检查（新增）

### D1. 查询接口（apps / instances / user-secrets）

```bash
curl -s "$SERVER/v1/apps" -H "Authorization: Bearer $ADMIN_TOKEN" | jq .
curl -s "$SERVER/v1/instances" -H "Authorization: Bearer $ADMIN_TOKEN" | jq .
curl -s "$SERVER/v1/user-secrets" -H "Authorization: Bearer $ADMIN_TOKEN" | jq .
```

**验证点 ✓**:
- 都返回 200
- `apps` 不泄露 `credentials_encrypted`
- `user-secrets` 列表不泄露 `secret_encrypted`

### D2. 非级联删除阻断

```bash
# 若 app 下仍有 user_secrets/instances，删除应失败
curl -s -o /dev/null -w "%{http_code}" -X DELETE \
  "$SERVER/v1/apps/gmail-app" \
  -H "Authorization: Bearer $ADMIN_TOKEN"
```

**预期**: 409（有依赖记录）

### D3. 级联删除

```bash
# 允许级联删除 app 及其依赖数据
curl -s -X DELETE \
  "$SERVER/v1/apps/gmail-app?cascade=true" \
  -H "Authorization: Bearer $ADMIN_TOKEN"
```

**预期**: 200，随后查询 apps / user-secrets / instances 对应记录已移除

---

## Checklist

| # | 测试项 | 预期 | 通过 |
|---|--------|------|------|
| A1 | 生成 Master Key + Admin Token | 两个值都记录 | ☐ |
| A3 | 注册 App (带 admin token) | 201 created | ☐ |
| A4 | OAuth 授权 | authorized + email | ☐ |
| A6 | 注册 TEE Instance | 201 registered, FID 匹配 | ☐ |
| A7 | curl fetch secrets | 3 个 base64 blob 返回 | ☐ |
| B2 | 二进制版本 | linux/amd64 | ☐ |
| B3 | jingui init | 生成 .appkeys.json + 输出 pubkey/FID | ☐ |
| B6 | jingui read | 输出真实 client_id | ☐ |
| B7 | stdout 掩蔽 | [REDACTED_BY_JINGUI] | ☐ |
| B8 | stderr 掩蔽 | [REDACTED_BY_JINGUI] | ☐ |
| B9 | 多 secret 掩蔽 | 三个全部掩蔽 | ☐ |
| B10 | 普通 env 透传 | account=user@example.com | ☐ |
| B11 | 退出码传递 | exit code: 42 | ☐ |
| B12 | gogcli 集成 | 正常工作 + 输出无泄漏 | ☐ |
| C0 | Admin Token 认证 | 无/错 token → 401, secrets/fetch 不需 token | ☐ |
| C1 | 错误 app_id | 403 | ☐ |
| C2 | 错误 user_id | 403 | ☐ |
| C3 | 不存在 FID | 404 | ☐ |
