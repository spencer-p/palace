package prettytime

import (
	"testing"
	"time"
)

func TestDurationBetween(t *testing.T) {
	now := time.Date(2024, 04, 01, 15, 35, 0, 0, time.UTC)
	table := []struct {
		then time.Time
		want string
	}{{
		then: now.Add(-1 * time.Nanosecond),
		want: "0s",
	}, {
		then: now.Add(-1 * time.Second),
		want: "1s",
	}, {
		then: now.Add(-10 * time.Minute),
		want: "10m",
	}, {
		then: now.Add(-10 * time.Hour),
		want: "10h",
	}, {
		// Just barely in the previous day.
		then: truncateToDay(now).Add(-1 * time.Minute),
		want: "1d",
	}, {
		then: now.Add(-2 * 24 * time.Hour),
		want: "2d",
	}, {
		then: now.Add(-7 * 24 * time.Hour),
		want: "7d",
	}, {
		then: now.Add(-29 * 24 * time.Hour),
		want: "1mo",
	}}

	for _, tc := range table {
		t.Run(tc.want, func(t *testing.T) {
			if got := DurationBetween(now, tc.then); got != tc.want {
				t.Errorf("DuartionBetween(%q, %q) returned %q, wanted %q",
					now, tc.then, got, tc.want)
			}
		})
	}
}
