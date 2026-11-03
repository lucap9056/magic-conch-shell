[English](./README.md) | 繁體中文
# `examples/` 堆疊

Magic Conch Shell 的完整整合測試部署：TLS gateway →
認證（通用 OIDC/OAuth2 + JWT）→ 聊天/上傳服務 → LLM core。

```
Client (integration_test.go)
  │  HTTPS :443
  ▼
gateway (Caddy) ── 讀取 examples/Caddyfile
  ├─ /health           → respond "OK"
  ├─ /oauth2/*         → dex:5556
  ├─ /auth/*           → corvauth:80
  ├─ POST /api/upload  → forward_auth corvauth/verify → fileupload:8080
  └─ /api/*            → forward_auth corvauth/verify → magic-conch-api:8080
                                                        │ gRPC
                                                        ▼
                                                     core:50051 ──▶ Gemini API
corvauth ──▶ postgres、redis、dex（token/userinfo）
fileupload ──▶ redis、Gemini API
```

服務之間透過 compose 網路以 TCP 連線；只有 `gateway` 對外開放 port（`443`）。gateway
所路由的公開 API 合約見 [`openapi.yml`](openapi.yml)。`core` 的 gRPC 介面只供內部使用，
合約見 `core/structs/structs.proto` 以及下方摘要。

除了只走標準 OAuth2/OIDC 的 `dex`，這裡所有 HTTP 服務都使用相同的 JSON 格式：
`{"success": bool, "message": <T>}`。任何 `5xx` 都回傳固定的通用訊息，真正的錯誤只寫進
server 端 log。

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

唯一對外發佈 port（`443:443`）的服務。使用 `examples/certs/` 中的自簽憑證終止 TLS
（由 `certs/gen-certs.ps1` 產生，SAN 涵蓋 `gateway`/`localhost`/`127.0.0.1`），並依下表路由：

| 路由 | Upstream | 說明 |
|---|---|---|
| `/health` | — | `respond "OK" 200` |
| `/oauth2/*` | `dex:5556` | 保留路徑（Dex 的 issuer 路徑即 `/oauth2`） |
| `/auth/*` | `corvauth:80` | 去掉 `/auth` |
| `POST /api/upload` | `fileupload:8080` | `verified_api`，去掉 `/api` |
| `/api/*` | `magic-conch-api:8080` | `verified_api`，去掉 `/api` |

`verified_api` snippet 在 `route` 區塊中依序執行：

1. 移除 client 自行帶入的 `X-Forwarded-User-Email`/`X-Forwarded-Device-ID`/`X-Forwarded-Username`。
2. `forward_auth corvauth:80` 以請求的 `Authorization` header 呼叫 `/verify`。回應 `2xx` 時把這三個
   header 複製到請求上；其他狀態（例如 `401`）則把 corvauth 的回應直接回給 client。
3. 轉發前去掉 `/api`。

`CADDY_HOST`（bind 位址）、`GATEWAY_PORT`、`TLS_CERT_FILE`、`TLS_KEY_FILE` 從環境變數讀取，
括號內為預設值。修改 `GATEWAY_PORT` 時，也要同步修改 `docker-compose.yml` 的 `ports:` 對應。

## dex

**Image:** `ghcr.io/dexidp/dex:v2.46.1`（`dex`）
**設定檔：** `examples/dex/config.yaml` · **文件：** https://dexidp.io/docs/

以 [Dex](https://github.com/dexidp/dex) 作為 OIDC/OAuth2 provider，設定只適用於 E2E 測試：
`mockCallback` connector 讓每次登入都以同一個 mock 使用者通過（`kilgore@kilgore.trout`，
`email_verified: true`），不需要登入表單；`skipApprovalScreen: true` 會跳過授權確認頁。
Token 以 RS256 簽章，資料存放在記憶體。Issuer 為 `https://localhost/oauth2`，因此 Dex 的
所有 endpoint 都在 `/oauth2/` 底下（`/auth`、`/token`、`/userinfo`、`/keys`、
`/.well-known/openid-configuration`）。

Dex 監聽 `dex:5556`。因為 `OAUTH2_AUTH_URL=https://localhost/oauth2/auth` 會在 client 端
登入流程中把瀏覽器導到這裡，所以 gateway 會把 `/oauth2/*` 路由到 Dex。從 `/oauth2/auth`
開始，依序跟隨 Dex 的 302/303 redirect，直到 `Location` 抵達 `https://localhost/auth/callback`。

`examples/dex/config.yaml` 註冊了 `test-client` / `test-secret` 這個 client，redirect URI 為
`https://localhost/auth/callback`。

## corvauth

**Image:** `ghcr.io/lucap9056/corvauth:0.4`
**原始碼：** https://github.com/lucap9056/corvauth

裝置綁定的 session 認證 server，以 **managed users** 模式運作：設定了 `DATABASE_URL`，
`DB_AUTO_CREATE_SCHEMA=true` 會在啟動時建立 `users` 與 `auth_user_devices`，
`ALLOW_REGISTRATION=true` 會為第一次登入的使用者建立帳號。它以 generic provider
（`OAUTH2_PROVIDER=test-provider`）透過 Dex 登入：瀏覽器前往
`OAUTH2_AUTH_URL=https://localhost/oauth2/auth`，corvauth 則直接呼叫
`http://dex:5556/oauth2/...` 上的 `OAUTH2_TOKEN_URL`/`OAUTH2_USERINFO_URL`。

路由（掛載於 `/auth/*`，轉發前會去掉前綴）：

| Method 與路徑 | 輸入 | 回傳 |
|---|---|---|
| `GET /login` | — | `{message:{url}}` |
| `GET /callback` | query `code`、`state`；可選 header `X-Device-Name` | `{message:{access_token, refresh_token}}` + `refresh_token` cookie |
| `GET /verify` | Bearer | `204` + `X-Forwarded-User-Email`/`X-Forwarded-Device-ID`/`X-Forwarded-Username`，或 `401` |
| `GET /verify-optional` | Bearer（可選） | `204`，登入時才附帶身分 header |
| `POST /refresh` | `refresh_token` cookie 或 body | `{message:{access_token, refresh_token}}`（兩者都會輪替） |
| `POST /refresh-access` | 同上 | `{message:"<access_token>"}`（只換 access token） |
| `POST /refresh-status` | 同上 | `{message:{device_id, issued_at, expires_at}}` |
| `POST /logout` | 同上（可選） | `{message:"Logged out and device session revoked"}` |
| `DELETE /users/me` | Bearer | `{message:"Account deleted"}`，刪除所有 session |
| `GET /health` | — | `200 OK` |

`GET /verify` 也是 gateway 對 `/api/*` 執行 `forward_auth` 的目標。完整 API 與設定請參考
[corvauth 文件](https://github.com/lucap9056/corvauth/tree/main/docs)。

## core

**Image:** `ghcr.io/lucap9056/magic-conch-shell/core:3.0.0` · **原始碼：** `core/`（本 repo）

LLM 引擎，透過 `GRPC_ADDRESS=:50051` 以 gRPC 提供給 `magic-conch-api`（`core:50051`）
使用。合約為 `core/structs/structs.proto`：unary
`AssistantService.Chat(Request) → Response`，其中
`Request{language: Language, current_message, history_messages}`、
`PromptMessage{parts: []PromptPart{type, content, mime_type}}`。錯誤以 gRPC status code
回傳，message 為給使用者看的回覆。

主要環境變數：`LLM_API_KEY`（Gemini——⚠ `examples/.env` 以明文存放真實金鑰，已被
gitignore，但請視為機敏資料）、`MODEL_NAME`，以及用來驗證 `fileupload` 所發 filetoken 的
`FILE_TOKEN_KEYS=${FILE_TOKEN_KEY}`。啟動時會列出 `LLM_API_KEY` 可用的模型，
`MODEL_NAME` 不在其中就結束行程。Compose 的 healthcheck 會執行
`grpc_health_probe -addr=localhost:50051`，`magic-conch-api` 會等它通過後才啟動。

## fileupload

**Image:** `ghcr.io/lucap9056/magic-conch-shell/fileupload:2.0.0` · **原始碼：** `fileupload/`（本 repo）

`:8080` 上的 `POST /upload`（由 `/api/upload` 路由而來；JWT 由 gateway 驗證）。接受
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
與 `:8080` 上的 `POST /`（由 `POST /api/` 路由而來；JWT 由 gateway 驗證）。Body 以 `protojson`
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

**Image:** `postgres:16-alpine`——只供 `corvauth` 使用。corvauth 會在啟動時建立資料表
（`DB_AUTO_CREATE_SCHEMA=true`）：`users` 與 `auth_user_devices`。

資料存放在 `./postgres_data`（bind mount，已 gitignore）——刪除該目錄即可重置資料庫。

## redis

**Image:** `redis:7-alpine`——共用實例，以 key 前綴區分使用者：

| 使用者 | Key 格式 | 用途 | TTL |
|---|---|---|---|
| `fileupload` | `fileupload:token:<mime>:<sha256>` | 檔案 hash → 已發出的 filetoken | token 有效期減 5 分鐘（約 47 小時） |
| `corvauth` | 內部使用（登入 `state`、裝置 secret 快取、`/callback`/`/refresh` 去重） | 登入流程狀態 + session 快取 | state：短期 |

Redis 以記憶體模式運作：重啟後待完成的登入與快取的裝置 secret 會清空；`postgres` 保留
使用者與裝置 session。
