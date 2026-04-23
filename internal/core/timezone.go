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

var offsetRegex = regexp.MustCompile(`^(?:UTC|GMT)?\s*([+-]?)\s*(\d{1,2})$`)

func ParseTimezoneAlias(input string) string {
	input = strings.ToUpper(strings.TrimSpace(input))

	aliases := map[string]string{
		"MSK": "Europe/Moscow",
		"МСК": "Europe/Moscow",
		"EKT": "Asia/Yekaterinburg",
		"ЕКБ": "Asia/Yekaterinburg",
		"UTC": "UTC",
		"GMT": "UTC",
	}
	if mapped, ok := aliases[input]; ok {
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

func GetTimezoneName(lat, lon float64) string {
	return timezonemapper.LatLngToTimezoneString(lat, lon)
}

// FormatTimezone returns a user-friendly string for the timezone, e.g. "MSK (Europe/Moscow)"
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
	
	// If the abbreviation is a cryptic offset like +03, try to find a better one
	// Or just return the abbreviation with the IANA name
	if strings.ContainsAny(abbr, "+-") && len(abbr) <= 5 {
		return tz
	}

	return fmt.Sprintf("%s (%s)", abbr, tz)
}

// GeocodeCity uses Nominatim to resolve a city name into latitude and longitude.
func GeocodeCity(ctx context.Context, city string) (lat, lon float64, err error) {
	apiURL := fmt.Sprintf("https://nominatim.openstreetmap.org/search?q=%s&format=json&limit=1", url.QueryEscape(city))
	
	req, _ := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	req.Header.Set("User-Agent", "ReminderBot/1.0")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return 0, 0, err
	}
	defer resp.Body.Close()

	var result []struct {
		Lat string `json:"lat"`
		Lon string `json:"lon"`
	}
	
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, 0, err
	}

	if len(result) == 0 {
		return 0, 0, errors.New("city not found")
	}

	lat, err = strconv.ParseFloat(result[0].Lat, 64)
	if err != nil {
		return 0, 0, err
	}
	lon, err = strconv.ParseFloat(result[0].Lon, 64)
	if err != nil {
		return 0, 0, err
	}

	return lat, lon, nil
}
