# Agent Guidelines for FatBot

## Build/Test Commands

- Build: `go build -o fatbot`
- Run: `go run .`
- Test all: `go test ./...`
- Test single package: `go test ./users` (or ./updates, ./state, etc.)
- Test single function: `go test -run TestIsOlderThan ./users`

## Code Style

- Import order: stdlib, external packages, then `fatbot/*` internal packages
- Use `gorm.Model` embedding for DB models with timestamps
- Error handling: return errors, log with `log.Error()`, capture critical ones with `sentry.CaptureException(err)`
- Use `charmbracelet/log` for logging (levels: Debug, Info, Warn, Error, Fatal)
- Types: define struct types in package files (e.g., `User`, `Workout`, `Group` in users/)
- Naming: PascalCase for exported, camelCase for unexported, descriptive names
- Custom errors: define typed errors with Error() method (see `NoSuchUserError`, `NoSuchUpdateError`)
- Telegram API: use `tgbotapi` from local replacement `./telegram-bot-apix/`
- Config: use `viper` for config loading from `config.yaml`, `os.Getenv()` for secrets
- State management: Redis-based state in `state/` package with menu-driven flows
- Testing: table-driven tests with subtests (see `workouts_test.go`)

## Environment

- Go 1.22, SQLite (custom fork), Redis for state, Sentry for production errors
- Set `TELEGRAM_APITOKEN`, `OPENAI_APITOKEN`, `SENTRY_DSN` via env vars
