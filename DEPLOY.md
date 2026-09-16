# Deploy on Contabo (Docker)

This API is meant to run **next to** your other containers. It uses its own
Compose project name (`youtube-creator-api`), its own network, and a dedicated
host port (`18080` by default) so it will not replace or collide with other stacks.

## 1. Prerequisites on Contabo

SSH in:

```bash
ssh root@YOUR_CONTABO_IP
```

Install Docker + Compose plugin if not already present:

```bash
# Ubuntu example
curl -fsSL https://get.docker.com | sh
systemctl enable --now docker
docker compose version
```

Confirm other projects are fine:

```bash
docker ps
```

## 2. Upload the project

From your PC (PowerShell):

```powershell
scp -r "C:\Users\UsmanShabbir\OneDrive - Veroke\Desktop\YouTube_Creator_API" root@YOUR_CONTABO_IP:/opt/youtube-creator-api
```

Or clone with git if the repo is remote.

On the server:

```bash
cd /opt/youtube-creator-api
```

## 3. Create `.env` on the server

```bash
cp .env.example .env
nano .env
```

Example for Postgres **on the same Contabo host** (not inside this compose file):

```env
HTTP_ADDR=:8080

# host.docker.internal = Contabo host from inside the container
# Encode @ in password as %40  (e.g. abc@2026 -> abc%402026)
DATABASE_URL=postgres://yt_user:abc%402026@host.docker.internal:5432/yt_videos?sslmode=disable

API_KEYS=generate-a-long-random-secret-here

# Your frontend origin(s), comma-separated
CORS_ALLOWED_ORIGINS=https://your-frontend.example.com

RATE_LIMIT_RPM=120
DEFAULT_PAGE_SIZE=20
MAX_PAGE_SIZE=100
MAX_ALL_PAGE_SIZE=5000
READY_ONLY=true
```

### Postgres connection options

| Postgres location | `DATABASE_URL` host |
|---|---|
| Installed on Contabo host (apt / native) | `host.docker.internal` |
| Another Docker container on a **shared** network | that container name, e.g. `postgres` |
| Remote VPS (e.g. `94.72.118.33`) | that IP / hostname |

If Postgres is in another compose stack and you want to use its network:

```bash
docker network ls
# then in docker-compose.yml under api:
#   networks:
#     - youtube_creator_net
#     - other_stack_default
# and declare:
# networks:
#   other_stack_default:
#     external: true
```

Also ensure Postgres `pg_hba.conf` allows the Docker bridge / container IPs (or `host.docker.internal` path via host).

## 4. Pick a free host port

```bash
ss -tlnp | grep -E '18080|8080|3000'
```

If `18080` is free, keep the default in `docker-compose.yml`:

```yaml
ports:
  - "18080:8080"
```

If taken, change only the **left** side, e.g. `"18081:8080"`.

## 5. Build and start (does not stop other containers)

```bash
cd /opt/youtube-creator-api
docker compose up -d --build
docker compose ps
docker compose logs -f --tail=100
```

Health check:

```bash
curl -s http://127.0.0.1:18080/healthz
# expect: ok

curl -s -H "X-API-Key: YOUR_KEY" http://127.0.0.1:18080/api/v1/health
```

## 6. Expose publicly (recommended: reverse proxy)

Do **not** open raw Docker ports on the public firewall if you already use Nginx/Caddy for other apps. Add a vhost instead.

### Nginx example

```nginx
server {
    listen 80;
    server_name api-creators.yourdomain.com;

    location / {
        proxy_pass http://127.0.0.1:18080;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

Then TLS with certbot as you do for other sites.

### Firewall

Only needed if you expose the port directly:

```bash
ufw allow 18080/tcp   # optional; prefer nginx :80/:443 only
```

## 7. Useful commands

```bash
# Restart only this app
docker compose restart

# Rebuild after code changes
docker compose up -d --build

# Stop only this stack (other projects stay up)
docker compose down

# Logs
docker compose logs -f api
```

## 8. Safety checklist (shared Contabo host)

- Compose `name:` is `youtube-creator-api` — unique project namespace
- Container name `youtube-creator-api` — unique
- Network `youtube_creator_net` — private to this app
- Host port `18080` — choose unused
- This compose does **not** start Postgres / Redis / Nginx — uses your existing ones
- Never commit `.env` or real `API_KEYS`

## Quick smoke test from outside

```bash
curl -H "X-API-Key: YOUR_KEY" https://api-creators.yourdomain.com/api/v1/creators/100/videos?limit=5
```
