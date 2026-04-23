# PRD — Reminder Bot Refactor (Senior Go)

## Original problem statement (2026-04-23)

> Проанализируй код в этом репозитории; Обрати внимание на файл
> PROJECT_OVERVIEW.md, agents.md, claude.md, а также и на всю остальную
> кодовую базу; Требуется сделать рефакторинг и при этом НИЧЕГО не
> сломать; Убедись то что все необходимые тесты написаны, и то что эти
> тесты написаны не ради тестов, а ради того чтобы реально что то
> проверять.
> Также убедись что уровень проекта соответствует Senior Go developer.
> Я тебе разрешаю делать Все; Permission Level - MAXIMUM; Главное
> ничего не сломай по итогу)

## Product

Telegram reminder bot in Go 1.25.7 (~7,400 LoC). SQLite storage.
Natural-language parsing (RU/EN), recurrence (weekdays/interval),
timezone auto-detection, social features (friends, cross-user
reminders).

## Personas

- **Alex** — power user juggling work + personal reminders.
- **Marina** — professional receiving scheduled reminders from colleagues.
- **Nikita** — friend-reminder sender (another user's nag-bot).

## Architecture (post-refactor)

```
cmd/bot/main.go      — entry point (exits non-zero on bot failure)
internal/config      — typed .env loader (100% covered)
internal/storage     — SQLite repos behind interfaces (76% covered)
internal/core        — pure business logic (82% covered)
    ├── parser.go        — NLP time/interval parser w/ injected Clock
    ├── recurrence.go    — NextOccurrence (weekdays + interval)
    ├── timezone.go      — alias/offset/IANA + Nominatim geocode
    ├── service.go       — ReminderService façade
    ├── friends.go       — FriendService façade
    ├── state.go         — SessionManager / FSM
    ├── notifier.go      — NotificationManager (batch, back-off)
    └── scheduler.go     — periodic ticker loop
internal/telegram    — Telegram-SDK-bound handlers (51% covered)
```

## Requirements (static)

- Deterministic business logic (Clock injection for tests).
- Every reminder fires exactly once per trigger (no duplicates).
- Friendships are mutual; removing one clears author_id on both sides.
- All persistent types survive process restart.
- All public errors wrap the underlying cause with %w.
- Zero unchecked errors flagged by golangci-lint errcheck.

## What's been done (2026-04-23)

**Commits (atomic, as requested by user choice #5a):**

1. `a6a408a` fix(tests): restore green baseline
   — Added missing `Users()` on `dummyTestStorage`, fixed `ChatFullInfo`
     to telego v1.7.0 flat fields. Tests were failing to compile.

2. `f01092a` fix(core,storage): duplicate notifications + silent errors
   — Critical bug: `NotificationManager.ProcessDueReminders` overwrote
     `notify_at` with the original past value after a successful
     `Notify`, causing notification storms every scheduler tick.
     `reminderRepo.Add` silently swallowed `LastInsertId()` errors.
     Both fixed, both regression-tested.

3. `45bb841` refactor(core): deterministic clock, clean aliases, wrapped errors
   — `Clock` interface + `NewParserWithClock` (PROJECT_OVERVIEW §3).
   — Hoisted timezone alias map and HTTP client to package level.
   — All network / parse / exec errors wrapped with `%w` + context prefix.
   — `.golangci.yml` (errcheck/govet/staticcheck/unused/gocritic/revive/
     gosec/misspell/nolintlint) + rewritten `Makefile` with build/test/
     test-race/cover/fmt/vet/lint/tidy/ci targets.

4. `3c66a1b` test: expand coverage with scenarios from PROJECT_OVERVIEW
   — Added deterministic Parser tests (past-year rollover, 48h threshold,
     empty input, table-driven intervals).
   — Full Friends handshake: invite → accept/reject, mutuality, duplicate
     invite, author cleanup on unfriend, re-invite after removal.
   — Snooze chain (relative to latest trigger), weekly bitmask = 21.
   — Permission matrix for DeleteFriendReminder (author/target/stranger).
   — Batch scheduler (50 due in one tick, back-off end-to-end verified).
   — SQLite repos: handshake mutuality, UNIQUE-safe re-invite, users
     upsert round-trip, scope isolation GetByChatID vs GetFriendReminders,
     `ClearAuthor` scoping, concurrent `Add` with unique IDs.

5. `290d0f9` chore(lint): zero golangci-lint issues under Senior-Go ruleset
   — Package docs, justified blank import, `_ =` on fire-and-forget,
     `fmt.Fprintf` vs `WriteString(fmt.Sprintf...)`, combined param
     types, `0o755`, `http.NoBody`, range-validated weekday shifts,
     `behaviour`→`behavior`, context threaded through keyboard builders.
   — `main.go` now exits non-zero on bh.Start() failure.

## Quality gates (2026-04-23 final run)

| Gate                    | Result                           |
|-------------------------|----------------------------------|
| `go build ./...`        | OK                               |
| `go vet ./...`          | clean                            |
| `gofmt -l .`            | empty                            |
| `go test ./... -race`   | PASS (142 tests)                 |
| `golangci-lint run`     | 0 issues (v2.11.4, Senior rules) |
| Coverage                | core 82.7%, storage 76.3%, config 100%, total 61.5% |

## Backlog (P1)

- Replace startup `ALTER TABLE ... ADD COLUMN` probes with a
  versioned migration system (goose/golang-migrate).
- Wire Prometheus counters into NotificationManager (fired/snoozed/
  skipped/bounced).
- Add GitHub Actions workflow that runs `make ci`.
- Cover `internal/telegram` the rest of the way (51% → 75%+) via
  table-driven callback dispatch tests.

## Backlog (P2)

- Graceful DB-close on shutdown with a timeout.
- Pluggable storage (Postgres) behind the existing interface.
- i18n extraction (currently RU-hardcoded strings in rendering).
