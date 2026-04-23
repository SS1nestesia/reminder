package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/zsefvlol/timezonemapper"
)

// offsetRegex matches UTC/GMT offsets such as "UTC+3", "GMT -5", "+7", "-10".
var offsetRegex = regexp.MustCompile(`^(?:UTC|GMT)?\s*([+-]?)\s*(\d{1,2})$`)

// timezoneAliases maps the common user-facing aliases to their canonical
// IANA names. Populated once at package init — do not mutate at runtime.
var timezoneAliases = map[string]string{
	"MSK": "Europe/Moscow",
	"МСК": "Europe/Moscow",
	"EKT": "Asia/Yekaterinburg",
	"ЕКБ": "Asia/Yekaterinburg",
	"UTC": "UTC",
	"GMT": "UTC",
}

// geocodeHTTPClient is the shared HTTP client used by GeocodeCity.
// Re-using a client preserves keep-alive connections to Nominatim and
// avoids spawning a new transport/DNS resolver per request.
var geocodeHTTPClient = &http.Client{Timeout: 10 * time.Second}

// ParseTimezoneAlias resolves a common timezone alias (e.g. "MSK", "UTC+3")
// into its canonical IANA string. Returns "" if the input does not match
// any known alias or offset pattern.
func ParseTimezoneAlias(input string) string {
	input = strings.ToUpper(strings.TrimSpace(input))

	if mapped, ok := timezoneAliases[input]; ok {
		return mapped
	}

	matches := offsetRegex.FindStringSubmatch(input)
	if len(matches) == 3 {
		sign := matches[1]
		hours := matches[2]
		if sign == "" {
			sign = "+" // Default is positive
		}

		// IANA Etc/GMT offsets have INVERTED signs!
		// GMT+3 (Ahead of UTC) -> Etc/GMT-3
		etcSign := "-"
		if sign == "-" {
			etcSign = "+"
		}
		return fmt.Sprintf("Etc/GMT%s%s", etcSign, hours)
	}

	return ""
}

// GetTimezoneName returns the IANA timezone name for the given latitude and
// longitude, using an in-memory polygon map (no network calls).
func GetTimezoneName(lat, lon float64) string {
	return timezonemapper.LatLngToTimezoneString(lat, lon)
}

// FormatTimezone returns a user-friendly string for the timezone,
// e.g. "MSK (Europe/Moscow)". On unknown/invalid input the raw tz string
// is returned as-is.
func FormatTimezone(tz string) string {
	if tz == "" {
		return ""
	}

	loc, err := time.LoadLocation(tz)
	if err != nil {
		return tz
	}

	// Get abbreviation for now
	abbr, _ := time.Now().In(loc).Zone()

	// If the abbreviation is a cryptic offset like +03, prefer the
	// IANA name alone over something like "+03 (Etc/GMT-3)".
	if strings.ContainsAny(abbr, "+-") && len(abbr) <= 5 {
		return tz
	}

	return fmt.Sprintf("%s (%s)", abbr, tz)
}

// GeocodeCity uses Nominatim to resolve a city name into latitude and longitude.
// Network errors and JSON-decode errors are wrapped with %w for chain inspection.
func GeocodeCity(ctx context.Context, city string) (lat, lon float64, err error) {
	apiURL := fmt.Sprintf("https://nominatim.openstreetmap.org/search?q=%s&format=json&limit=1", url.QueryEscape(city))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return 0, 0, fmt.Errorf("geocode: build request: %w", err)
	}
	req.Header.Set("User-Agent", "ReminderBot/1.0")

	resp, err := geocodeHTTPClient.Do(req)
	if err != nil {
		return 0, 0, fmt.Errorf("geocode: do request: %w", err)
	}
	defer resp.Body.Close()

	var result []struct {
		Lat string `json:"lat"`
		Lon string `json:"lon"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, 0, fmt.Errorf("geocode: decode response: %w", err)
	}

	if len(result) == 0 {
		return 0, 0, errors.New("geocode: city not found")
	}

	lat, err = strconv.ParseFloat(result[0].Lat, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("geocode: parse lat: %w", err)
	}
	lon, err = strconv.ParseFloat(result[0].Lon, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("geocode: parse lon: %w", err)
	}

	return lat, lon, nil
}
