[English](./README.md) | 繁體中文
# `examples/` 堆疊

Magic Conch Shell 的完整整合測試部署：TLS gateway → router →
認證（通用 OIDC/OAuth2 + JWT）→ 聊天/上傳服務 → LLM core。

```
Client (integration_test.go)
  │  HTTPS :443
  ▼
gateway (Caddy)
  │  UDS
  ▼
nautrouds  ── 讀取 examples/nautrouds/Ntufile ──┐
  ├─ */oauth2/*        → oauth2-provider          │  全部走 UDS，
  ├─ */auth/*          → auth-service             │  共用 volume
  ├─ POST /api/upload  → auth-service(/verify) → fileupload
  ├─ */api/*           → auth-service(/verify) → magic-conch-api
  └─ */health          → nautrouds 內建 $health
                                                   │
auth-service ──DATABASE_URL──▶ postgres           │
             ──REDIS_URL─────▶ redis ◀─────────────┘
core ──LLM_API_KEY──▶ Gemini API
core ──GRPC_ADDRESS(UDS)──▶ magic-conch-api
```

所有服務間的呼叫都透過共用 `nautrouds_sockets` volume 上的 Unix domain socket 進行；
只有 `gateway` 對外開放 TCP port（`443`）。nautrouds 所路由的公開 API 合約見
[`openapi.yml`](openapi.yml)。`core` 的 gRPC 介面只供內部使用，合約見
`core/structs/structs.proto` 以及下方摘要。

除了只走標準 OAuth2/OIDC 的測試用 `oauth2-provider`，這裡所有 HTTP 服務都使用相同的
JSON 格式：`{"success": bool, "message": <T>}`。任何 `5xx` 都回傳固定的通用訊息，
真正的錯誤只寫進 server 端 log。

## 設定

在 `examples/.env` 填入 Gemini API key，以及 `fileupload`（簽章）與 `core`（驗證）
共用的 filetoken 簽章金鑰：

```env
LLM_API_KEY=your_gemini_api_key
FILE_TOKEN_KEY=k1:<base64url secret，至少 32 bytes>
```

用以下指令產生 secret：

```bash
echo "k1:$(openssl rand -base64 32 | tr '+/' '-_' | tr -d '=')"
```

接著啟動堆疊並執行 end-to-end 測試：

```bash
docker compose up -d
cd integration_test
go test ./...
```

---

## gateway (Caddy)

**Image:** `caddy:2.10.0` · **設定檔：** `examples/Caddyfile`

唯一對外發佈 TCP port（`443:443`）的服務。使用 `examples/certs/` 中的自簽憑證終止 TLS
（由 `certs/gen-certs.ps1` 產生，SAN 涵蓋 `gateway`/`localhost`/`127.0.0.1`），並把所有
請求 reverse proxy 到 nautrouds 的 entrypoint socket。路由邏輯全部交給 nautrouds，
Caddy 只負責 TLS→UDS 的轉接。

```
{$CADDY_HOST:0.0.0.0}:{$GATEWAY_PORT:443} {
    tls {$TLS_CERT_FILE:...} {$TLS_KEY_FILE:...}
    route { reverse_proxy unix//var/run/nautrouds/entrypoints/nautrouds-0.sock }
}
```

`CADDY_HOST`/`GATEWAY_PORT`/`TLS_CERT_FILE`/`TLS_KEY_FILE` 從環境變數讀取，括號內為
預設值。修改 `GATEWAY_PORT` 時，也要同步修改 `docker-compose.yml` 的 `ports:` 對應。

## nautrouds

**Image:** `lucap9056/nautrouds:26-06.19-05` · **設定檔：** `examples/nautrouds/Ntufile`
**文件/原始碼：** https://github.com/nautrouds/nautrouds

Reverse proxy / service router，依照上方 Ntufile 的路由表透過 UDS 連到各個 backend。
完整語法（wildcard、`$` 開頭的 middleware/virtual service、`service(/path)` 串接
middleware 的寫法）請參考上方連結的 upstream repo。

這份 Ntufile 特有的兩個寫法：

- **`*/api/[|*]`**：`[|*]` 是預設的 catch-all 剩餘路徑（「空字串或任何內容」），所以這條
  規則代表「任何 method、`/api/` 開頭、加上其後的任何路徑」，也就是 `/api/` 底下所有
  尚未被上方較精確的 `POST */api/upload` 規則接走的請求。
- **`auth-service(/verify)`**：轉發前先串接 `auth-service` 的 `/verify` endpoint 作為
  關卡。回應 `204` 時，其 header（`X-Forwarded-User-ID`/`X-Forwarded-Device-ID`）會注入
  到轉發的請求中；非 2xx 則直接回給 client。

Healthcheck（`test -S .../nautrouds-0.sock`）確認的是 socket 檔案存在；要確認 backend
連線請直接呼叫 `GET /health` 或各服務的 endpoint。

## oauth2-provider

**Image:** `oauth2-test-provider:local`（本機 build，未發佈）
**原始碼：** https://github.com/lucap9056/auth-middleware/tree/main/oauth2/test-provider

刻意設計成不安全的 OIDC/OAuth2 stub，專供 E2E 測試——**只能在測試環境使用。**
`/auth` 會直接核發 code 並立即 redirect，**完全不檢查憑證**。每次登入都會以同一個固定
身分通過驗證，身分由環境變數設定（`USER_SUB`/`USER_EMAIL`/`USER_NAME`，預設為
`test-user-001` / `user@example.com` / `Test User`，`docker-compose.yml` 沿用預設值）。
`/token` 回傳的是**未簽章（`alg:none`）的 `id_token`**。Endpoint：
`/.well-known/openid-configuration`、`/auth`、`/token`、`/userinfo`。

因為 `OAUTH2_AUTH_URL=https://localhost/oauth2/auth` 會在 client 端登入流程中把瀏覽器
導到這裡，所以它也經由 gateway 路由，供瀏覽器與服務共同使用。

## auth-service

**Image:** `lucap9056/oauth2-middleware:2.0.0`
**原始碼：** https://github.com/lucap9056/auth-middleware/tree/main/oauth2

Repo 名稱是 `auth-middleware`，其中的 `/oauth2` module 以 `oauth2-middleware` image
發佈（同一個專案）。該 repo 另有獨立的 `/jwt` 與 `/database` module——
`init-sql-fetcher` 會從 `/database` 的固定 commit 抓取 `init.sql`。

通用的 OAuth2/OIDC 登入 + JWT session middleware（支援 Discord/Google/GitHub/任何
OIDC/OAuth2），在這裡透過 `OIDC_ISSUER_URL=https://gateway/oauth2` 指向
`oauth2-provider`。運作模式由 `DATABASE_URL`/`PASS_OAUTH_TOKEN` 決定，共三種；本部署
設定 `DATABASE_URL`、未設定 `PASS_OAUTH_TOKEN`，因此運作於 **Stateful Revoke** 模式：
簽發自己的 JWT，並撤銷 provider 的 token。

路由（掛載於 `/auth/*`，轉發前會去掉前綴）：

| Method 與路徑 | 認證 | 回傳 |
|---|---|---|
| `GET /login` | — | `{message:{url, verifier?}}`（`verifier` 屬於 client 持有 PKCE 的模式，本部署用不到） |
| `GET /callback` | query `code`、`state`；可選 header `X-Device-Name` | `{message:{access_token, refresh_token}}` |
| `GET /verify` | Bearer | `204` + `X-Forwarded-User-ID`/`X-Forwarded-Device-ID`，或 `401` |
| `POST /refresh` | cookie/header/body 中的 refresh token | `{message:{access_token, refresh_token}}`（兩者都會輪替） |
| `POST /refresh-access` | 同上 | `{message:"<access_token>"}`（只換 access token） |
| `POST /logout` | 同上 | `{message:"Logged out and device session revoked"}` |
| `DELETE /users/me` | Bearer | `{message:"Account deleted"}`，連帶刪除 `user_devices` |
| `GET /health` | — | `200 OK` |

`GET /verify` 也是 nautrouds 用在 `/api/*` 上的串接認證 middleware；由於 `*/auth/*`
整組路由到這裡，client 也能直接呼叫它。

Entrypoint 會先執行 `update-ca-certificates`（信任 `gatewayCA.cert`），讓它透過
`OAUTH2_TOKEN_URL=https://gateway/oauth2/token` 回呼時能驗證 gateway 的自簽憑證。

## core

**Image:** `ghcr.io/lucap9056/magic-conch-shell/core:3.0.0` · **原始碼：** `core/`（本 repo）

LLM 引擎，透過 `GRPC_ADDRESS=unix:///var/run/nautrouds/core.sock` 以 gRPC 提供給
`magic-conch-api` 使用。合約為 `core/structs/structs.proto`：unary
`AssistantService.Chat(Request) → Response`，其中
`Request{language: Language, current_message, history_messages}`、
`PromptMessage{parts: []PromptPart{type, content, mime_type}}`。錯誤以 gRPC status code
回傳，message 為給使用者看的回覆。

主要環境變數：`LLM_API_KEY`（Gemini——⚠ `examples/.env` 以明文存放真實金鑰，已被
gitignore，但請視為機敏資料）、`MODEL_NAME`，以及用來驗證 `fileupload` 所發 filetoken 的
`FILE_TOKEN_KEYS=${FILE_TOKEN_KEY}`。啟動時會列出 `LLM_API_KEY` 可用的模型，
`MODEL_NAME` 不在其中就結束行程。Compose 的 healthcheck 會對 gRPC socket 執行
`grpc_health_probe`，`magic-conch-api` 會等它通過後才啟動。

## fileupload

**Image:** `ghcr.io/lucap9056/magic-conch-shell/fileupload:2.0.0` · **原始碼：** `fileupload/`（本 repo）

`POST /upload`（掛載於 `/api/upload`；JWT 由上游的 nautrouds 驗證）。接受
`multipart/form-data` 的 `file` 欄位，上限 20 MiB。以 `http.DetectContentType` 判斷 MIME，
判斷不出來時改用副檔名；允許 `image/png`、`image/jpeg`、`image/webp`、`image/heif`、
`image/heic`。上傳到 Gemini Files API 後，以 `FILE_TOKEN_KEY` 把 `{uri, mime, exp}` 簽成
filetoken，回傳 `{message:{key: "<filetoken>", mime_type}}`。Token 有效 47 小時，
落在 Gemini Files 的 48 小時保存期限內。`REDIS_URL` 以檔案 hash 快取已發出的 token，
重複上傳同一張圖會拿到同一個 token。

`GEMINI_API_KEY` 必須與 `core` 的 `LLM_API_KEY` 屬於同一個 Gemini project——這裡兩者都讀取
`${LLM_API_KEY}`。

## magic-conch-api

**Image:** `ghcr.io/lucap9056/magic-conch-shell/httpserver:3.0.0` · **原始碼：** `httpserver/`（本 repo）

位於 `core` 前方的輕量 HTTP↔gRPC 轉接層，也就是實際的聊天 endpoint。提供 `GET /health`
與 `POST /`（掛載於 `POST /api/`；JWT 由上游的 nautrouds 驗證）。Body 以 `protojson`
解析，上限 1 MiB：`language` 填 `LANGUAGE_EN`/`LANGUAGE_ZH`（或 `1`/`2`），欄位名稱
snake_case 與 camelCase 皆可，未知欄位回 `400`，`current_message` 為必填。以 5 秒
timeout 透過 gRPC 呼叫 `core`，並依 status code 對應 HTTP status：

| gRPC code | HTTP | `message` |
|---|---|---|
| `InvalidArgument`、`FailedPrecondition` | `400` | `core` 給使用者看的回覆 |
| `ResourceExhausted` | `429` | `core` 給使用者看的回覆 |
| `Unavailable` | `503` | `service temporarily unavailable` |
| `DeadlineExceeded`、`Canceled` | `504` | `request timed out` |
| 其他 | `500` | `internal server error` |

## postgres

**Image:** `postgres:16-alpine`——只供 `auth-service` 使用。Schema 由 `init-sql-fetcher`
從 `auth-middleware` repo 的**固定 commit**（`1e17700df4f504fc11288a4d6ef679f2d7856f06`）
抓取 `database/init.sql`。升級 `auth-service` 版本時，請同步更新這個 commit，讓 schema
保持一致。

```sql
users        (user_id PK, username, email UNIQUE)
user_devices (device_id PK, device_name, user_id FK → users ON DELETE CASCADE,
              secret, updated_at [indexed])
```

資料存放在 `./postgres_data`（bind mount，已 gitignore）——刪除該目錄即可重置資料庫。

## redis

**Image:** `redis:7-alpine`——共用實例，以 key 前綴區分使用者：

| 使用者 | Key 格式 | 用途 | TTL |
|---|---|---|---|
| `fileupload` | `fileupload:token:<mime>:<sha256>` | 檔案 hash → 已發出的 filetoken | token 有效期減 5 分鐘（約 47 小時） |
| `auth-service` | 內部使用（PKCE state、refresh token 快取） | 登入流程狀態 + session 快取 | state：短期；token：`REFRESH_TOKEN_TTL`（24h） |

Redis 以記憶體模式運作：重啟後待完成的登入與快取的 refresh token 會清空，使用者需重新登入；
`postgres` 中的使用者與裝置資料維持不變。
