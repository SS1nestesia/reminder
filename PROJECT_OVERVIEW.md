# Reminder Bot: Comprehensive Project Overview & AI Guide

This document is the **Single Source of Truth** for the Reminder Bot project. It contains architecture details, coding standards, technical specifications, and exhaustive test cases optimized for AI agents.

---

## 🚀 Project Overview

The Reminder Bot is a Telegram-based productivity tool designed for personal and social task management. It uses Natural Language Processing (NLP) to parse time and supports complex recurrence patterns.

### Core Features
- **NLP Parsing**: Russian/English support (e.g., *"через 2 часа"*, *"завтра в 10 утра"*).
- **Recurrence**: Daily, weekly (bitmask-based), and custom intervals.
- **Social Layer**: Invite friends via deep-links and set reminders for them.
- **Dynamic UI**: Edit, Snooze, and Reschedule notifications via inline buttons.
- **Timezone Awareness**: Coordinate-based or manual timezone configuration.

### 🛠 Tech Stack
- **Go 1.25.7**: Core language.
- **telego**: Telegram Bot API framework.
- **SQLite (modernc.org)**: CGO-free database.
- **when**: NLP engine with custom RU/EN rule-sets for non-trivial time parsing.
- **timezonemapper**: High-performance IANA timezone lookups for coordinate-based auto-config.
- **SQL-only States**: State management is moved from memory to SQLite to support **DBOS-style durable execution**.

---

## 🗄 Technical Architecture

### Database Schema (SQLite)

#### Table: `reminders`
| Column | Type | Description |
|--------|------|-------------|
| `id` | INT (PK) | Unique ID. |
| `chat_id` | INT | Owner of the reminder. |
| `author_id` | INT | 0 if self, else friend's Chat ID. |
| `text` | TEXT | Reminder content. |
| `notify_at` | DATETIME | Next trigger time (stored in **UTC**). |
| `interval` | TEXT | Duration string (e.g., "24h") for recurrence. |
| `weekdays` | INT | Bitmask (Mon=1, Tue=2, ..., Sun=64). |
| `last_message_id`| INT | ID of the last sent notification message. |
| `created_at` | DATETIME | Timestamp of creation. |

#### Table: `user_states` (Sessions)
| Column | Type | Description |
|--------|------|-------------|
| `chat_id` | INT (PK) | Telegram user ID. |
| `state` | TEXT | Current FSM state. |
| `pending_text` | TEXT | Buffered text for multi-step flows. |
| `pending_reminder_id` | INT | ID of reminder being edited. |
| `session_message_id` | INT | ID of the active menu message to update. |
| `timezone` | TEXT | User's IANA timezone string. |
| `updated_at` | DATETIME | Last state change. |

#### Table: `friends`
| Column | Type | Description |
|--------|------|-------------|
| `user_id` | INT | User who initiated/received the request. |
| `friend_id` | INT | The other party in the friendship. |
| `status` | TEXT | 'pending', 'accepted', 'blocked'. |
| `created_at` | DATETIME | Timestamp of connection. |

#### Table: `users` (Profile Cache)
| Column | Type | Description |
|--------|------|-------------|
| `chat_id` | INT (PK) | Telegram ID. |
| `first_name` | TEXT | User's first name. |
| `last_name` | TEXT | User's last name. |
| `username` | TEXT | Telegram @username. |
| `updated_at` | DATETIME | Last profile sync. |

---

## ⚙️ Core Subsystems

### **1. NLP Time Parser (`internal/core/parser.go`)**
- **Core Engine**: Uses `github.com/olebedev/when` with `ru.All` rules for natural language time parsing.
- **Interval Parsing**: Custom regex-based parser supporting both RU (`мин`, `час`, `день`) and EN (`m`, `h`, `d`) units.
- **Past-Time Logic**: If a time is parsed as "in the past" (e.g., "March 25th" when it's already April), the system automatically rolls it forward to the **next year** if it was an explicit date.

### 2. Timezone Engine (`internal/core/timezone.go`)
- **Geocoding**: Uses **OpenStreetMap Nominatim API** to resolve city names to coordinates.
- **Mapping**: Uses `timezonemapper` to convert coordinates to IANA TZ strings.
- **GMT Logic**: Handles `Etc/GMT` sign inversion (e.g., User's `GMT+3` → stored as `Etc/GMT-3`) for IANA compatibility.

### 3. The Scheduler (`internal/core/scheduler.go`)
A robust background loop that:
1. Polls `reminders` every 60 seconds (high-precision durability).
2. Selects items where `notify_at <= now()`.
3. Triggers notifications via the Telegram Bot.
4. Updates `last_message_id` immediately upon successful delivery (checkpointing).
5. Calculates the `next_notify_at` for recurrent tasks using bitmasks.

### 4. Social Interaction (`internal/telegram/friends.go`)
Handles the 3-way handshake for friendship:
1. **Invite**: `t.me/bot?start=invite_<ID>` link generation.
2. **Acceptance**: Targeted callback handling.
3. **Validation**: Every social reminder creation checks mutual friendship status.
