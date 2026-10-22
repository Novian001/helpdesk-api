# Helpdesk / Incident Management API

A production-style REST API for managing internal tickets and incidents, written
in Go with Fiber, PostgreSQL and pgx. There is no ORM: every query is SQL you
can read, and every write is a transaction you can follow.

## 1. Project overview

The API lets employees open tickets, lets agents work on them, and lets admins
manage users and tickets. Every important change to a ticket is written to an
audit trail (`ticket_histories`) inside the same transaction as the change
itself.

Roles:

| Role   | Can do |
|--------|--------|
| ADMIN  | everything: manage users, manage any ticket, assign, delete, view all |
| AGENT  | view all tickets, update them, assign them, comment |
| USER   | create tickets, view and comment on their own tickets, close/reopen them |

## 2. Architecture

```
HTTP request
   │
   ▼
httpapi/     handlers + middleware (Fiber)  → JSON only, no business rules
   │
   ▼
service/     business rules, validation, RBAC decisions
   │
   ▼
store/       SQL over pgx (pgxpool)          → transactions, no HTTP types
   │
   ▼
PostgreSQL
```

- `internal/apperr` holds the single error type. Every failure becomes
  `{"error":{"code":"...","message":"...","details":{...}}}`, so the HTTP status
  and the machine readable code never drift apart.
- Dependencies are injected through constructors (`NewTicketService(tickets,
  users)`). No service framework, no container, no interface that only has one
  implementation.

## 3. Folder structure

```
main.go                     entrypoint: config, wiring, graceful shutdown
internal/
  apperr/                   typed application errors + error envelope
  auth/                     Argon2id hashing, JWT issue/parse
  config/                   environment configuration
  db/                       pgx pool, versioned migration runner
    migrations/             001..005 SQL files, embedded in the binary
  httpapi/                  Fiber app, middleware, handlers, Swagger UI
    docs/                   swagger.json + Swagger UI page
  models/                   domain structs and enums
  service/                  business rules (auth, ticket, comment, validation)
  store/                    SQL data access (users, tickets, comments, tokens)
  integration/              integration tests against a real PostgreSQL
```

## 4. Database schema

| Table | Purpose | Key constraints |
|-------|---------|-----------------|
| `users` | accounts | `email` unique (case-insensitive, `citext`), `role` CHECK in ADMIN/AGENT/USER |
| `tickets` | incidents | FK `created_by` → users, FK `assigned_to` → users, CHECK on `priority` and `status` |
| `comments` | discussion per ticket | FK `ticket_id` → tickets (cascade), FK `user_id` → users |
| `ticket_histories` | audit trail | FK `ticket_id` → tickets (cascade), FK `actor_id` → users (set null) |
| `refresh_tokens` | refresh token hashes | unique `token_hash`, FK `user_id` → users |
| `schema_migrations` | applied migration versions | created by the runner |

Indexes exist only for real query paths: ticket list ordering
(`created_at DESC`), the common filters (`status`, `priority`), ownership
(`created_by`), assignment (`assigned_to`), a GIN full text index over
`title + description` for `?search=`, comment and history lookups by
`ticket_id`, and the refresh token hash.

History actions: `TICKET_CREATED`, `STATUS_CHANGED`, `RESOLVED`, `CLOSED`,
`PRIORITY_CHANGED`, `TITLE_CHANGED`, `ASSIGNED`.

Migrations are plain SQL files numbered `001_...` to `005_...`, embedded with
`//go:embed` and applied by `db.Migrate` on startup. Each file runs in its own
transaction and is recorded in `schema_migrations`, so a restart never applies
the same file twice.

## 5. Authentication flow

1. `POST /api/v1/auth/register` — email and password are validated, the
   password is hashed with Argon2id (never stored in plaintext), a user row is
   created and a token pair is returned. The first account on an empty database
   becomes ADMIN so a fresh install is usable; the admin can then change roles
   with `PATCH /api/v1/users/:id/role`.
2. `POST /api/v1/auth/login` — the password is verified and a token pair is
   returned.
3. Tokens: the **access token** is a short lived JWT (default 15m) carrying
   `sub`, `role` and `name`; the **refresh token** is a long lived JWT
   (default 168h). Only its SHA-256 hash is stored in `refresh_tokens`.
4. `POST /api/v1/auth/refresh` — validates the refresh token, revokes the old
   one and issues a new pair. A revoked token cannot be reused.
5. `POST /api/v1/auth/logout` — revokes the refresh token.
6. `GET /api/v1/me` — returns the profile of the caller.

Protected routes use the `Authorization: Bearer <access token>` header. The
`authRequired` middleware parses the token, rejects anything invalid or
expired, and puts the user id and role into the request context. The
`requireRole` middleware then gates staff-only routes.

## 6. Authorization / RBAC

Authorization is enforced in the backend at two levels:

- Route level: `requireRole(models.RoleAdmin)` for `DELETE /tickets/:id` and
  the user management routes, `requireRole(models.RoleAdmin, models.RoleAgent)`
  for assignment.
- Service level: every service method re-checks the actor. A USER only sees
  their own tickets — the restriction is pushed into SQL (`created_by = $1`),
  not filtered afterwards. A USER cannot change priority, title, category or
  description, cannot assign, and cannot delete. Comments can only be deleted
  by their author or by an ADMIN.

The 403 response is a normal API answer, not a hidden button.

## 7. API endpoints

Base path `/api/v1`. Every response body is JSON.

| Method | Path | Auth |
|--------|------|------|
| POST | `/auth/register` | public |
| POST | `/auth/login` | public |
| POST | `/auth/refresh` | public |
| POST | `/auth/logout` | public |
| GET | `/me` | bearer |
| GET | `/users` | ADMIN |
| PATCH | `/users/:id/role` | ADMIN |
| POST | `/tickets` | bearer |
| GET | `/tickets` | bearer |
| GET | `/tickets/:id` | bearer |
| PATCH | `/tickets/:id` | bearer |
| DELETE | `/tickets/:id` | ADMIN |
| POST | `/tickets/:id/assign` | ADMIN, AGENT |
| GET | `/tickets/:id/history` | bearer |
| POST | `/tickets/:id/comments` | bearer |
| GET | `/tickets/:id/comments` | bearer |
| DELETE | `/comments/:id` | bearer (own) or ADMIN |

List query parameters: `page` (default 1), `limit` (default 20, max 100),
`status`, `priority`, `search`. The list response is:

```json
{"data": [], "pagination": {"page": 1, "limit": 20, "total": 100, "total_pages": 5}}
```

Errors use one envelope for every status code (400, 401, 403, 404, 409, 422,
500):

```json
{"error": {"code": "TICKET_NOT_FOUND", "message": "ticket not found"}}
```

Stack traces and internal details are logged, never returned.

## 8. Local development

```bash
# Postgres locally
docker run -d --name helpdesk-pg -e POSTGRES_USER=helpdesk \
  -e POSTGRES_PASSWORD=helpdesk -e POSTGRES_DB=helpdesk -p 5432:5432 postgres:16-alpine

cp .env.example .env       # then edit DATABASE_URL and JWT_SECRET
export PATH=/usr/local/go/bin:$PATH
go mod download
go run .
```

The API listens on `SERVER_PORT` (default 8080) and applies pending migrations
at startup.

## 9. Docker

```bash
docker compose up --build
```

`docker-compose.yml` starts two services: `postgres` (16-alpine, with a health
check) and `api` (multi-stage build, non-root user, static binary). The API
waits for the database to become healthy, then migrates and serves on
<http://localhost:8080>. `docker compose down -v` removes the volume too.

## 10. Testing

```bash
# unit tests only (no database needed)
go test ./...

# integration tests against a real PostgreSQL
createdb helpdesk_test
TEST_DATABASE_URL='postgres://helpdesk:helpdesk@localhost:5432/helpdesk_test?sslmode=disable' go test ./...
```

Unit tests cover password hashing, JWT issue/parse/expiry, config parsing,
authorization rules and pagination arithmetic. Integration tests
(`internal/integration`) run the real Fiber app against a real PostgreSQL
database: register, login, `/me`, refresh rotation, logout, ticket CRUD,
pagination, filter, search, assignment, comments, history, validation errors,
404 handling and the RBAC rules. The test database is separate from the
development database.

## 11. Environment variables

| Variable | Required | Default | Meaning |
|----------|----------|---------|---------|
| `DATABASE_URL` | yes | – | PostgreSQL connection string |
| `JWT_SECRET` | yes | – | HMAC key, at least 32 characters |
| `JWT_ACCESS_TTL` | no | `15m` | access token lifetime |
| `JWT_REFRESH_TTL` | no | `168h` | refresh token lifetime |
| `SERVER_PORT` | no | `8080` | listen port |
| `APP_ENV` | no | `development` | `development` enables debug logs |

`.env` is git-ignored; `.env.example` documents the variables. No secret is
hardcoded in the source or in the compose file (the compose file uses a
development default that must be replaced in production).

## 12. Swagger

The OpenAPI 3.0 document is embedded in the binary and served by the API:

- UI: <http://localhost:8080/swagger> (redirects to `/swagger/index.html`)
- raw document: <http://localhost:8080/swagger/swagger.json>

In the UI, click **Authorize**, paste the `access_token` returned by
`POST /api/v1/auth/login`, and the protected endpoints become callable.

## 13. Design decisions

- **No ORM.** pgx with explicit SQL keeps the data access visible and the
  queries explainable in an interview. The stores are three small structs
  (`UserStore`, `TicketStore`, `CommentStore`, `RefreshTokenStore`), not a
  generic repository.
- **Argon2id over bcrypt.** Argon2id is the current recommendation; bcrypt was
  the alternative. Both are acceptable, Argon2id wins on memory hardness.
  `internal/auth/password.go` uses the standard library only.
- **Refresh tokens are stored hashed.** A database leak must not hand out
  usable sessions.
- **One error type.** `apperr.Error` carries status, code, message and field
  details. Handlers never build error JSON by hand.
- **Migrations embedded in the binary.** One container, one command, no
  separate migration step to forget.
- **First user becomes ADMIN.** Avoids a seed script for local use; the rule is
  documented here and in the code.
- **Transactions for state plus history.** `TicketStore.Update` writes the
  ticket and its audit rows in one transaction, so the trail can never drift.

## 14. Trade-offs

- Tickets are hard deleted for admins; there is no soft delete or archive.
- Search uses PostgreSQL full text with the `simple` configuration, so it does
  not stem or rank languages.
- No rate limiting, no captcha, no email verification.
- Roles are fixed strings in the database; a permissions table would be needed
  for fine grained policies.
- The list endpoint always orders by `created_at DESC`; there is no sort
  parameter.

## 15. What would be improved in production

- Refresh token rotation with reuse detection (revoke the whole family when a
  revoked token is presented).
- Structured logs shipped to a collector with metrics and traces
  (OpenTelemetry).
- Rate limiting and brute force lockout on the login endpoint.
- Email verification and password reset flows.
- Cursor based pagination for very large tables, plus a covering index tuned
  to the real query plans.
- Read replica for reporting, and a background worker for notifications.
- CI running `go vet`, `golangci-lint` and the integration suite against a
  throwaway PostgreSQL service.
