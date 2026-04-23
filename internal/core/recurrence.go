package core

import (
	"reminder-bot/internal/storage"
	"time"
)

// NextOccurrence calculates the next time a reminder should fire based on its recurrence rules.
// Returns the next notification time and true if a next occurrence was found.
// Returns zero time and false if it's a one-time reminder or no more occurrences are possible.
func NextOccurrence(r storage.Reminder, userLoc *time.Location, now time.Time) (time.Time, bool) {
	if r.Interval == "" && r.Weekdays == 0 {
		return time.Time{}, false
	}

	if r.Interval != "" {
		duration, err := time.ParseDuration(r.Interval)
		if err != nil {
			return time.Time{}, false
		}

		if userLoc == nil {
			userLoc = time.UTC
		}

		// If the duration is exactly a multiple of 24 hours, use AddDate to respect DST
		isDaily := duration > 0 && duration%(24*time.Hour) == 0
		days := int(duration / (24 * time.Hour))

		next := r.NotifyAt.In(userLoc)
		if isDaily {
			for !next.After(now) {
				next = next.AddDate(0, 0, days)
			}
		} else {
			for !next.After(now) {
				next = next.Add(duration)
			}
		}
		return next.UTC(), true
	}

	if r.Weekdays != 0 {
		if userLoc == nil {
			userLoc = time.UTC
		}
		
		nowInLoc := now.In(userLoc)
		origInLoc := r.NotifyAt.In(userLoc)

		// Start from the user's intended time of day, but synchronized to "now" date
		// to check if today is a possibility.
		base := time.Date(nowInLoc.Year(), nowInLoc.Month(), nowInLoc.Day(),
			origInLoc.Hour(), origInLoc.Minute(), origInLoc.Second(), 0, userLoc)

		for i := 0; i <= 7; i++ {
			next := base.AddDate(0, 0, i)
			if !next.After(nowInLoc) {
				continue
			}

			wd := next.Weekday()
			// Mon=1 (shift 0), Tue=2 (shift 1), ..., Sat=6 (shift 5), Sun=0 (shift 6)
			shift := uint(wd) - 1
			if wd == time.Sunday {
				shift = 6
			}

			if (r.Weekdays & (1 << shift)) != 0 {
				return next.UTC(), true
			}
		}
	}

	return time.Time{}, false
}
