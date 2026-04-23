// Package storage defines persistence abstractions (interfaces and types)
// for the reminder bot and provides a SQLite-backed implementation.
// Business logic depends only on the interfaces, making the bot
// testable with in-memory fakes and portable to other backends.
package storage

import (
        "context"
        "database/sql"
        "errors"
        "fmt"
        "os"
        "path/filepath"
        "strings"
        "time"

        // modernc.org/sqlite is a CGO-free pure-Go SQLite driver; imported for
        // its side-effects (registration of the "sqlite" database/sql driver).
        _ "modernc.org/sqlite"
)

type sqliteStorage struct {
        db        *sql.DB
        reminders *reminderRepo
        sessions  *sessionRepo
        friends   *friendRepo
        users     *userRepo
}

func NewSQLiteStorage(dbPath string) (Storage, error) {
        if dir := filepath.Dir(dbPath); dir != "." && dir != "" {
                _ = os.MkdirAll(dir, 0o755)
        }

        db, err := sql.Open("sqlite", dbPath+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
        if err != nil {
                return nil, fmt.Errorf("storage: failed to open sqlite: %w", err)
        }

        ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()

        if err := db.PingContext(ctx); err != nil {
                return nil, fmt.Errorf("storage: failed to ping: %w", err)
        }

        if err := initSchema(ctx, db); err != nil {
                return nil, fmt.Errorf("storage: failed to init schema: %w", err)
        }

        s := &sqliteStorage{
                db: db,
        }
        s.reminders = &reminderRepo{db: db}
        s.sessions = &sessionRepo{db: db}
        s.friends = &friendRepo{db: db}
        s.users = &userRepo{db: db}

        return s, nil
}

func (s *sqliteStorage) Reminders() ReminderRepository {
        return s.reminders
}

func (s *sqliteStorage) Sessions() SessionRepository {
        return s.sessions
}

func (s *sqliteStorage) Friends() FriendRepository {
        return s.friends
}

func (s *sqliteStorage) Users() UserRepository {
        return s.users
}

func (s *sqliteStorage) Close() error {
        return s.db.Close()
}

func initSchema(ctx context.Context, db *sql.DB) error {
        queries := []string{
                `CREATE TABLE IF NOT EXISTS reminders (
                        id              INTEGER PRIMARY KEY AUTOINCREMENT,
                        chat_id         INTEGER NOT NULL,
                        author_id       INTEGER DEFAULT 0,
                        text            TEXT    NOT NULL,
                        notify_at       DATETIME NOT NULL,
                        interval        TEXT    DEFAULT '',
                        weekdays        INTEGER DEFAULT 0,
                        last_message_id INTEGER DEFAULT 0,
                        created_at      DATETIME DEFAULT CURRENT_TIMESTAMP
                )`,
                `CREATE INDEX IF NOT EXISTS idx_notify_at ON reminders(notify_at)`,
                `CREATE TABLE IF NOT EXISTS user_states (
                        chat_id            INTEGER PRIMARY KEY,
                        state              TEXT NOT NULL DEFAULT '',
                        pending_text       TEXT DEFAULT '',
                        pending_reminder_id INTEGER DEFAULT 0,
                        session_message_id INTEGER DEFAULT 0,
                        timezone           TEXT DEFAULT '',
                        updated_at         DATETIME DEFAULT CURRENT_TIMESTAMP
                )`,
                `CREATE TABLE IF NOT EXISTS friends (
                        user_id    INTEGER NOT NULL,
                        friend_id  INTEGER NOT NULL,
                        status     TEXT NOT NULL,
                        created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
                        PRIMARY KEY (user_id, friend_id)
                )`,
                `CREATE TABLE IF NOT EXISTS users (
                        chat_id    INTEGER PRIMARY KEY,
                        first_name TEXT DEFAULT '',
                        last_name  TEXT DEFAULT '',
                        username   TEXT DEFAULT '',
                        updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
                )`,
        }

        for _, q := range queries {
                if _, err := db.ExecContext(ctx, q); err != nil {
                        return err
                }
        }

        // Migration: ensure new columns exist (for existing DBs)
        _, _ = db.ExecContext(ctx, "ALTER TABLE reminders ADD COLUMN last_message_id INTEGER DEFAULT 0")
        _, _ = db.ExecContext(ctx, "ALTER TABLE reminders ADD COLUMN interval TEXT DEFAULT ''")
        _, _ = db.ExecContext(ctx, "ALTER TABLE reminders ADD COLUMN weekdays INTEGER DEFAULT 0")
        _, _ = db.ExecContext(ctx, "ALTER TABLE reminders ADD COLUMN author_id INTEGER DEFAULT 0")
        _, _ = db.ExecContext(ctx, "ALTER TABLE user_states ADD COLUMN pending_text TEXT DEFAULT ''")
        _, _ = db.ExecContext(ctx, "ALTER TABLE user_states ADD COLUMN session_message_id INTEGER DEFAULT 0")
        _, _ = db.ExecContext(ctx, "ALTER TABLE user_states ADD COLUMN pending_reminder_id INTEGER DEFAULT 0")
        _, _ = db.ExecContext(ctx, "ALTER TABLE user_states ADD COLUMN timezone TEXT DEFAULT ''")

        return nil
}

// --- Reminder Repository Implementation ---

type reminderRepo struct {
        db *sql.DB
}

func (r *reminderRepo) Add(ctx context.Context, rem *Reminder) error {
        res, err := r.db.ExecContext(ctx,
                "INSERT INTO reminders (chat_id, author_id, text, notify_at, interval, weekdays) VALUES (?, ?, ?, ?, ?, ?)",
                rem.ChatID, rem.AuthorID, rem.Text, rem.NotifyAt.UTC(), rem.Interval, rem.Weekdays,
        )
        if err != nil {
                return fmt.Errorf("reminderRepo.Add: exec: %w", err)
        }
        id, err := res.LastInsertId()
        if err != nil {
                return fmt.Errorf("reminderRepo.Add: last insert id: %w", err)
        }
        rem.ID = id
        return nil
}

func (r *reminderRepo) GetByChatID(ctx context.Context, chatID int64) ([]Reminder, error) {
        rows, err := r.db.QueryContext(ctx,
                "SELECT id, chat_id, author_id, text, notify_at, interval, weekdays, last_message_id, created_at FROM reminders WHERE chat_id = ? AND author_id = 0 ORDER BY notify_at ASC",
                chatID,
        )
        if err != nil {
                return nil, err
        }
        defer func() { _ = rows.Close() }()

        var list []Reminder
        for rows.Next() {
                var rem Reminder
                if err := rows.Scan(&rem.ID, &rem.ChatID, &rem.AuthorID, &rem.Text, &rem.NotifyAt, &rem.Interval, &rem.Weekdays, &rem.LastMessageID, &rem.CreatedAt); err != nil {
                        return nil, err
                }
                list = append(list, rem)
        }
        return list, rows.Err()
}

func (r *reminderRepo) GetByID(ctx context.Context, id int64) (*Reminder, error) {
        var rem Reminder
        err := r.db.QueryRowContext(ctx,
                "SELECT id, chat_id, author_id, text, notify_at, interval, weekdays, last_message_id, created_at FROM reminders WHERE id = ?",
                id,
        ).Scan(&rem.ID, &rem.ChatID, &rem.AuthorID, &rem.Text, &rem.NotifyAt, &rem.Interval, &rem.Weekdays, &rem.LastMessageID, &rem.CreatedAt)
        if err != nil {
                if errors.Is(err, sql.ErrNoRows) {
                        return nil, ErrNotFound
                }
                return nil, err
        }
        return &rem, nil
}

func (r *reminderRepo) Delete(ctx context.Context, chatID, id int64) error {
        res, err := r.db.ExecContext(ctx, "DELETE FROM reminders WHERE id = ? AND chat_id = ?", id, chatID)
        if err != nil {
                return err
        }
        rows, err := res.RowsAffected()
        if err != nil {
                return err
        }
        if rows == 0 {
                return ErrNotFound
        }
        return nil
}

func (r *reminderRepo) Update(ctx context.Context, rem *Reminder) error {
        res, err := r.db.ExecContext(ctx,
                "UPDATE reminders SET text = ?, notify_at = ?, interval = ?, weekdays = ?, last_message_id = ?, author_id = ? WHERE id = ?",
                rem.Text, rem.NotifyAt.UTC(), rem.Interval, rem.Weekdays, rem.LastMessageID, rem.AuthorID, rem.ID,
        )
        if err != nil {
                return err
        }
        rows, err := res.RowsAffected()
        if err != nil {
                return err
        }
        if rows == 0 {
                return ErrNotFound
        }
        return nil
}

func (r *reminderRepo) GetDue(ctx context.Context, before time.Time) ([]Reminder, error) {
        rows, err := r.db.QueryContext(ctx,
                "SELECT id, chat_id, author_id, text, notify_at, interval, weekdays, last_message_id, created_at FROM reminders WHERE notify_at <= ? ORDER BY notify_at ASC",
                before.UTC(),
        )
        if err != nil {
                return nil, err
        }
        defer func() { _ = rows.Close() }()

        var list []Reminder
        for rows.Next() {
                var rem Reminder
                if err := rows.Scan(&rem.ID, &rem.ChatID, &rem.AuthorID, &rem.Text, &rem.NotifyAt, &rem.Interval, &rem.Weekdays, &rem.LastMessageID, &rem.CreatedAt); err != nil {
                        return nil, err
                }
                list = append(list, rem)
        }
        return list, rows.Err()
}

func (r *reminderRepo) MarkAsNotified(ctx context.Context, id int64, nextNotifyAt time.Time) error {
        res, err := r.db.ExecContext(ctx, "UPDATE reminders SET notify_at = ? WHERE id = ?", nextNotifyAt.UTC(), id)
        if err != nil {
                return err
        }
        rows, err := res.RowsAffected()
        if err != nil {
                return err
        }
        if rows == 0 {
                return ErrNotFound
        }
        return nil
}

func (r *reminderRepo) DeleteByID(ctx context.Context, id int64) error {
        res, err := r.db.ExecContext(ctx, "DELETE FROM reminders WHERE id = ?", id)
        if err != nil {
                return err
        }
        rows, err := res.RowsAffected()
        if err != nil {
                return err
        }
        if rows == 0 {
                return ErrNotFound
        }
        return nil
}

func (r *reminderRepo) GetByAuthorAndTarget(ctx context.Context, authorID, targetChatID int64) ([]Reminder, error) {
        rows, err := r.db.QueryContext(ctx,
                "SELECT id, chat_id, author_id, text, notify_at, interval, weekdays, last_message_id, created_at FROM reminders WHERE author_id = ? AND chat_id = ? ORDER BY notify_at ASC",
                authorID, targetChatID,
        )
        if err != nil {
                return nil, err
        }
        defer func() { _ = rows.Close() }()

        var list []Reminder
        for rows.Next() {
                var rem Reminder
                if err := rows.Scan(&rem.ID, &rem.ChatID, &rem.AuthorID, &rem.Text, &rem.NotifyAt, &rem.Interval, &rem.Weekdays, &rem.LastMessageID, &rem.CreatedAt); err != nil {
                        return nil, err
                }
                list = append(list, rem)
        }
        return list, rows.Err()
}

func (r *reminderRepo) GetFriendReminders(ctx context.Context, chatID int64) ([]Reminder, error) {
        rows, err := r.db.QueryContext(ctx,
                "SELECT id, chat_id, author_id, text, notify_at, interval, weekdays, last_message_id, created_at FROM reminders WHERE chat_id = ? AND author_id != 0 ORDER BY notify_at ASC",
                chatID,
        )
        if err != nil {
                return nil, err
        }
        defer func() { _ = rows.Close() }()

        var list []Reminder
        for rows.Next() {
                var rem Reminder
                if err := rows.Scan(&rem.ID, &rem.ChatID, &rem.AuthorID, &rem.Text, &rem.NotifyAt, &rem.Interval, &rem.Weekdays, &rem.LastMessageID, &rem.CreatedAt); err != nil {
                        return nil, err
                }
                list = append(list, rem)
        }
        return list, rows.Err()
}

func (r *reminderRepo) GetOutgoingFriendReminders(ctx context.Context, authorID int64) ([]Reminder, error) {
        rows, err := r.db.QueryContext(ctx,
                "SELECT id, chat_id, author_id, text, notify_at, interval, weekdays, last_message_id, created_at FROM reminders WHERE author_id = ? AND chat_id != ? ORDER BY notify_at ASC",
                authorID, authorID,
        )
        if err != nil {
                return nil, fmt.Errorf("reminderRepo.GetOutgoingFriendReminders: query: %w", err)
        }
        defer func() { _ = rows.Close() }()

        var list []Reminder
        for rows.Next() {
                var rem Reminder
                if err := rows.Scan(&rem.ID, &rem.ChatID, &rem.AuthorID, &rem.Text, &rem.NotifyAt, &rem.Interval, &rem.Weekdays, &rem.LastMessageID, &rem.CreatedAt); err != nil {
                        return nil, fmt.Errorf("reminderRepo.GetOutgoingFriendReminders: scan: %w", err)
                }
                list = append(list, rem)
        }
        return list, rows.Err()
}


func (r *reminderRepo) ClearAuthor(ctx context.Context, authorID, targetChatID int64) error {
        _, err := r.db.ExecContext(ctx,
                "UPDATE reminders SET author_id = 0 WHERE author_id = ? AND chat_id = ?",
                authorID, targetChatID,
        )
        return err
}

// --- Session Repository Implementation ---

type sessionRepo struct {
        db *sql.DB
}

// ensureSession creates a session row with default values if it doesn't exist.
// This prevents individual Set* methods from overwriting other fields on INSERT.
func (s *sessionRepo) ensureSession(ctx context.Context, chatID int64) {
        _, _ = s.db.ExecContext(ctx,
                `INSERT OR IGNORE INTO user_states (chat_id, state, updated_at) VALUES (?, '', CURRENT_TIMESTAMP)`,
                chatID,
        )
}

func (s *sessionRepo) SetState(ctx context.Context, chatID int64, state string) error {
        _, err := s.db.ExecContext(ctx,
                `INSERT INTO user_states (chat_id, state, updated_at) VALUES (?, ?, CURRENT_TIMESTAMP)
                ON CONFLICT(chat_id) DO UPDATE SET state = excluded.state, updated_at = CURRENT_TIMESTAMP`,
                chatID, state,
        )
        return err
}

func (s *sessionRepo) GetState(ctx context.Context, chatID int64) (string, error) {
        var state string
        err := s.db.QueryRowContext(ctx, "SELECT state FROM user_states WHERE chat_id = ?", chatID).Scan(&state)
        if err != nil {
                if errors.Is(err, sql.ErrNoRows) {
                        return "", nil
                }
                return "", err
        }
        return state, nil
}

func (s *sessionRepo) DeleteState(ctx context.Context, chatID int64) error {
        _, err := s.db.ExecContext(ctx, "UPDATE user_states SET state = '', updated_at = CURRENT_TIMESTAMP WHERE chat_id = ?", chatID)
        return err
}

func (s *sessionRepo) SetPendingText(ctx context.Context, chatID int64, text string) error {
        s.ensureSession(ctx, chatID)
        _, err := s.db.ExecContext(ctx,
                `UPDATE user_states SET pending_text = ?, updated_at = CURRENT_TIMESTAMP WHERE chat_id = ?`,
                text, chatID,
        )
        return err
}

func (s *sessionRepo) GetPendingText(ctx context.Context, chatID int64) (string, error) {
        var text string
        err := s.db.QueryRowContext(ctx, "SELECT pending_text FROM user_states WHERE chat_id = ?", chatID).Scan(&text)
        if err != nil {
                if errors.Is(err, sql.ErrNoRows) {
                        return "", nil
                }
                return "", err
        }
        return text, nil
}

func (s *sessionRepo) ClearPendingText(ctx context.Context, chatID int64) error {
        _, err := s.db.ExecContext(ctx, "UPDATE user_states SET pending_text = '', updated_at = CURRENT_TIMESTAMP WHERE chat_id = ?", chatID)
        return err
}

func (s *sessionRepo) SetSessionMessageID(ctx context.Context, chatID int64, messageID int) error {
        s.ensureSession(ctx, chatID)
        _, err := s.db.ExecContext(ctx,
                `UPDATE user_states SET session_message_id = ?, updated_at = CURRENT_TIMESTAMP WHERE chat_id = ?`,
                messageID, chatID,
        )
        return err
}

func (s *sessionRepo) GetSessionMessageID(ctx context.Context, chatID int64) (int, error) {
        var id int
        err := s.db.QueryRowContext(ctx, "SELECT session_message_id FROM user_states WHERE chat_id = ?", chatID).Scan(&id)
        if err != nil {
                if errors.Is(err, sql.ErrNoRows) {
                        return 0, nil
                }
                return 0, err
        }
        return id, nil
}

func (s *sessionRepo) SetPendingReminderID(ctx context.Context, chatID, id int64) error {
        s.ensureSession(ctx, chatID)
        _, err := s.db.ExecContext(ctx,
                `UPDATE user_states SET pending_reminder_id = ?, updated_at = CURRENT_TIMESTAMP WHERE chat_id = ?`,
                id, chatID,
        )
        return err
}

func (s *sessionRepo) GetPendingReminderID(ctx context.Context, chatID int64) (int64, error) {
        var id int64
        err := s.db.QueryRowContext(ctx, "SELECT pending_reminder_id FROM user_states WHERE chat_id = ?", chatID).Scan(&id)
        if err != nil {
                if errors.Is(err, sql.ErrNoRows) {
                        return 0, nil
                }
                return 0, err
        }
        return id, nil
}

func (s *sessionRepo) SetTimezone(ctx context.Context, chatID int64, tz string) error {
        s.ensureSession(ctx, chatID)
        _, err := s.db.ExecContext(ctx,
                `UPDATE user_states SET timezone = ?, updated_at = CURRENT_TIMESTAMP WHERE chat_id = ?`,
                tz, chatID,
        )
        return err
}

func (s *sessionRepo) GetTimezone(ctx context.Context, chatID int64) (string, error) {
        var tz string
        err := s.db.QueryRowContext(ctx, "SELECT timezone FROM user_states WHERE chat_id = ?", chatID).Scan(&tz)
        if err != nil {
                if errors.Is(err, sql.ErrNoRows) {
                        return "", nil
                }
                return "", err
        }
        return tz, nil
}

func (s *sessionRepo) Cleanup(ctx context.Context, olderThan time.Time) error {
        _, err := s.db.ExecContext(ctx, "DELETE FROM user_states WHERE updated_at < ?", olderThan.UTC())
        return err
}

// --- Friend Repository Implementation ---

type friendRepo struct {
        db *sql.DB
}

func (f *friendRepo) AddFriend(ctx context.Context, userID, friendID int64) error {
        _, err := f.db.ExecContext(ctx,
                `INSERT INTO friends (user_id, friend_id, status, created_at) VALUES (?, ?, ?, CURRENT_TIMESTAMP)`,
                userID, friendID, FriendStatusPending,
        )
        if err != nil {
                // Check for unique constraint violation
                if strings.Contains(err.Error(), "UNIQUE constraint") || strings.Contains(err.Error(), "PRIMARY KEY") {
                        return ErrAlreadyExists
                }
                return err
        }
        return nil
}

func (f *friendRepo) AcceptFriend(ctx context.Context, userID, friendID int64) error {
        // Update the pending request to accepted
        res, err := f.db.ExecContext(ctx,
                `UPDATE friends SET status = ? WHERE user_id = ? AND friend_id = ? AND status = ?`,
                FriendStatusAccepted, userID, friendID, FriendStatusPending,
        )
        if err != nil {
                return err
        }
        rows, err := res.RowsAffected()
        if err != nil {
                return err
        }
        if rows == 0 {
                return ErrNotFound
        }
        // Create the reverse direction (friendID -> userID) as accepted
        _, _ = f.db.ExecContext(ctx,
                `INSERT OR IGNORE INTO friends (user_id, friend_id, status, created_at) VALUES (?, ?, ?, CURRENT_TIMESTAMP)`,
                friendID, userID, FriendStatusAccepted,
        )
        return nil
}

func (f *friendRepo) RemoveFriend(ctx context.Context, userID, friendID int64) error {
        // Remove both directions
        _, err := f.db.ExecContext(ctx,
                `DELETE FROM friends WHERE (user_id = ? AND friend_id = ?) OR (user_id = ? AND friend_id = ?)`,
                userID, friendID, friendID, userID,
        )
        return err
}

func (f *friendRepo) GetFriends(ctx context.Context, userID int64) ([]Friend, error) {
        rows, err := f.db.QueryContext(ctx,
                `SELECT user_id, friend_id, status, created_at FROM friends WHERE user_id = ? AND status = ?`,
                userID, FriendStatusAccepted,
        )
        if err != nil {
                return nil, err
        }
        defer func() { _ = rows.Close() }()

        var list []Friend
        for rows.Next() {
                var fr Friend
                if err := rows.Scan(&fr.UserID, &fr.FriendID, &fr.Status, &fr.CreatedAt); err != nil {
                        return nil, err
                }
                list = append(list, fr)
        }
        return list, rows.Err()
}

func (f *friendRepo) GetPendingRequests(ctx context.Context, userID int64) ([]Friend, error) {
        rows, err := f.db.QueryContext(ctx,
                `SELECT user_id, friend_id, status, created_at FROM friends WHERE friend_id = ? AND status = ?`,
                userID, FriendStatusPending,
        )
        if err != nil {
                return nil, err
        }
        defer func() { _ = rows.Close() }()

        var list []Friend
        for rows.Next() {
                var fr Friend
                if err := rows.Scan(&fr.UserID, &fr.FriendID, &fr.Status, &fr.CreatedAt); err != nil {
                        return nil, err
                }
                list = append(list, fr)
        }
        return list, rows.Err()
}

func (f *friendRepo) IsFriend(ctx context.Context, userID, friendID int64) (bool, error) {
        var count int
        err := f.db.QueryRowContext(ctx,
                `SELECT COUNT(*) FROM friends WHERE user_id = ? AND friend_id = ? AND status = ?`,
                userID, friendID, FriendStatusAccepted,
        ).Scan(&count)
        if err != nil {
                return false, err
        }
        return count > 0, nil
}

func (f *friendRepo) HasPendingRequest(ctx context.Context, userID, friendID int64) (bool, error) {
        var count int
        err := f.db.QueryRowContext(ctx,
                `SELECT COUNT(*) FROM friends WHERE user_id = ? AND friend_id = ? AND status = ?`,
                userID, friendID, FriendStatusPending,
        ).Scan(&count)
        if err != nil {
                return false, err
        }
        return count > 0, nil
}

// --- User Repository Implementation ---

type userRepo struct {
        db *sql.DB
}

func (u *userRepo) Upsert(ctx context.Context, user *User) error {
        _, err := u.db.ExecContext(ctx,
                `INSERT INTO users (chat_id, first_name, last_name, username, updated_at)
                VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
                ON CONFLICT(chat_id) DO UPDATE SET
                        first_name = excluded.first_name,
                        last_name = excluded.last_name,
                        username = excluded.username,
                        updated_at = CURRENT_TIMESTAMP`,
                user.ChatID, user.FirstName, user.LastName, user.Username,
        )
        return err
}

func (u *userRepo) Get(ctx context.Context, chatID int64) (*User, error) {
        var user User
        err := u.db.QueryRowContext(ctx,
                `SELECT chat_id, first_name, last_name, username, updated_at FROM users WHERE chat_id = ?`,
                chatID,
        ).Scan(&user.ChatID, &user.FirstName, &user.LastName, &user.Username, &user.UpdatedAt)
        if err != nil {
                if errors.Is(err, sql.ErrNoRows) {
                        return nil, ErrNotFound
                }
                return nil, err
        }
        return &user, nil
}
