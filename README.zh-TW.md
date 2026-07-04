# Magic Conch Shell (神奇海螺)

[![Go Version](https://img.shields.io/github/go-mod/go-version/lucap9056/magic-conch-shell?filename=core/go.mod)](https://go.dev/)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

這是一個基於 Go 語言、Google Gemini AI 以及 Lua 引擎開發的分散式決策系統。靈感來自於《海綿寶寶》中的「神奇海螺」，旨在為使用者的任何疑問提供絕對、隨機且有趣的決策建議。

## 核心特色

- **AI 驅動的決策解析**：利用 Google Gemini (Pro/Flash) 模型解析使用者意圖，而非直接回答問題。
- **Lua 確定性引擎**：AI 會生成 Lua 代碼，並在受限的沙盒環境中執行，確保決策過程既具有隨機性又符合邏輯規則。
- **多端整合**：
  - **Discord Bot**：支援文字與圖片輸入，並能追蹤對話上下文。
  - **gRPC API**：高效能的後端通訊，支援 TLS 加密。
  - **HTTP API**：方便 Web 或其他服務整合。
- **圖片分析能力**：支援 Discord 附件圖片分析，透過 imagecache 自動處理圖片下載、快取與 Gemini 上傳。
- **靈活部署**：支援分散式 (gRPC) 或單體式 (Integrated) 部署模式。

## 系統架構

```mermaid
graph TD
    User([使用者]) --> Discord[Discord Frontend]
    User --> Web[Web/HTTP Client]
    
    Discord -- gRPC --> Core[Core Service]
    Web -- HTTP 聊天 --> HTTP[HTTP Server]
    Web -- HTTP 上傳 --> FileUpload[File Upload Service]
    HTTP -- gRPC --> Core
    
    subgraph "Core Service"
        Gemini[Google Gemini API]
        Lua[Lua Sandbox Engine]
    end
    
    Core --> Lua
    Core <--> Gemini
    
    ImageCache[(圖片快取<br/>BadgerDB, 或透過 IMAGE_CACHE_PATH 改用 Redis)]
    FileUpload -- 上傳 --> Gemini
    FileUpload -. 儲存 rdx:// 參照 .-> ImageCache
    Core -. 解析 rdx:// 參照 .-> ImageCache
```

`FileUpload` 與 `Core` 共用同一份圖片快取：`FileUpload` 把圖片上傳到 Gemini 一次
後存下一個不透明的 `rdx://` 參照，之後 `Core` 直接解析這個參照即可，不需重新上
傳——完整的實際部署流程可參考 [`examples/README.md`](examples/README.md)。

## 目錄說明

- `/core`: 核心邏輯，包含 AI 解析器、Lua 執行環境與 gRPC 伺服器。
- `/discord`: Discord 機器人前端實現。
- `/httpserver`: 基於 HTTP 的 API 轉發層。
- `/fileupload`: 在聊天請求之前先上傳圖片用的 HTTP 服務（見上方架構圖）。
- `/grpcclient`: 通用的 gRPC 客戶端封裝。
- `/integrated`: 將核心服務與特定前端（如 Discord）整合在同一個行程中的輕量級版本。
- `/examples`: 完整的 Docker Compose 部署範例（gateway、OAuth2/JWT 認證、所有服務），內含自己的 [README](examples/README.md) 與 [OpenAPI 規格](examples/openapi.yml)。

## 快速開始

最快上手的方式是使用**單體式 (integrated)** 版本：Core 與 Discord 機器人在同一
個行程中執行，不需要架設 gRPC 伺服器，也不需要另外設定圖片快取後端。

### 前置要求

- Go 1.22 或以上版本
- Google AI (Gemini) API Key
- Discord Bot Token

### 環境變數配置

在 `integrated/discord/cmd` 下建立 `.env` 檔案（環境變數是在啟動當下的工作目
錄讀取的——原因見下方 [服務說明](#服務說明)）：

```env
LLM_API_KEY=your_gemini_api_key
MODEL_NAME=gemini-1.5-flash
ALLOWED_IMAGE_DOMAINS=cdn.discordapp.com,media.discordapp.net
DISCORD_TOKEN=your_discord_bot_token
```

### 執行

```bash
cd integrated/discord/cmd
go run main.go
```

若要以分散式方式部署（Core、Discord、HTTP Server、File Upload 各自獨立、透過
gRPC 連接），請參考下方 [服務說明](#服務說明)。若要體驗接近正式環境、包含 TLS
閘道與 OAuth2/JWT 認證的完整微服務堆疊（Docker Compose），請參考
[`examples/README.md`](examples/README.md)。

## 服務說明

以下每個服務都是獨立的 Go 模組、獨立的行程。它們讀取環境變數的方式相同：啟動
時會在**當下的工作目錄**尋找 `.env`（或 `.env.local`、`.env.development` 等），
而不是專案根目錄——因此請把每個服務的 `.env` 放在你 `go run main.go` 前
`cd` 進去的那個目錄（預設是各自的 `cmd/` 資料夾）。

### Core — `core/v2/cmd`

gRPC 引擎：Gemini 決策解析、Lua 沙盒引擎，以及可選的互動式終端機模式。啟動時
會對 Gemini API 執行一次即時自我測試，失敗就結束行程。

| 環境變數 | 是否必填 | 預設值 | 用途 |
|---|---|---|---|
| `LLM_API_KEY` | 是 | — | Gemini API 金鑰。 |
| `MODEL_NAME` | 是 | — | Gemini 模型名稱（如 `gemini-1.5-flash`）。 |
| `GRPC_ADDRESS` | 否 | 未設定則不啟動 gRPC 伺服器 | 監聽位址，可為 TCP 或 `unix://...`。 |
| `ALLOWED_IMAGE_DOMAINS` | 否 | 未設定會封鎖所有圖片網域* | 圖片網址主機名稱白名單，以逗號分隔。 |
| `IMAGE_CACHE_PATH` | 否 | 本機 BadgerDB，路徑 `image_cache` | 設為 `redis://...` 會改用 Redis 快取。 |
| `CONSOLE_MODE` | 否 | 關閉 | 設為 `true` 會在 gRPC 伺服器旁同時執行互動式終端機。 |
| `GRPC_TLS_CERT` / `GRPC_TLS_KEY` | 否 | 不加密 | 啟用 gRPC TLS（兩者需同時設定）。 |
| `GRPC_TLS_CA` | 否 | 無 | 啟用 mTLS（驗證用戶端憑證）。 |
| `GRPC_MAX_RECV_MSG_SIZE` / `GRPC_MAX_SEND_MSG_SIZE` | 否 | 各 4 MiB | gRPC 訊息大小上限。 |
| `GRPC_KEEPALIVE_TIME` / `GRPC_KEEPALIVE_TIMEOUT` | 否 | `2h` / `20s` | gRPC keepalive 參數。 |
| `LANG` | 否 | `en` | 終端機模式下的系統指令語言。 |

\* `ALLOWED_IMAGE_DOMAINS` 未設定時會封鎖所有圖片網域，而不是全部允許——若有
使用圖片輸入功能請務必明確設定。

```bash
cd core/v2/cmd
go run main.go
```

### Discord — `discord/cmd`

獨立的 Discord 機器人前端，透過 gRPC 與 Core 通訊。

| 環境變數 | 是否必填 | 預設值 | 用途 |
|---|---|---|---|
| `DISCORD_TOKEN` | 是 | — | Discord Bot Token。 |
| `GRPC_ADDRESS` | 是 | — | 要連線的 Core gRPC 位址。 |
| `GRPC_TLS_CA` | 否 | 無 | 用於驗證 Core TLS 憑證的 CA 憑證。 |
| `GRPC_TLS_CERT` / `GRPC_TLS_KEY` | 否 | 不加密 | 與 Core 進行 mTLS 的用戶端憑證/金鑰。 |
| `LANG` | 否 | `en` | 預設回應語言。 |

```bash
cd discord/cmd
go run main.go
```

### HTTP Server — `httpserver/cmd`

輕量的 HTTP↔gRPC 轉接層，連接 Core。

| 環境變數 | 是否必填 | 預設值 | 用途 |
|---|---|---|---|
| `HTTP_ADDRESS` | 是 | — | 監聽位址，可為 TCP 或 `unix://...`。 |
| `GRPC_ADDRESS` | 是 | — | 要連線的 Core gRPC 位址。 |
| `CORS_ALLOWED_ORIGINS` | 否 | 停用 CORS | 允許的來源清單，以逗號分隔（`*` 代表允許任何來源）。 |
| `GRPC_TLS_CERT` / `GRPC_TLS_KEY` | 否 | 不加密 | 連線 Core gRPC 用的用戶端憑證/金鑰。 |
| `GRPC_TLS_CA` | 否 | 無 | 用於驗證 Core TLS 憑證的 CA 憑證。 |

```bash
cd httpserver/cmd
go run main.go
```

### File Upload — `fileupload/cmd`

獨立的圖片上傳服務，在聊天請求之前先行處理（見上方[系統架構](#系統架構)）；
與 Core 共用同一份圖片快取，透過 `rdx://` 參照連結。

| 環境變數 | 是否必填 | 預設值 | 用途 |
|---|---|---|---|
| `HTTP_ADDRESS` | 是 | — | 監聽位址，可為 TCP 或 `unix://...`。 |
| `GEMINI_API_KEY` | 是 | — | 用於上傳圖片的 Gemini API 金鑰。 |
| `REDIS_URL` | 是 | — | 共用圖片快取所使用的 Redis 連線位址。 |
| `RDX_HOSTNAME` | 否 | `rediscache` | 回傳的 `rdx://<hostname>/<uuid>` 參照中的主機名稱片段——必須與 Core 的 `ALLOWED_IMAGE_DOMAINS` 相符。 |
| `CORS_ALLOWED_ORIGINS` | 否 | 停用 CORS | 與 HTTP Server 相同。 |

注意：本服務使用 `GEMINI_API_KEY`，而 Core 與單體式版本使用 `LLM_API_KEY`
代表同一把金鑰——這是服務間命名不一致，並非筆誤。

```bash
cd fileupload/cmd
go run main.go
```

### 單體式 Discord — `integrated/discord/cmd`

將 Core 與 Discord 整合進單一行程——不使用 gRPC、不需獨立的 Core 行程，圖片
快取固定使用本機 BadgerDB。即[快速開始](#快速開始)所使用的版本。

| 環境變數 | 是否必填 | 預設值 | 用途 |
|---|---|---|---|
| `LLM_API_KEY` | 是 | — | Gemini API 金鑰。 |
| `MODEL_NAME` | 是 | — | Gemini 模型名稱。 |
| `DISCORD_TOKEN` | 是 | — | Discord Bot Token。 |
| `ALLOWED_IMAGE_DOMAINS` | 否 | 未設定會封鎖所有圖片網域* | 與 Core 相同的注意事項。 |
| `LANG` | 否 | `en` | 預設回應語言。 |

```bash
cd integrated/discord/cmd
go run main.go
```

## 使用方法

### Discord
- **直接標記機器人**：@神奇海螺 我今天該吃拉麵還是漢堡？
- **回覆對話**：機器人會追蹤對話上下文來進行決策。
- **圖片輸入**：你可以傳送圖片並問「我該買哪件衣服？」，機器人會分析圖片內容。
- **斜線指令**：
  - `/q`: 向海螺提問。
  - `/set-channel`: 設定機器人主動監聽的頻道（僅限管理員）。

### 終端機模式 (Core Console)
啟動 core 服務後，可以直接在終端機輸入問題進行測試。

## 開發與自定義

### 自定義決策邏輯
你可以修改 core/v2/assistant/luascript/script.lua 來自定義 Lua 函數的行為，或修改 core/v2/assistant/systemInstruction 來調整 AI 生成 Lua 代碼的引導語。
