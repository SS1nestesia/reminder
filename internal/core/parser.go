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

// ParseTime parses a natural-language time expression in Russian/English.
// Results are always returned in UTC for storage consistency.
//
// Semantics:
//   - A time parsed in the near past (≤ 48h ago) is rejected with ErrPastTime
//     because the user likely meant a future time they mis-typed.
//   - A time parsed further in the past is assumed to be an explicit date
//     that has already occurred this calendar year and is rolled forward
//     by one year (e.g. "25 марта" in April → March 25 next year).
func (p *Parser) ParseTime(input string) (time.Time, error) {
	if strings.TrimSpace(input) == "" {
		return time.Time{}, ErrEmptyInput
	}
	now := p.clock.Now().In(p.Loc)
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
}

var intervalRegex = regexp.MustCompile(`(\d+)\s*([а-яa-z]+)`)

// ParseInterval parses a duration string in Go format ("24h") or
// Russian/English natural language ("2 часа", "3 дня", "1ч 30мин").
// Returns the canonical duration string (e.g. "2h0m0s") on success.
func (p *Parser) ParseInterval(input string) (string, error) {
	input = strings.ToLower(strings.TrimSpace(input))
	if input == "" {
		return "", ErrEmptyInput
	}

	if d, err := time.ParseDuration(input); err == nil && d >= 1*time.Minute {
		return d.String(), nil
	}

	matches := intervalRegex.FindAllStringSubmatch(input, -1)
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

