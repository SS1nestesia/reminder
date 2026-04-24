package core

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/olebedev/when"
	"github.com/olebedev/when/rules/ru"
)

// Sentinel parser errors. Wrap external errors with %w when appropriate.
var (
	ErrInvalidTime = errors.New("parser: invalid time")
	ErrPastTime    = errors.New("parser: time is in the past")
	ErrEmptyInput  = errors.New("parser: empty input")
)

// whenParser is a package-level, read-only NLP engine. It is safe for
// concurrent use: Parse() takes the reference time as an argument and does
// not mutate parser state.
var whenParser = func() *when.Parser {
	p := when.New(nil)
	p.Add(ru.All...)
	return p
}()

// Clock abstracts time.Now() so callers (notably tests) can inject a
// deterministic time source. A nil Clock means use real wall-clock time.
//
// This aligns with PROJECT_OVERVIEW §3 (Temporal-style determinism): core
// business logic should not depend on the global clock.
type Clock interface {
	Now() time.Time
}

type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }

// SystemClock is the default real-wall-clock implementation.
var SystemClock Clock = realClock{}

// Parser handles parsing natural language strings into time or duration
// structures. It is safe for concurrent use as all fields are read-only
// after construction.
type Parser struct {
	Loc   *time.Location
	clock Clock
}

// NewParser returns a Parser bound to loc using the system clock.
// If loc is nil, DefaultLoc is used.
func NewParser(loc *time.Location) *Parser {
	return NewParserWithClock(loc, SystemClock)
}

// NewParserWithClock is the same as NewParser but accepts a custom clock
// for deterministic testing. A nil clock falls back to SystemClock.
func NewParserWithClock(loc *time.Location, clock Clock) *Parser {
	if loc == nil {
		loc = DefaultLoc
	}
	if clock == nil {
		clock = SystemClock
	}
	return &Parser{Loc: loc, clock: clock}
}

// pastTimeThreshold is the cutoff used to decide whether a parsed past
// time is a "recent past" (e.g. "вчера", earlier today) — rejected — or
// an explicit date that has already passed this year and should roll
// forward (e.g. "25 марта" parsed in April). Documented here so tests
// and future maintainers don't have to re-derive it from the math.
const pastTimeThreshold = 48 * time.Hour

// relativeTimeRegex matches friendly relative-time expressions that the
// `when` RU rule-set doesn't cover well: seconds, "+N unit", bare
// "через N unit" with units in seconds.
//
// The parser tries these patterns FIRST, so they always win over the
// generic NLP rules. Group 1 is a sign ("+" or "-"), group 2 is the
// number, group 3 is the unit prefix (letters only; Cyrillic or latin).
var relativeTimeRegex = regexp.MustCompile(
	`(?i)^\s*(?:через\s+|\+)?([+-]?\d+)\s*([а-яёa-z]+)\.?\s*$`,
)

// relativeTimeUnits maps (lowercase, accent-free) prefix → seconds per
// unit for the fast-path relative-time matcher. Uses HasPrefix so short
// forms ("с", "сек", "минут") share entries. Ordered longest-first when
// iterated via `relativeTimeUnitKeys`.
var relativeTimeUnits = map[string]int64{
	"секунд": 1, "секунды": 1, "секунда": 1, "сек": 1, "с": 1, "s": 1,
	"минут": 60, "минуты": 60, "минута": 60, "мин": 60, "м": 60, "m": 60,
	"час": 3600, "часа": 3600, "часов": 3600, "ч": 3600, "h": 3600,
	"день": 86400, "дня": 86400, "дней": 86400, "д": 86400, "d": 86400,
	"недел": 7 * 86400, "недели": 7 * 86400, "неделя": 7 * 86400,
}

// relativeTimeUnitKeys holds the keys of relativeTimeUnits sorted
// longest-first so prefix lookups ("минут" is tried before "м") work.
var relativeTimeUnitKeys = sortedKeysDesc(relativeTimeUnits)

func sortedKeysDesc[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	// insertion sort by length desc — the map is tiny (<20 keys).
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && len(keys[j]) > len(keys[j-1]); j-- {
			keys[j], keys[j-1] = keys[j-1], keys[j]
		}
	}
	return keys
}

// matchRelativeDuration tries to interpret the input as a pure relative
// duration expression (e.g. "через 30 секунд", "+2 часа", "-15 мин").
// Returns the duration and ok=true on success.
func matchRelativeDuration(input string) (time.Duration, bool) {
	m := relativeTimeRegex.FindStringSubmatch(input)
	if len(m) != 3 {
		return 0, false
	}
	n, err := strconv.ParseInt(m[1], 10, 64)
	if err != nil {
		return 0, false
	}
	unitStr := strings.ToLower(strings.ReplaceAll(m[2], "ё", "е"))
	for _, key := range relativeTimeUnitKeys {
		if strings.HasPrefix(unitStr, key) {
			return time.Duration(n) * time.Duration(relativeTimeUnits[key]) * time.Second, true
		}
	}
	return 0, false
}

// ParseTime parses a natural-language time expression in Russian/English.
// Results are always returned in UTC for storage consistency.
//
// Semantics:
//   - A time parsed in the near past (≤ 48h ago) is rejected with ErrPastTime
//     because the user likely meant a future time they mis-typed.
//   - A time parsed further in the past is assumed to be an explicit date
//     that has already occurred this calendar year and is rolled forward
//     by one year (e.g. "25 марта" in April → March 25 next year).
//
// The parser recognises (in order of preference):
//   1. Relative duration shortcuts — "через 30 секунд", "+1ч", "-15 мин",
//      "30 минут" — resolved to `now + duration` (also accepts seconds
//      which the `when` RU rules don't handle).
//   2. Natural-language dates/times via the `when` library (RU rules):
//      "завтра в 15:04", "в пятницу в 9:00", "25 марта в 14:30".
func (p *Parser) ParseTime(input string) (time.Time, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return time.Time{}, ErrEmptyInput
	}
	now := p.clock.Now().In(p.Loc)

	// Fast-path: relative duration ("через 30 секунд", "+1 час").
	// This covers seconds, which the `when` RU rules do not handle, and
	// gives a single canonical route for "through"/"+N" expressions.
	if d, ok := matchRelativeDuration(input); ok {
		t := now.Add(d).UTC()
		nowUTC := p.clock.Now().UTC()
		if !t.After(nowUTC) {
			return time.Time{}, ErrPastTime
		}
		return t, nil
	}

	r, err := whenParser.Parse(input, now)
	if err != nil {
		return time.Time{}, fmt.Errorf("%w: %v", ErrInvalidTime, err)
	}
	if r == nil {
		return time.Time{}, ErrInvalidTime
	}
	t := r.Time.UTC()
	nowUTC := p.clock.Now().UTC()
	if t.Before(nowUTC) {
		if nowUTC.Sub(t) > pastTimeThreshold {
			tNextYear := t.AddDate(1, 0, 0)
			if tNextYear.After(nowUTC) {
				return tNextYear, nil
			}
		}
		return time.Time{}, ErrPastTime
	}
	return t, nil
}

// intervalUnits maps Russian/English unit prefixes to their value in
// minutes. Map is built once at package init for efficiency.
var intervalUnits = map[string]int64{
	"мин": 1, "минута": 1, "минуты": 1, "минут": 1, "m": 1,
	"час": 60, "часа": 60, "часов": 60, "ч": 60, "h": 60,
	"ден": 1440, "дня": 1440, "дней": 1440, "день": 1440, "д": 1440, "d": 1440,
	"недел": 10080, "неделя": 10080, "недели": 10080, "w": 10080,
}

var intervalRegex = regexp.MustCompile(`(?i)(\d+)\s*([а-яёa-z]+)\.?`)

// intervalShortcuts are fixed phrases that expand to a preset duration.
// They are tried BEFORE the numeric regex and are case-insensitive after
// trimming punctuation.
var intervalShortcuts = map[string]time.Duration{
	"полчаса":         30 * time.Minute,
	"пол часа":        30 * time.Minute,
	"пол-часа":        30 * time.Minute,
	"полтора часа":    90 * time.Minute,
	"полтора":         90 * time.Minute,
	"полдня":          12 * time.Hour,
	"пол дня":         12 * time.Hour,
	"пол-дня":         12 * time.Hour,
	"сутки":           24 * time.Hour,
	"день":            24 * time.Hour,
	"неделя":          7 * 24 * time.Hour,
	"неделю":          7 * 24 * time.Hour,
	"месяц":           30 * 24 * time.Hour, // approximate — recurring reminders fire on fixed cadence
	"полумесяц":       15 * 24 * time.Hour,
	"ежедневно":       24 * time.Hour,
	"ежечасно":        time.Hour,
	"каждую минуту":   time.Minute,
	"каждые полчаса":  30 * time.Minute,
	"каждый день":     24 * time.Hour,
	"каждую неделю":   7 * 24 * time.Hour,
}

// normalizeIntervalInput lower-cases, folds ё→е, trims whitespace and
// common trailing punctuation so shortcut lookups are forgiving.
func normalizeIntervalInput(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, "ё", "е")
	s = strings.TrimRight(s, ".,!? ")
	return s
}

// ParseInterval parses a duration string in Go format ("24h"), Russian/
// English natural language ("2 часа", "3 дня", "1ч 30мин"), or a common
// fixed phrase ("полчаса", "сутки", "каждый день").
// Returns the canonical Go duration string (e.g. "2h0m0s") on success.
func (p *Parser) ParseInterval(input string) (string, error) {
	normalized := normalizeIntervalInput(input)
	if normalized == "" {
		return "", ErrEmptyInput
	}

	// Shortcut phrases — tried first so "полчаса" beats any regex match.
	if d, ok := intervalShortcuts[normalized]; ok {
		return d.String(), nil
	}

	if d, err := time.ParseDuration(normalized); err == nil && d >= 1*time.Minute {
		return d.String(), nil
	}

	matches := intervalRegex.FindAllStringSubmatch(normalized, -1)
	if len(matches) == 0 {
		return "", fmt.Errorf("parser: could not parse interval %q", input)
	}

	var totalMinutes int64
	for _, m := range matches {
		val, err := strconv.ParseInt(m[1], 10, 64)
		if err != nil {
			return "", fmt.Errorf("parser: invalid number %q: %w", m[1], err)
		}
		unitStr := m[2]

		found := false
		for k, multiplier := range intervalUnits {
			if strings.HasPrefix(unitStr, k) {
				totalMinutes += val * multiplier
				found = true
				break
			}
		}
		if !found {
			return "", fmt.Errorf("parser: unknown unit %q", unitStr)
		}
	}

	if totalMinutes < 1 {
		return "", fmt.Errorf("parser: interval too short (<1 minute)")
	}

	return (time.Duration(totalMinutes) * time.Minute).String(), nil
}
