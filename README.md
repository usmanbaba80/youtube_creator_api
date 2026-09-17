# YouTube Creator API

Separate Go backend that reads the **same Postgres database** filled by the YouTube scraper.
It exposes versioned, API-key-protected GET endpoints for a creator's ready videos, shorts, and playlists.

This project is intentionally **not** inside the scraper repo.

## API design choice

**Separate endpoints** (not one mega endpoint):

| Endpoint | Purpose |
|---|---|
| `GET /api/v1/creators/{creatorId}` | Creator profile |
| `GET /api/v1/creators/{creatorId}/videos` | Long-form videos |
| `GET /api/v1/creators/{creatorId}/shorts` | Shorts |
| `GET /api/v1/creators/{creatorId}/playlists` | Playlist list |
| `GET /api/v1/creators/{creatorId}/playlists/{playlistId}` | Playlist + ready items |

Why separate:
- Different pagination and payload shapes (playlists nest items)
- Clients fetch only what they need
- Easier caching and versioning later

`creatorId` is the Excel / business `creator_id`.  
If the same id exists in multiple categories, pass `?category_id=1`.

## Incomplete content

By default (`READY_ONLY=true`) APIs only return content that is ready for clients:

- Videos / shorts: `transfer_status = uploaded` and `bunny_url` present
- Playlists: `metadata_status = done` (or `fetched`); items only when playable Bunny URL exists (including reused video/short links)

Incomplete scrape/metadata/transfer rows stay in Postgres but are hidden from these responses.

## Security

- API key via `X-API-Key` or `Authorization: Bearer <key>`
- Constant-time key comparison
- Per-IP + key rate limiting
- Security headers (`nosniff`, `DENY` frame, `no-store`, etc.)
- CORS allowlist
- No stack traces in JSON errors
- `/api/v1/*` requires auth; `/healthz` is open for probes

## Versioning

All product routes live under `/api/v1/...`.  
Breaking changes go to `/api/v2/...` without removing v1.

## API documentation (PDF)

Generated docs:

- [`docs/YouTube_Creator_API_Documentation.pdf`](docs/YouTube_Creator_API_Documentation.pdf)

Regenerate:

```bat
pip install reportlab
python scripts\generate_api_docs_pdf.py
```

See [DEPLOY.md](DEPLOY.md) for running this API in Docker next to other containers
(unique compose project, network, and host port `18080`).

1. Point the scraper at Postgres (`DATABASE_URL=postgresql+psycopg2://...`) and run the pipeline so tables exist and have data.

2. Copy env:

```bat
copy .env.example .env
```

3. Edit `.env` with Postgres URL and at least one API key.

4. Install deps and run:

```bat
go mod tidy
go run ./cmd/server
```

## Example calls

```bat
curl -H "X-API-Key: change-me-long-random-key" http://localhost:8080/api/v1/creators/100

curl -H "X-API-Key: change-me-long-random-key" "http://localhost:8080/api/v1/creators/100/videos?page=1&limit=20"

# All ready videos for this creator (capped by MAX_ALL_PAGE_SIZE)
curl -H "X-API-Key: change-me-long-random-key" "http://localhost:8080/api/v1/creators/100/videos?limit=all"
curl -H "X-API-Key: change-me-long-random-key" "http://localhost:8080/api/v1/creators/100/shorts?all=true"

curl -H "X-API-Key: change-me-long-random-key" "http://localhost:8080/api/v1/creators/100/shorts?category_id=1"

curl -H "X-API-Key: change-me-long-random-key" http://localhost:8080/api/v1/creators/100/playlists

curl -H "X-API-Key: change-me-long-random-key" http://localhost:8080/api/v1/creators/100/playlists/P110001
```

## Response shape

```json
{
  "success": true,
  "data": [],
  "meta": {
    "page": 1,
    "limit": 20,
    "total": 0,
    "total_pages": 0
  }
}
```

### Pagination

| Query | Meaning |
|---|---|
| `page=1&limit=20` | Normal paging (default `limit=20`, max `MAX_PAGE_SIZE=100`) |
| `limit=all` | Return all matching rows in one response |
| `all=true` | Same as `limit=all` |

`limit=all` is capped by `MAX_ALL_PAGE_SIZE` (default 5000). If the result set is larger, the API returns `400 too_many_results` and you should page instead.

Errors:

```json
{
  "success": false,
  "error": {
    "code": "unauthorized",
    "message": "invalid or missing API key"
  }
}
```
