English | [繁體中文](./README.zh-TW.md)
# Integrated Discord — Docker

Docker and Docker Compose examples for the integrated Discord bot. For features, usage, and the full environment variable list, see the [root README](../../README.md#integrated-discord--integrateddiscordcmd).

Create an empty `channels.db` to keep the response-channel setting across restarts:

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
  -e LANG=en \
  -v magic-conch-data:/data \
  -v "$(pwd)/channels.db:/app/channels.db" \
  --restart unless-stopped \
  ghcr.io/lucap9056/magic-conch-shell/integrated-discord:2.0.0
```

## Docker Compose

`.env`:

```env
LLM_API_KEY=your_gemini_api_key
DISCORD_TOKEN=your_discord_bot_token
```

`docker-compose.yml`:

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
      LANG: en
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

## Build the Image

```bash
docker build -f docker/Dockerfile -t magic-conch-discord .
```
