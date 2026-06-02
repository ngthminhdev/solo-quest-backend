# SoloQuest Backend

Golang backend for SoloQuest application using Gin framework.

## Project Structure

```
solo_quest_backend/
├── cmd/
│   ├── api/main.go                    # Application entry point
│   └── migrate/main.go               # Database migration CLI tool
├── pkg/
│   └── logger/
│       ├── logger.go                  # Core Zap logger with rotating file
│       ├── context.go                 # Context-aware logging helpers
│       └── trace.go                   # Operation trace with slow-op detection
├── internal/
│   ├── config/config.go               # Environment configuration
│   ├── database/
│   │   ├── database.go                # PostgreSQL connection (GORM)
│   │   ├── gorm_logger.go            # GORM -> Zap logger adapter
│   │   ├── migrate.go                # DeprecatedAutoMigrate (not used in startup)
│   │   ├── migration_runner.go       # SQL migration runner (golang-migrate)
│   │   └── seed.go                   # Dev user ID helper
│   ├── dto/
│   │   ├── progress_dto.go           # Progress response DTOs
│   │   ├── log_dto.go               # Log response DTOs
│   │   ├── daily_checkin_dto.go     # Daily check-in request/response DTOs
│   │   ├── daily_review_dto.go      # Daily review request/response DTOs
│   │   └── reward_dto.go           # Reward list, claim, redemption DTOs
│   ├── handlers/
│   │   ├── health.go                 # Health check handlers
│   │   ├── auth.go                   # Auth handlers (dev-login)
│   │   ├── api.go                    # API handlers (users, quests, settings)
│   │   ├── onboarding.go            # Onboarding handlers
│   │   ├── quest_action.go          # Quest action handlers
│   │   ├── progress_handler.go      # Progress handler (progress, weekly-chart, xp-history)
│   │   ├── log_handler.go          # Log handler with filtering
│   │   ├── daily_checkin_handler.go # Daily check-in handler
│   │   ├── daily_review_handler.go # Daily review handler
│   │   └── reward_handler.go       # Reward handler (list, claim, redemptions)
│   ├── middleware/
│   │   ├── cors.go                    # CORS middleware
│   │   ├── request_logger.go          # Request logging middleware
│   │   ├── recovery.go               # Panic recovery middleware
│   │   ├── auth.go                   # JWT auth skeleton
│   │   └── user_context.go           # Dev user context middleware
│   ├── models/                        # Core data models
│   ├── observability/
│   │   └── http_logging.go           # HTTP field builders, sanitization
│   ├── routes/
│   │   └── routes.go                 # Route definitions
│   ├── services/
│   │   ├── user_service.go           # User service
│   │   ├── quest_service.go          # Quest service
│   │   ├── reward_service.go         # Reward service (list, claim, redemptions)
│   │   ├── log_service.go            # Log service with filtering + pagination
│   │   ├── settings_service.go       # Settings service
│   │   ├── bootstrap_service.go      # Default dev user bootstrap
│   │   ├── onboarding_service.go     # Onboarding service
│   │   ├── quest_action_service.go   # Quest action service (start/complete/skip/snooze)
│   │   ├── progress_service.go       # Progress service (computed progress, weekly chart, XP history)
│   │   ├── daily_checkin_service.go  # Daily check-in service (save/upsert, get by date)
│   │   └── daily_review_service.go   # Daily review service (save/upsert, summary, get by date)
│   ├── testutils/
│   │   ├── test_config.go           # Test config loader
│   │   ├── test_db.go               # Test DB setup/helpers (SQLite in-memory)
│   │   └── test_router.go           # Test router setup
│   └── utils/
│       └── context.go                # User context helpers
├── migrations/
│   ├── 001_init_schema.up.sql        # Baseline schema (all 12 tables)
│   ├── 001_init_schema.down.sql      # Drop all tables (TEST ONLY)
│   ├── 002_fix_daily_checkin_enum_constraints.up.sql   # Fix numeric->enum drift
│   └── 002_fix_daily_checkin_enum_constraints.down.sql # Safe no-op
├── docs/
│   └── API_CONTRACT.md              # Full API contract for Flutter integration
├── tests/
│   └── e2e/
│       ├── onboarding_flow_test.go        # Onboarding E2E tests
│       ├── quest_action_flow_test.go       # Quest action E2E tests
│       ├── progress_log_flow_test.go       # Progress + log E2E tests
│       ├── daily_checkin_review_flow_test.go # Daily check-in + review E2E tests
│       ├── reward_claim_flow_test.go       # Reward claim E2E tests
│       └── full_core_flow_test.go          # Full core flow E2E test (26 sub-tests)
├── scripts/
│   ├── create_test_db.sh            # Test DB setup script
│   └── smoke_core_flow.sh           # Smoke test script for manual verification
├── docker-compose.yml                 # Local PostgreSQL setup
├── .env                               # Environment variables (dev)
├── .env.test                          # Test environment variables
└── .env.example                       # Environment template
```

## Current Status

**Phase 12A — Database Migration Strategy Overhaul**
- Replaced GORM AutoMigrate with versioned SQL migrations (golang-migrate)
- Created baseline schema migration for all 12 tables
- Created migration to fix daily_checkins enum drift (numeric -> string)
- Added migration CLI tool (`cmd/migrate/main.go`)
- Added migration safety tests against soloquest_test
- All timestamps use UTC

## Backend Time Standard

**UTC only.** All timestamps in the backend use UTC. Frontend will convert UTC timestamps to local display time.

Current limitation: user-specific timezone support is postponed.

## Database Migration Strategy

### Source of Truth

**SQL migration files in `migrations/` are the source of truth for all schema changes.**

GORM AutoMigrate is NOT used for normal application startup. It is kept as `DeprecatedAutoMigrate` for emergency/dev-only prototyping only.

### Why AutoMigrate Was Replaced

- AutoMigrate can fail with schema drift when column types change (e.g., numeric -> enum string)
- Old check constraints persist while AutoMigrate tries to alter column types
- AutoMigrate guesses schema from Go models, which is fragile for type changes
- No version tracking or rollback capability
- Cannot safely handle data migration during schema changes

### Migration Tool

Uses [golang-migrate/migrate](https://github.com/golang-migrate/migrate) with PostgreSQL driver.

```bash
# Run all pending migrations
go run cmd/migrate/main.go up

# Roll back last migration (DANGEROUS - do not use on dev data casually)
go run cmd/migrate/main.go down

# Show current migration version
go run cmd/migrate/main.go version
```

Set `MIGRATIONS_PATH` env var to override the default `./migrations` directory.

### Creating New Migrations

Every DB schema change must create a new migration file pair:

```
migrations/003_your_description.up.sql
migrations/003_your_description.down.sql
```

Number them sequentially. The up migration applies the change, the down migration reverses it.

#### Examples

**Adding a column:**
```sql
-- 003_add_avatar_color.up.sql
ALTER TABLE user_profiles ADD COLUMN avatar_color VARCHAR(20) DEFAULT '#4A90D9';
```

**Renaming a column:**
```sql
-- 004_rename_display_name.up.sql
ALTER TABLE user_profiles RENAME COLUMN display_name TO full_name;
```

**Changing column type with USING:**
```sql
-- 005_change_field_type.up.sql
ALTER TABLE daily_checkins ALTER COLUMN energy_level TYPE VARCHAR(20) USING energy_level::VARCHAR(20);
```

**Dropping constraints safely (idempotent):**
```sql
-- 006_drop_old_constraint.up.sql
DO $$
BEGIN
    ALTER TABLE daily_checkins DROP CONSTRAINT IF EXISTS old_constraint_name;
EXCEPTION WHEN OTHERS THEN NULL;
END $$;
```

**Preserving data during migrations:**
```sql
-- 007_convert_values.up.sql
UPDATE daily_checkins SET energy_level = CASE energy_level
    WHEN '1' THEN 'veryLow'
    WHEN '2' THEN 'low'
    WHEN '3' THEN 'medium'
    WHEN '4' THEN 'high'
    WHEN '5' THEN 'veryHigh'
    ELSE energy_level
END
WHERE energy_level ~ '^[0-9]+$';
```

## Database Policy

### Development Database (`soloquest`)

- **Preserve data.** Do not drop database, drop tables, or truncate data.
- Use forward migrations that preserve and transform existing data.
- Do not run destructive reset automatically.
- Schema changes must be done by forward migrations only.

### Test Database (`soloquest_test`)

- Can be cleaned/reset/truncated during tests.
- Test cleanup must never touch the development database.
- Tests verify `DATABASE_URL` contains `soloquest_test` before any destructive operation.

### Production Database

- Do not run destructive migrations automatically.
- Migration strategy must be explicit and controlled (manual for now).

## API Contract

Full API contract document: `docs/API_CONTRACT.md`

This document covers all endpoints with request/response shapes for Flutter integration.

## Implemented Feature Groups

- **Dev auth/user context** — Dev login, user profile
- **Onboarding** — Save onboarding data, check status
- **Quest actions** — Start, complete, skip, snooze quests
- **Progress** — Computed progress, weekly chart, XP history
- **Logs** — Filtered log entries (type, date range, pagination)
- **Daily check-in** — Morning state capture, upsert by date
- **Daily review** — End-of-day reflection, computed quest summary
- **Rewards claim** — Transactional claim with points deduction
- **Settings** — User settings read

## Test Environment

### .env.test

```
PORT=9001
APP_ENV=test
DATABASE_URL=postgres://postgres@localhost:5432/soloquest_test?sslmode=disable
JWT_SECRET=test_secret
DEV_USER_EMAIL=testuser@soloquest.local
```

### Create Test Database

```bash
./scripts/create_test_db.sh

# Or manually
docker exec soloquest-db psql -U postgres -c "CREATE DATABASE soloquest_test"
```

## How to Run

```bash
# 1. Start PostgreSQL
docker compose up -d

# 2. Install dependencies
go mod tidy

# 3. Run database migrations
go run cmd/migrate/main.go up

# 4. Run the server
go run cmd/api/main.go
```

The server starts on `PORT` (default 9000). On startup, it automatically runs pending SQL migrations.

## How to Run Tests

```bash
# Run all tests
go test ./...

# Run unit tests only
go test ./internal/services/...
go test ./internal/handlers/...

# Run migration tests (requires soloquest_test database)
go test ./internal/database/...

# Run E2E tests only
go test ./tests/e2e/...

# Run with verbose output
go test -v ./...
```

Note: Unit/handler/E2E tests use SQLite in-memory. Migration tests use `soloquest_test` PostgreSQL database.

## How to Run Smoke Script

```bash
# Against local running backend
./scripts/smoke_core_flow.sh

# Against custom URL
./scripts/smoke_core_flow.sh http://localhost:9000
```

## Test Coverage Summary

| Test File | Tests | Description |
|-----------|-------|-------------|
| `internal/database/migration_runner_test.go` | 3 | Migration up, idempotent, version check |
| `internal/database/schema_migration_test.go` | 3 | Enum drift fix, data preservation, safety check |
| `internal/services/bootstrap_service_test.go` | 3 | Bootstrap user creation, idempotency |
| `internal/services/onboarding_service_test.go` | 5 | Onboarding save, update, upsert, log creation |
| `internal/services/quest_action_service_test.go` | 11 | Start, complete, skip, snooze + edge cases |
| `internal/services/progress_service_test.go` | 8 | Progress, weekly chart, XP history + filters |
| `internal/services/log_service_test.go` | 8 | Log filtering by type/date/from/to + pagination |
| `internal/services/daily_checkin_service_test.go` | 5 | Check-in save, upsert, get, validation |
| `internal/services/daily_review_service_test.go` | 7 | Review save, upsert, summary, get, validation |
| `internal/services/reward_service_test.go` | 8 | Reward list, claim, insufficient, double claim, redemption history |
| `internal/handlers/user_handler_test.go` | 5 | GET /me, onboarding status, save onboarding |
| `internal/handlers/quest_action_handler_test.go` | 6 | Quest action endpoints + error cases |
| `internal/handlers/progress_handler_test.go` | 10 | Progress/chart/xp-history endpoints + filters |
| `internal/handlers/daily_checkin_handler_test.go` | 4 | Check-in endpoints + validation |
| `internal/handlers/daily_review_handler_test.go` | 5 | Review endpoints + validation |
| `internal/handlers/reward_handler_test.go` | 6 | Reward list, claim, insufficient, double, invalid ID, redemptions |
| `tests/e2e/onboarding_flow_test.go` | 1 (4 sub-tests) | Full onboarding flow |
| `tests/e2e/quest_action_flow_test.go` | 3 (12 sub-tests) | Full quest action flow |
| `tests/e2e/progress_log_flow_test.go` | 1 (8 sub-tests) | Progress + log filtering E2E flow |
| `tests/e2e/daily_checkin_review_flow_test.go` | 1 (9 sub-tests) | Daily check-in + review E2E flow |
| `tests/e2e/reward_claim_flow_test.go` | 1 (8 sub-tests) | Reward claim full E2E flow |
| `tests/e2e/full_core_flow_test.go` | 1 (26 sub-tests) | Full core user journey E2E test |

## Available Endpoints

| Method | Endpoint | Response |
|--------|----------|----------|
| GET | `/health` | `{"status": "ok", "service": "soloquest-backend"}` |
| GET | `/health/db` | `{"status": "ok", "database": "connected"}` |
| GET | `/health/models` | `{"status": "ok", "models": [...]}` |
| POST | `/api/auth/dev-login` | Dev user + fake tokens |
| GET | `/api/users/me` | Current user profile |
| GET | `/api/users/me/daily-status` | Daily check-in + review status |
| POST | `/api/users/me/onboarding` | Save onboarding data |
| GET | `/api/users/me/onboarding/status` | Onboarding status |
| GET | `/api/quests` | Quests for current user |
| POST | `/api/quests/:id/start` | Start a quest |
| POST | `/api/quests/:id/complete` | Complete a quest (earn XP) |
| POST | `/api/quests/:id/skip` | Skip a quest |
| POST | `/api/quests/:id/snooze` | Snooze a quest |
| GET | `/api/checkins/today` | Today's check-in status |
| GET | `/api/checkins?date=YYYY-MM-DD` | Check-in by date |
| POST | `/api/checkins` | Save/upsert daily check-in |
| GET | `/api/reviews/today` | Today's review status |
| GET | `/api/reviews?date=YYYY-MM-DD` | Review by date |
| GET | `/api/reviews/summary?date=YYYY-MM-DD` | Computed quest summary |
| POST | `/api/reviews` | Save/upsert daily review |
| GET | `/api/progress` | Computed progress for current user |
| GET | `/api/progress/weekly-chart` | Weekly chart (Mon-Sun) |
| GET | `/api/progress/xp-history` | XP transaction history |
| GET | `/api/rewards` | Rewards with wallet + can_claim |
| POST | `/api/rewards/:id/claim` | Claim a reward (transactional) |
| GET | `/api/rewards/redemptions` | Reward redemption history |
| GET | `/api/logs` | Log entries with filtering |
| GET | `/api/settings` | Settings for current user |

## Current Limitations

- Real Google login is not implemented yet
- Production JWT is not enforced yet
- AI quest generation is not implemented yet
- Flutter is not integrated yet
- User-specific timezone is postponed

## Next Recommended Step

Phase 13 — Flutter API foundation and mock replacement plan
or
Phase 13A — Real Google login/JWT before Flutter, if production auth is required first
