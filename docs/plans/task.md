| Status | Task |
|---|---|
| ✅ | Task 1: Extract Parser utility |
| ✅ | Task 2: Extract StateManager |
| ✅ | Task 3: Split callbacks and text handlers |
| ✅ | Task 4: Inject Clock into Parser (deterministic time, PROJECT_OVERVIEW §3) |
| ✅ | Task 5: Fix ProcessDueReminders notify_at regression bug + tests |
| ✅ | Task 6: Fix swallowed LastInsertId error in reminderRepo.Add |
| ✅ | Task 7: Hoist timezone aliases and HTTP client to package level |
| ✅ | Task 8: Propagate context through friend-menu keyboard builders |
| ✅ | Task 9: Add golangci-lint config (Senior-Go ruleset) and reach zero-issue state |
| ✅ | Task 10: Expand test coverage to core 82%, storage 76% (friends, snooze, bitmask, batch scheduling, concurrent writes) |
| ✅ | Task 11: Package doc comments, error wrapping (%w), proper errcheck handling |
| ✅ | Task 12: main.go exits non-zero on Bot handler termination error |

## Follow-up ideas (not yet started)

- Replace `ALTER TABLE ... ADD COLUMN` bootstrap with a versioned migration table (goose / golang-migrate).
- Add repository-level context cancellation tests (insert/update while ctx is canceled).
- Add structured metrics (Prometheus counters for fired / snoozed / skipped reminders).
- Publish a release pipeline (.github/workflows) with the `make ci` target.
