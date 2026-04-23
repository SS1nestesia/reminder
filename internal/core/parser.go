package core

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/olebedev/when"
	"github.com/olebedev/when/rules/ru"
)

var (
	ErrInvalidTime = fmt.Errorf("parser: invalid time")
	ErrPastTime    = fmt.Errorf("parser: time is in the past")

	whenParser = func() *when.Parser {
		p := when.New(nil)
		p.Add(ru.All...)
		return p
	}()
)

// Parser handles parsing natural language strings into time or duration structures
type Parser struct {
	Loc *time.Location
}

func NewParser(loc *time.Location) *Parser {
	if loc == nil {
		loc = DefaultLoc
	}
	return &Parser{Loc: loc}
}

func (p *Parser) ParseTime(input string) (time.Time, error) {
	now := time.Now().In(p.Loc)
	r, err := whenParser.Parse(input, now)
	if err != nil || r == nil {
		return time.Time{}, ErrInvalidTime
	}
	t := r.Time.UTC()
	nowUTC := time.Now().UTC()
	if t.Before(nowUTC) {
		// If the parsed time is more than 48h in the past, it's likely an explicit date
		// (e.g. "25 марта в 14:30") that has already passed this year → try next year.
		// For recent past ("вчера", "15:00" earlier today), the gap is ≤48h, so we reject.
		if nowUTC.Sub(t) > 48*time.Hour {
			tNextYear := t.AddDate(1, 0, 0)
			if tNextYear.After(nowUTC) {
				return tNextYear, nil
			}
		}
		return time.Time{}, ErrPastTime
	}
	return t, nil
}

func (p *Parser) ParseInterval(input string) (string, error) {
	input = strings.ToLower(strings.TrimSpace(input))
	if input == "" {
		return "", fmt.Errorf("empty input")
	}

	if d, err := time.ParseDuration(input); err == nil && d >= 1*time.Minute {
		return d.String(), nil
	}

	var totalMinutes int64
	units := map[string]int64{
		"мин": 1, "минута": 1, "минуты": 1, "минут": 1, "m": 1,
		"час": 60, "часа": 60, "часов": 60, "ч": 60, "h": 60,
		"ден": 1440, "дня": 1440, "дней": 1440, "день": 1440, "д": 1440, "d": 1440,
	}

	re := regexp.MustCompile(`(\d+)\s*([а-яa-z]+)`)
	matches := re.FindAllStringSubmatch(input, -1)

	if len(matches) == 0 {
		return "", fmt.Errorf("could not parse interval")
	}

	for _, m := range matches {
		val, _ := strconv.ParseInt(m[1], 10, 64)
		unitStr := m[2]

		found := false
		for k, multiplier := range units {
			if strings.HasPrefix(unitStr, k) {
				totalMinutes += val * multiplier
				found = true
				break
			}
		}
		if !found {
			return "", fmt.Errorf("unknown unit: %s", unitStr)
		}
	}

	if totalMinutes < 1 {
		return "", fmt.Errorf("interval too short")
	}

	return (time.Duration(totalMinutes) * time.Minute).String(), nil
}

