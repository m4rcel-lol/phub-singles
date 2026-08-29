# pornhub.singles

A single-user bio-link page — profile header, an ordered list of links, first-party
click counters — plus an admin panel to manage all of it. Dark, restrained, and fast:
the whole front end is ~80 kB over the wire.

> **This is a parody project.** It is not affiliated with, endorsed by or connected to
> Pornhub, Aylo or any of their brands, and it hosts no adult media. See `/notice`.

## Stack

| Layer      | Choice                                          | Why |
|------------|-------------------------------------------------|-----|
| Frontend   | Angular 22, standalone components, zoneless     | Signals-only change detection, OnPush everywhere, lazy admin routes |
| Backend    | Go 1.26, `net/http` only                        | Go 1.22+ `ServeMux` covers method+pattern routing, so no router dependency |
| Database   | SQLite via `modernc.org/sqlite`                 | Pure Go, no cgo, one file on one volume |
| Ingress    | Caddy                                           | TLS and `reverse_proxy`, nothing else |
| Runtime    | Distroless, non-root, static binary             | ~20 MB image, no shell, no package manager |

Only two Go dependencies are used: the SQLite driver and `golang.org/x/crypto` for bcrypt.
The front end has no runtime dependencies beyond Angular itself.

## Quick start

```bash
cp .env.example .env
# edit .env: set PHS_ADMIN_PASSWORD, and SITE_ADDRESS if you have a domain
docker compose up --build -d
```

Then open <https://localhost> (Caddy issues a local certificate for `localhost`, so your
browser will warn once) and sign in at <https://localhost/admin>.

Seed data ships with the schema, so the site looks finished on first boot: a profile and
six example links at `/creator`.

| Path        | What it is |
|-------------|------------|
| `/`         | Landing page (with the sign-in / dashboard buttons) |
| `/<handle>` | The public bio page (`/creator` after a fresh install; the handle is editable) |
| `/admin`    | Admin panel — links, profile, stats, password |
| `/notice`   | Parody notice |
| `/privacy`  | Privacy policy |
| `/terms`    | Terms of service |

Stop and remove everything (the named volumes keep the database and uploads):

```bash
docker compose down
```

## Accounts, roles and badges

Every account has a role, and the role decides what it can do and which badge
its page carries.

| Role | Can do | Badge |
|------|--------|-------|
| `owner` | everything an admin can, plus **Site settings** and granting/revoking admin | **Owner** |
| `admin` | manage the page, links and stats; add members; verify, reset and delete accounts *below* their own rank | **Administrator** |
| `member` | nothing — a demoted account keeps its data and badges but cannot sign in | — |

Permission is decided by rank — **owner > admin > member — and you may only act
on an account strictly below you**. An administrator therefore cannot touch
another administrator, cannot touch the owner, and cannot touch their own
account; the owner can act on everyone. The server returns those permissions per
row (`canManage`, `canChangeRole`) and the panel only mirrors them, so no button
ever leads to a 403.

Ownership itself is the one thing no session can change: `set-owner` exists only
on the command line, so even a stolen owner session cannot hand the site away.

Everything except ownership transfer can be done from the **People** tab in the
panel. The CLI is the same set of operations, for when nobody can sign in:

```bash
docker compose exec app phs-server user list
docker compose exec -e PHS_NEW_PASSWORD=secret-password app phs-server user create bob
docker compose exec app phs-server user grant-admin bob     # → Administrator badge
docker compose exec app phs-server user revoke-admin bob    # → demoted to member
docker compose exec app phs-server user set-owner bob       # → Owner badge + the page (CLI only)
docker compose exec app phs-server user passwd bob
docker compose exec app phs-server user verify bob
docker compose exec app phs-server user delete bob
```

`phs-server user help` lists everything. Prefer `-e PHS_NEW_PASSWORD=…` over
`--password=…`: the flag is visible in the container's process list.

### The Verified badge

Verified appears when **either** an administrator grants it (Accounts tab in the
panel, or `phs-server user verify`) **or** the page passes
**10,000 views** — at which point it unlocks on its own and needs no grant. The
Accounts tab shows the progress bar towards that threshold.

## The owner panel

The **Site** tab appears only for the owner. It holds the settings that change
what every visitor sees, and that an operator should be able to change without
editing `.env` and redeploying:

| Setting | Effect |
|---------|--------|
| Headline, intro text | The copy at the top of `/` |
| Verified threshold | Page views that unlock the Verified badge automatically |
| Maintenance mode + notice | Visitors get the notice and a `503`; signed-in administrators still see the real site, so you can check your work while it is dark |
| Allow search engines | Off writes a full `Disallow` into `/robots.txt`, which the server generates from this setting |

Deployment concerns — ports, TLS, database path, session lifetime — stay in
`.env`, because changing them means restarting the container anyway.

## Honest view counts

A counter that anyone can inflate is not worth showing, so a beacon has to pass
three checks before it moves a number:

1. **It has to come from a page on this site.** `Sec-Fetch-Site` (or, on older
   browsers, `Origin`/`Referer`) must say same-origin. A bare `POST /api/views`
   from a shell is ignored.
2. **It has to look like a browser.** Empty user agents and the obvious bots,
   crawlers, previewers and HTTP libraries are excluded.
3. **The same visitor is counted once per window.** A keyed hash (HMAC-SHA-256,
   truncated to 128 bits) of the client address and user agent, under a secret
   generated on first start and kept only in the database, is stored with an
   expiry: 12 hours for a view (`PHS_VIEW_WINDOW`), 15 minutes per link for a
   click. Reloading the page in between changes nothing.

Filtered beacons still answer `204`, so a client cannot probe its way around the
rules. The fingerprint cannot be reversed into an address, is never stored next
to the counters, and is deleted when it expires — the privacy policy on the site
says exactly this.

Genuinely determined farming would need a rotating address pool and a real
browser; casual self-farming by holding down F5 does nothing.

## Configuration

Everything is read from the environment; nothing is baked into the image. `.env.example`
documents the full set. The variables that matter:

| Variable | Default | Notes |
|----------|---------|-------|
| `SITE_ADDRESS` | `localhost` | Caddy site address. Use your domain in production. |
| `ACME_EMAIL` | — | Contact address for Let's Encrypt. Must not be empty. |
| `PHS_ADMIN_USERNAME` | `admin` | Bootstrap admin, created on first start. |
| `PHS_ADMIN_PASSWORD` | — | Required on first start. Minimum 8 characters, bcrypt-hashed before storage. |
| `PHS_ADMIN_PASSWORD_RESET` | `false` | Set `true` for a single start to reset a forgotten password, then set it back. |
| `PHS_PUBLIC_URL` | `http://localhost:8080` | Canonical URL; used for logging and the same-origin check. |
| `PHS_SESSION_TTL` | `24h` | Admin session lifetime. |
| `PHS_VIEW_WINDOW` | `12h` | How long one visitor's page view is de-duplicated. |
| `PHS_SECURE_COOKIE` | `true` | Only set `false` when serving plain HTTP locally. |
| `PHS_MAX_UPLOAD_BYTES` | `2097152` | Avatar upload limit. |
| `PHS_CORS_ORIGINS` | — | Comma-separated extra origins. Empty means same-origin only, which is correct behind Caddy. |
| `PHS_LOG_LEVEL` / `PHS_LOG_FORMAT` | `info` / `json` | Structured logging via `log/slog`. |

The admin password is only read on first start (or when `PHS_ADMIN_PASSWORD_RESET=true`);
after that, change it from the **Account** tab, which also revokes every open session.

## Local development

Two terminals, no Docker:

```bash
# 1) API on :8080, with a local database and uploads directory
cd backend
PHS_ADMIN_PASSWORD=devpassword PHS_SECURE_COOKIE=false go run ./cmd/server
```

```bash
# 2) Angular dev server on :4200, proxying /api and /uploads to :8080
cd frontend
npm install
npm start
```

Open <http://localhost:4200>. `proxy.conf.json` forwards the API calls, so cookies stay
same-origin.

To run the production build the way the container does:

```bash
cd frontend && npm run build
cp -R dist/frontend/browser/. ../backend/internal/web/dist/
cd ../backend && go run ./cmd/server
```

`backend/internal/web/dist` is embedded with `go:embed`; the checked-in `index.html`
placeholder only exists so `go build` works in a fresh checkout.

### Tests

```bash
cd backend && go test ./...
```

The suite covers the public payload shape, the counters, authentication, the link
lifecycle including reordering, the SPA fallback, security headers, cross-origin
rejection, handle resolution and handle validation.

## API

Public — no authentication, rate limited per IP:

| Method | Path | Purpose |
|--------|------|---------|
| `GET` | `/api/page[?handle=x]` | Site copy, profile, badges and enabled links. With `handle`, a mismatch is a 404; during maintenance, a `503` for everyone but administrators. |
| `POST` | `/api/views` | Increment the page-view counter. |
| `POST` | `/api/links/{id}/click` | Increment a link's click counter. |
| `GET` | `/api/session` | Who the caller is; always 200, so anonymous visits do not 401. |
| `GET` | `/api/health` | Liveness. |
| `GET` | `/api/ready` | Readiness (pings the database). |

Admin — requires the session cookie and a same-origin request:

| Method | Path | Purpose |
|--------|------|---------|
| `POST` | `/api/admin/login` | Sign in; sets `phs_session`. |
| `POST` | `/api/admin/logout` | Revoke the session. |
| `POST` | `/api/admin/password` | Change password; revokes all sessions. |
| `GET`/`PUT` | `/api/admin/profile` | Read/update handle, name, tagline, bio. |
| `POST`/`DELETE` | `/api/admin/profile/avatar` | Upload/remove the avatar. |
| `GET`/`POST` | `/api/admin/links` | List/create links. |
| `PUT`/`DELETE` | `/api/admin/links/{id}` | Update/delete a link. |
| `PUT` | `/api/admin/links/order` | Persist a new order (full id list). |
| `GET` | `/api/admin/stats[?days=14]` | Totals, per-link clicks, daily activity. |
| `GET` | `/api/admin/users` | Accounts with roles, badges and the caller's permissions over each. |
| `POST` | `/api/admin/users` | Create an account (admins may create members only). |
| `DELETE` | `/api/admin/users/{username}` | Delete an account below your rank. |
| `PUT` | `/api/admin/users/{username}/verified` | Grant/revoke Verified. |
| `POST` | `/api/admin/users/{username}/password` | Reset someone's password; revokes their sessions. |
| `PUT` | `/api/admin/users/{username}/role` | **Owner only.** Promote/demote. |
| `GET`/`PUT` | `/api/admin/settings` | **Owner only.** Site settings. |

Errors are always `{"error": "code", "message": "...", "fields": {...}}`.

## Architecture note

```
        browser
           │  https
    ┌──────▼──────┐   proxy only (no file_server, no try_files)
    │    caddy    │──────────────┐
    └─────────────┘              │
                          ┌──────▼───────────────────────────┐
                          │ app (single static Go binary)    │
                          │  • /api/*     JSON API           │
                          │  • /uploads/* avatar files       │
                          │  • /*         embedded Angular   │
                          └──────┬───────────────────────────┘
                                 │
                          ┌──────▼──────┐
                          │  /data      │  SQLite (WAL) + uploads
                          └─────────────┘
```

**One binary serves everything.** The Angular production build is embedded with
`go:embed`, so there is no static-file service to deploy or keep in sync, and the
Caddyfile stays a pure `reverse_proxy`. Unknown non-API paths fall back to `index.html`
so client-side routes survive a hard refresh; unknown `/api/*` paths return JSON 404s,
never the app shell.

**State is one directory.** `/data` holds `phs.db` and `uploads/`. Backing the site up is
copying that volume. Migrations are embedded `.sql` files applied in filename order and
recorded in `schema_migrations`, so adding `0004_*.sql` is the whole process for a schema
change. Seed statements are idempotent and never overwrite edits made in the admin panel.

**Sessions, not JWTs.** Signing in stores a random 32-byte token, SHA-256 hashed, in the
`sessions` table and returns it in an HttpOnly, `SameSite=Strict`, `Secure` cookie scoped
to `/api`. That makes logout and "revoke everything on password change" real operations
instead of a wait for expiry. Cross-origin state changes are rejected by an Origin check,
which together with `SameSite=Strict` removes the need for CSRF tokens.

**Defence in depth in the app, not only at the edge.** Security headers, a strict CSP,
gzip and rate limiting all live in Go, so they hold when the binary is run without Caddy
in front of it. Caddy adds TLS and HSTS. Public write endpoints use in-memory token
buckets (per IP); login is limited far harder than the analytics endpoints.

**Analytics are two integers, and they are defended.** A global counter pair plus
a per-day rollup and a per-link counter — with the three-stage filter described
above in front of them, so the numbers survive contact with the person they
flatter. The only visitor-derived value stored anywhere is a short-lived,
irreversible fingerprint whose sole purpose is de-duplication.

**Privilege is one comparison.** Every permission check reduces to "may only act
on an account of strictly lower rank" — `mayManage` in
`internal/httpx/users.go`, on top of `store.Rank`. Ownership transfer is not
exposed over HTTP at all, so a compromised admin session cannot escalate to it.

Known trade-off: there is no server-side rendering, so crawlers that do not execute
JavaScript see the static Open Graph tags from `index.html` rather than per-profile
metadata. For a single-profile site that is a fair trade for the simplicity.

## Troubleshooting

**"It keeps sending me back to /admin/login."**

If you are signed out, that is the guard doing its job — `/admin` is protected,
and the page you asked for is remembered in `?next=` so signing in returns you
to it.

If it happens *after* a successful sign-in, the browser is throwing the session
cookie away. Almost always that is `PHS_SECURE_COOKIE=true` (the default) on a
site served over plain HTTP: a `Secure` cookie is only kept on HTTPS or on
`localhost`. The server logs a warning the moment it happens —
`issuing a Secure session cookie over an insecure request` — so check
`docker compose logs app`. Either serve the site over HTTPS (which the bundled
Caddy does for you) or set `PHS_SECURE_COOKIE=false` for a local HTTP run.

Two other causes worth ruling out: the account was demoted with `revoke-admin`
(login answers `403 not_an_admin`), or the password was changed, which revokes
every existing session on purpose.

## Layout

```
backend/
  cmd/server/           entrypoint, graceful shutdown, -healthcheck flag
  internal/config/      environment parsing and validation
  internal/httpx/       routing, middleware, handlers, uploads, SPA, gzip, ranks
  cmd/server/cli.go     account management (docker compose exec app phs-server user …)
  internal/logging/     slog setup
  internal/ratelimit/   token bucket
  internal/store/       SQLite access, site settings, badges, embedded migrations
  internal/web/dist/    embedded Angular build (placeholder in git)
frontend/
  src/app/core/         API client, auth, guards, models
  src/app/features/     landing, home (/<handle>), legal pages, admin + owner panels
  src/app/shared/       wordmark, public footer, badges
  public/               favicon, OG image, manifest, robots.txt
caddy/Caddyfile         reverse_proxy + TLS + security headers
Dockerfile              node build → go build → distroless runtime
docker-compose.yml      app + caddy, volumes, healthchecks
```

## Operations

**Backups.** `docker run --rm -v pornhub-singles_app-data:/data -v "$PWD:/backup" alpine
tar czf /backup/phs-backup.tgz -C /data .`

**Logs.** `docker compose logs -f app` — one structured JSON line per request, with a
request id echoed in the `X-Request-Id` response header.

**Health.** The app image healthchecks itself (`phs-server -healthcheck` hits
`/api/ready`); Caddy waits for it before starting.

**Shutdown.** `SIGTERM` drains in-flight requests within `PHS_SHUTDOWN_TIMEOUT`, then the
database is closed cleanly.

## Before you go live

- Set a real `PHS_ADMIN_PASSWORD` and a real `SITE_ADDRESS` / `ACME_EMAIL`.
- Change the handle, profile and links in the admin panel, and delete the sample links.
- Decide who is the owner (`phs-server user set-owner`) before handing out admin accounts.
- Look through the **Site** tab: headline, verification threshold and search-engine visibility all start at their defaults.
- Review `/privacy` and `/terms` — the text describes what this code actually does, but
  it is not legal advice, and the contact address in it (`hello@pornhub.singles`) needs
  to become one you read. Both live in `frontend/src/app/features/legal/`.
