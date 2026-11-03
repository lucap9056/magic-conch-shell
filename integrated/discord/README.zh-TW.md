[English](./README.md) | 繁體中文
# 單體式 Discord — Docker

單體式 Discord bot 的 Docker 與 Docker Compose 範例。功能、使用方法與完整環境變數請參考[根目錄 README](../../README.zh-TW.md#單體式-discord--integrateddiscordcmd)。

先建立一個空的 `channels.db`，用來在重啟後保留回應頻道的設定：

```bash
touch channels.db
```

## Docker

```bash
docker run -d --name magic-conch-discord \
  -e LLM_API_KEY=your_gemini_api_key \
  -e MODEL_NAME=gemini-2.5-flash-lite \
  -e DISCORD_TOKEN=your_discord_bot_token \
  -e ALLOWED_IMAGE_DOMAINS=cdn.discordapp.com,media.discordapp.net \
  -e IMAGE_CACHE_PATH=/data/image_cache \
  -e LANG=zh \
  -v magic-conch-data:/data \
  -v "$(pwd)/channels.db:/app/channels.db" \
  --restart unless-stopped \
  ghcr.io/lucap9056/magic-conch-shell/integrated-discord:2.0.0
```

## Docker Compose

`.env`：

```env
LLM_API_KEY=your_gemini_api_key
DISCORD_TOKEN=your_discord_bot_token
```

`docker-compose.yml`：

```yaml
services:
  magic-conch-discord:
    image: ghcr.io/lucap9056/magic-conch-shell/integrated-discord:2.0.0
    environment:
      LLM_API_KEY: ${LLM_API_KEY}
      MODEL_NAME: gemini-2.5-flash-lite
      DISCORD_TOKEN: ${DISCORD_TOKEN}
      ALLOWED_IMAGE_DOMAINS: cdn.discordapp.com,media.discordapp.net
      IMAGE_CACHE_PATH: /data/image_cache
      LANG: zh
    volumes:
      - magic-conch-data:/data
      - ./channels.db:/app/channels.db
    restart: unless-stopped

volumes:
  magic-conch-data:
```

```bash
docker compose up -d
```

## 建置 Image

```bash
docker build -f docker/Dockerfile -t magic-conch-discord .
```
