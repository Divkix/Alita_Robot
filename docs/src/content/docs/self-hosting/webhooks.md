---
title: Webhook Configuration
description: Configure webhooks for production deployments of Alita Robot.
---

# Webhook Configuration

Webhooks provide real-time message delivery from Telegram to your bot, making them ideal for production deployments. This guide covers the setup and configuration of webhooks for Alita Robot.

## Polling vs Webhooks

| Feature | Polling | Webhooks |
|---------|---------|----------|
| **Latency** | 1-3 seconds | Instant (~50ms) |
| **Setup Complexity** | Simple | Requires HTTPS |
| **Resource Usage** | Higher (constant requests) | Lower (on-demand) |
| **Network** | Works behind NAT | Requires public endpoint |
| **Use Case** | Development, testing | Production |
| **External Dependencies** | None | Reverse proxy or tunnel |

## Required Environment Variables

```bash
# Enable webhook mode
USE_WEBHOOKS=true

# Your public HTTPS domain (required)
WEBHOOK_DOMAIN=https://your-domain.com

# Random secret for webhook validation (recommended)
WEBHOOK_SECRET=your-random-secret-string

# HTTP server port (default: 8080)
HTTP_PORT=8080
```

## Unified HTTP Server

Alita Robot uses a single HTTP server for all endpoints:

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/health` | GET | Health check with database and Redis status |
| `/metrics` | GET | Prometheus metrics for monitoring |
| `/webhook/{secret}` | POST | Telegram webhook endpoint (webhook mode only) |

All endpoints run on the port specified by `HTTP_PORT` (default: 8080).

## Webhook URL Format

Telegram will send updates to:

```
https://{WEBHOOK_DOMAIN}/webhook/{WEBHOOK_SECRET}
```

For example, if:
- `WEBHOOK_DOMAIN=https://bot.example.com`
- `WEBHOOK_SECRET=abc123`

The webhook URL will be: `https://bot.example.com/webhook/abc123`

## Cloudflare Tunnel Setup

Cloudflare Tunnel is the recommended way to expose your bot to the internet without opening ports or managing certificates.

### Step 1: Install cloudflared

```bash
# macOS
brew install cloudflare/cloudflare/cloudflared

# Linux (Debian/Ubuntu)
curl -L https://pkg.cloudflare.com/cloudflare-main.gpg | sudo apt-key add -
echo "deb https://pkg.cloudflare.com/cloudflared $(lsb_release -cs) main" | sudo tee /etc/apt/sources.list.d/cloudflared.list
sudo apt update && sudo apt install cloudflared

# Docker
docker pull cloudflare/cloudflared
```

### Step 2: Authenticate

```bash
cloudflared tunnel login
```

### Step 3: Create a Tunnel

```bash
cloudflared tunnel create alita-bot
```

This creates a tunnel and outputs a tunnel ID.

### Step 4: Configure DNS

```bash
cloudflared tunnel route dns alita-bot bot.yourdomain.com
```

### Step 5: Create config.yml

```yaml
tunnel: <your-tunnel-id>
credentials-file: /root/.cloudflared/<tunnel-id>.json

ingress:
  - hostname: bot.yourdomain.com
    service: http://localhost:8080
  - service: http_status:404
```

### Step 6: Run the Tunnel

```bash
# Run in foreground
cloudflared tunnel run alita-bot

# Or install as a service
sudo cloudflared service install
```

### Step 7: Configure Environment

```bash
WEBHOOK_DOMAIN=https://bot.yourdomain.com
WEBHOOK_SECRET=your-secure-random-string
USE_WEBHOOKS=true
```

## Alternative: Cloudflare Tunnel Token

For Docker deployments, you can use a tunnel token instead:

```bash
# Generate token from Cloudflare dashboard
CLOUDFLARE_TUNNEL_TOKEN=your-tunnel-token
```

Add to docker-compose.yml:

```yaml
services:
  cloudflared:
    image: cloudflare/cloudflared:latest
    restart: always
    command: tunnel --no-autoupdate run --token ${CLOUDFLARE_TUNNEL_TOKEN}
    depends_on:
      - alita
```

## Security Best Practices

### 1. Always Use WEBHOOK_SECRET

The secret prevents unauthorized requests to your webhook endpoint:

```bash
# Generate a secure random secret
openssl rand -hex 32
```

### 2. Validate Webhook Origin

Alita automatically validates that requests come from Telegram by checking:
- The URL path contains your secret
- Request headers match Telegram's format

### 3. Use HTTPS Only

Telegram requires HTTPS for webhooks. Never use HTTP in production.

### 4. Keep Your Secret Private

- Never commit `WEBHOOK_SECRET` to version control
- Use environment variables or secrets management
- Rotate the secret periodically

### 5. Monitor Webhook Health

Check the `/health` endpoint regularly:

```bash
curl https://bot.yourdomain.com/health
```

## Nginx Reverse Proxy (Alternative)

If you prefer Nginx over Cloudflare Tunnel:

```nginx
server {
    listen 443 ssl http2;
    server_name bot.yourdomain.com;

    ssl_certificate /etc/letsencrypt/live/bot.yourdomain.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/bot.yourdomain.com/privkey.pem;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

## Switching Between Modes

### From Polling to Webhook

1. Stop the bot
2. Set `USE_WEBHOOKS=true` and configure webhook variables
3. Start the bot (it will automatically register the webhook with Telegram)

### From Webhook to Polling

1. Stop the bot
2. Set `USE_WEBHOOKS=false` or remove the variable
3. Start the bot (it will automatically delete the webhook)

The bot logs confirm the switch:

```
[Polling] Removed Webhook!
[Polling] Started Polling...!
```

or

```
[HTTPServer] Unified HTTP server started on port 8080 (health, metrics, webhook)
```

## Troubleshooting

### Webhook not receiving updates

1. Verify the domain is accessible:
   ```bash
   curl -I https://your-domain.com/health
   ```

2. Check Telegram webhook status:
   ```bash
   curl "https://api.telegram.org/bot<YOUR_TOKEN>/getWebhookInfo"
   ```

3. Ensure `WEBHOOK_SECRET` matches in the URL

### 401 Unauthorized errors

- Check that `WEBHOOK_SECRET` is correctly configured
- Verify the URL path includes the secret

### Connection timeout

- Ensure port 8080 (or your `HTTP_PORT`) is accessible
- Check firewall rules
- Verify Cloudflare Tunnel or reverse proxy is running

### SSL Certificate errors

- Use a valid SSL certificate (Let's Encrypt is free)
- Cloudflare Tunnel handles SSL automatically
