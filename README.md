# InkPress

A minimalist markdown publication platform and CMS built with Go, HTML/CSS/JS, and MariaDB.

## Features

- **Markdown publishing** — write posts in Markdown, rendered to clean HTML
- **Invite-only registration** — the first user to register becomes admin; subsequent users join via invitation links
- **Draft, publish, and schedule posts** — scheduled posts auto-publish at their set time
- **Tags** — organize posts with comma-separated tags
- **Comments** — visitors can leave comments; admins moderate (approve, mark spam, delete)
- **File uploads** — upload images (JPG, PNG, GIF, WebP, SVG) and PDFs up to 10 MB
- **RSS feed** — automatic RSS at `/rss.xml`
- **Ghost Source-inspired design** — typography-focused, generous whitespace, neutral zinc palette

## Tech Stack

- **Backend:** Go (Golang) with gorilla/mux
- **Database:** MariaDB 11
- **Frontend:** Server-rendered HTML templates, plain CSS and JS
- **Markdown:** gomarkdown/markdown

## Quick Start (Docker / Coolify)

1. Deploy this repo via Coolify (or any Docker host).
2. Set the following environment variables:
   - `SECRET_KEY` — a long random string for session security
   - `BASE_URL` — your site's public URL (e.g. `https://blog.example.com`)
   - `DB_PASS` — MariaDB password (change from default)
   - `DB_ROOT_PASS` — MariaDB root password
3. The first visit to `/admin/register?invite=<code>` won't work until you create an invitation. Since the first user must register via invitation but there are no users yet, the first registration is special: visit `/admin/register` without an invite parameter and the system will allow the first user to register as admin directly.

   **Actually:** The first user is created without needing an invitation. Visit `/admin/login`, and if no users exist, you'll see a "Create Account" link. The first account is automatically assigned the `admin` role.

4. Subsequent users join via invitation links generated from the admin panel.

## Environment Variables

| Variable | Default | Description |
|---|---|---|
| `DB_HOST` | `127.0.0.1` | MariaDB host |
| `DB_PORT` | `3306` | MariaDB port |
| `DB_USER` | `inkpress` | MariaDB user |
| `DB_PASS` | `inkpress` | MariaDB password |
| `DB_NAME` | `inkpress` | MariaDB database name |
| `DB_ROOT_PASS` | `inkpress_root` | MariaDB root password (docker-compose only) |
| `SECRET_KEY` | (change me) | Session secret key |
| `PORT` | `8080` | Server port |
| `BASE_URL` | `http://localhost:8080` | Public site URL |
| `UPLOADS_DIR` | `web/static/uploads` | Upload directory path |

## Local Development

```bash
go mod download
go run ./cmd/inkpress
```

Requires a MariaDB instance accessible at the configured `DB_HOST:DB_PORT`.

## Coolify Deployment

This project includes a `docker-compose.yaml` with two services:
- `mariadb` — database with persistent volume
- `app` — the Go application, exposed on port 8080

Coolify will build and deploy both services. The app uses `expose` (not `ports`) so Coolify can manage the network proxy.

## License

MIT
