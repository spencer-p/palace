package prettytime

import (
	"fmt"
	"time"
)

func DurationBetween(now, then time.Time) string {
	d := now.Sub(then)
	if truncateToDay(now).Equal(truncateToDay(then)) {
		return simpleDurationLessThanDay(d)
	}
	return simpleDurationMoreThanDay(d)
}

func simpleDurationLessThanDay(d time.Duration) string {
	switch {
	case d >= time.Hour:
		return fmt.Sprintf("%.0fh", d.Hours())
	case d >= time.Minute:
		return fmt.Sprintf("%.0fm", d.Minutes())
	case d >= time.Second:
		return fmt.Sprintf("%.0fs", d.Seconds())
	default:
		return "0s"
	}
}

func simpleDurationMoreThanDay(d time.Duration) string {
	const day = 24 * time.Hour
	switch {
	case d >= 28*day:
		return fmt.Sprintf("%dmo", d/(28*day))
	case d >= day:
		return fmt.Sprintf("%dd", d/day)
	default:
		return "1d"
	}
}

func truncateToDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}
