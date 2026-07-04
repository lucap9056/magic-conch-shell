English | [繁體中文](./README.zh-TW.md)
# Magic Conch Shell

[![Go Version](https://img.shields.io/github/go-mod/go-version/lucap9056/magic-conch-shell?filename=core/go.mod)](https://go.dev/)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

Magic Conch Shell is a distributed decision-making system built with Go, Google Gemini AI, and a Lua engine. Inspired by the "Magic Conch Shell" from SpongeBob SquarePants, it provides absolute, randomized, and humorous decision suggestions for any user inquiry.

## Core Features

- **AI-Driven Decision Parsing**: Utilizes Google Gemini (Pro/Flash) models to parse user intent rather than providing direct answers.
- **Lua Deterministic Engine**: The AI generates Lua code that executes within a restricted sandbox environment, ensuring decisions are both randomized and logically consistent.
- **Multi-Platform Integration**:
  - **Discord Bot**: Supports text and image inputs with conversation context tracking.
  - **gRPC API**: High-performance backend communication with TLS support.
  - **HTTP API**: Easy integration for Web and other services.
- **Image Analysis**: Supports Discord image attachments, handled automatically via imagecache for downloading, caching, and uploading to Gemini.
- **Flexible Deployment**: Supports both distributed (gRPC) and integrated (monolithic) deployment modes.

## System Architecture

```mermaid
graph TD
    User([User]) --> Discord[Discord Frontend]
    User --> Web[Web/HTTP Client]
    
    Discord -- gRPC --> Core[Core Service]
    Web -- HTTP chat --> HTTP[HTTP Server]
    Web -- HTTP upload --> FileUpload[File Upload Service]
    HTTP -- gRPC --> Core
    
    subgraph "Core Service"
        Gemini[Google Gemini API]
        Lua[Lua Sandbox Engine]
    end
    
    Core --> Lua
    Core <--> Gemini
    
    ImageCache[(Image Cache<br/>BadgerDB, or Redis via IMAGE_CACHE_PATH)]
    FileUpload -- upload --> Gemini
    FileUpload -. store rdx:// ref .-> ImageCache
    Core -. resolve rdx:// ref .-> ImageCache
```

`FileUpload` and `Core` share the same image cache: `FileUpload` uploads an image
to Gemini once and stores an opaque `rdx://` reference to it, `Core` resolves
that reference later instead of re-uploading — see `examples/README.md` for a
full walkthrough of this handoff in a real deployment.

## Directory Structure

- `/core`: Core logic, including AI parser, Lua execution environment, and gRPC server.
- `/discord`: Discord bot frontend implementation.
- `/httpserver`: HTTP API wrapper layer.
- `/fileupload`: HTTP service for uploading images ahead of a chat request (see diagram above).
- `/grpcclient`: Common gRPC client package.
- `/integrated`: Lightweight version integrating core services and frontends into a single process.
- `/examples`: Full Docker Compose deployment (gateway, OAuth2/JWT auth, all services) with its own [README](examples/README.md) and [OpenAPI spec](examples/openapi.yml).

## Quick Start

The fastest way to try Magic Conch Shell is the **integrated** build: Core and
the Discord bot run in a single process, so there's no gRPC server or separate
image-cache backend to set up.

### Prerequisites

- Go 1.22 or higher
- Google AI (Gemini) API Key
- Discord Bot Token

### Configuration

Create a `.env` file in `integrated/discord/cmd` (env vars are loaded from the
current working directory at launch — see [Services](#services) below):

```env
LLM_API_KEY=your_gemini_api_key
MODEL_NAME=gemini-1.5-flash
ALLOWED_IMAGE_DOMAINS=cdn.discordapp.com,media.discordapp.net
DISCORD_TOKEN=your_discord_bot_token
```

### Run

```bash
cd integrated/discord/cmd
go run main.go
```

For the distributed deployment (Core, Discord, HTTP Server, and File Upload as
separate gRPC-connected processes), see [Services](#services). For a
production-like microservices stack with a TLS gateway and OAuth2/JWT auth via
Docker Compose, see [`examples/README.md`](examples/README.md).

## Services

Each service is its own Go module and its own process. All of them load env
vars the same way on startup: they look in the **current working directory**
for `.env` (or `.env.local`, `.env.development`, etc.), not the repo root — so
place each service's `.env` in the directory you `cd` into before `go run
main.go` (its `cmd/` folder, by default).

### Core — `core/v2/cmd`

The gRPC engine: Gemini-backed decision parsing, the Lua sandbox, and an
optional interactive console. Runs a live self-test call against the Gemini
API on startup and exits if it fails.

| Env var | Required | Default | Purpose |
|---|---|---|---|
| `LLM_API_KEY` | Yes | — | Gemini API key. |
| `MODEL_NAME` | Yes | — | Gemini model name (e.g. `gemini-1.5-flash`). |
| `GRPC_ADDRESS` | No | server doesn't start | Listen address, TCP or `unix://...`. |
| `ALLOWED_IMAGE_DOMAINS` | No | blocks all image domains if unset* | Comma-separated hostname allow-list for image URLs. |
| `IMAGE_CACHE_PATH` | No | local BadgerDB at `image_cache` | A `redis://...` URL switches to a Redis-backed cache. |
| `CONSOLE_MODE` | No | off | `true` runs an interactive REPL alongside the gRPC server. |
| `GRPC_TLS_CERT` / `GRPC_TLS_KEY` | No | insecure | Enables gRPC TLS (both required together). |
| `GRPC_TLS_CA` | No | none | Enables mTLS (verifies client certs). |
| `GRPC_MAX_RECV_MSG_SIZE` / `GRPC_MAX_SEND_MSG_SIZE` | No | 4 MiB each | Max gRPC message size. |
| `GRPC_KEEPALIVE_TIME` / `GRPC_KEEPALIVE_TIMEOUT` | No | `2h` / `20s` | gRPC keepalive parameters. |
| `LANG` | No | `en` | Console-mode system-instruction language. |

\* An unset `ALLOWED_IMAGE_DOMAINS` blocks every image domain rather than
allowing all — set it explicitly if image input is used.

```bash
cd core/v2/cmd
go run main.go
```

### Discord — `discord/cmd`

Standalone Discord bot frontend; talks to Core over gRPC.

| Env var | Required | Default | Purpose |
|---|---|---|---|
| `DISCORD_TOKEN` | Yes | — | Discord bot token. |
| `GRPC_ADDRESS` | Yes | — | Core's gRPC address to dial. |
| `GRPC_TLS_CA` | No | none | CA cert to verify Core's TLS cert. |
| `GRPC_TLS_CERT` / `GRPC_TLS_KEY` | No | insecure | Client cert/key for mTLS to Core. |
| `LANG` | No | `en` | Default response language. |

```bash
cd discord/cmd
go run main.go
```

### HTTP Server — `httpserver/cmd`

Thin HTTP↔gRPC bridge to Core.

| Env var | Required | Default | Purpose |
|---|---|---|---|
| `HTTP_ADDRESS` | Yes | — | Listen address, TCP or `unix://...`. |
| `GRPC_ADDRESS` | Yes | — | Core's gRPC address to dial. |
| `CORS_ALLOWED_ORIGINS` | No | CORS disabled | Comma-separated allow-list (`*` allows any origin). |
| `GRPC_TLS_CERT` / `GRPC_TLS_KEY` | No | insecure | Client cert/key for calling Core over gRPC. |
| `GRPC_TLS_CA` | No | none | CA cert to verify Core's TLS cert. |

```bash
cd httpserver/cmd
go run main.go
```

### File Upload — `fileupload/cmd`

Independent service for uploading images ahead of a chat request (see
[System Architecture](#system-architecture)); shares an image cache with Core
via `rdx://` references.

| Env var | Required | Default | Purpose |
|---|---|---|---|
| `HTTP_ADDRESS` | Yes | — | Listen address, TCP or `unix://...`. |
| `GEMINI_API_KEY` | Yes | — | Gemini API key used to upload images. |
| `REDIS_URL` | Yes | — | Redis connection URL for the shared image cache. |
| `RDX_HOSTNAME` | No | `rediscache` | Hostname segment in the returned `rdx://<hostname>/<uuid>` reference — must match Core's `ALLOWED_IMAGE_DOMAINS`. |
| `CORS_ALLOWED_ORIGINS` | No | CORS disabled | Same as HTTP Server. |

Note: this service uses `GEMINI_API_KEY` while Core and the integrated build
use `LLM_API_KEY` for the same underlying key — a naming inconsistency across
services, not a typo.

```bash
cd fileupload/cmd
go run main.go
```

### Integrated Discord — `integrated/discord/cmd`

Core and Discord bundled into a single process — no gRPC, no separate Core
process, always a local BadgerDB image cache. Used by [Quick Start](#quick-start)
above.

| Env var | Required | Default | Purpose |
|---|---|---|---|
| `LLM_API_KEY` | Yes | — | Gemini API key. |
| `MODEL_NAME` | Yes | — | Gemini model name. |
| `DISCORD_TOKEN` | Yes | — | Discord bot token. |
| `ALLOWED_IMAGE_DOMAINS` | No | blocks all image domains if unset* | Same caveat as Core. |
| `LANG` | No | `en` | Default response language. |

```bash
cd integrated/discord/cmd
go run main.go
```

## Usage

### Discord
- **Mention the Bot**: "@MagicConch Should I have ramen or a burger today?"
- **Reply to Conversations**: The bot tracks context to make informed decisions.
- **Image Input**: Send an image and ask "Which outfit should I buy?", and the bot will analyze the content.
- **Slash Commands**:
  - `/q`: Ask the shell a question.
  - `/set-channel`: Set the designated response channel (Administrators only).

### Console Mode (Core)
When the core service starts, you can type questions directly into the terminal for testing.

## Development and Customization

### Custom Decision Logic
Modify `core/v2/assistant/luascript/script.lua` to customize Lua function behavior, or adjust `core/v2/assistant/systemInstruction` to refine how the AI generates Lua code.
