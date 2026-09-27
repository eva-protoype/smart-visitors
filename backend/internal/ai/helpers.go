package ai

import (
	"math"
	"time"
)

// IsOffHours mirrors utils/helpers.py is_off_hours (overnight wrap).
func IsOffHours(t time.Time) bool {
	h := t.Hour()
	start, end := OffHourStart, OffHourEnd
	if start <= end {
		return start <= h && h < end
	}
	return h >= start || h < end
}

func HoursBetween(a, b time.Time) float64 {
	return math.Abs(a.Sub(b).Hours())
}

func SafeLimit(n *int, def, max int) int {
	if n == nil {
		return def
	}
	v := *n
	if v < 1 {
		return 1
	}
	if v > max {
		return max
	}
	return v
}

func ClampLimit(n int, def, max int) int {
	if n < 1 {
		if def >= 1 && def <= max {
			// caller passed 0 (missing); use default
			if n == 0 {
				return def
			}
		}
		return 1
	}
	if n > max {
		return max
	}
	return n
}
