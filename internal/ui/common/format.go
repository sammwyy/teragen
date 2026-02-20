package common

import (
	"fmt"
	"strings"
	"time"
)

func FmtTokens(n int) string {
	if n >= 1000000 {
		return fmt.Sprintf("%.2fM", float64(n)/1000000.0)
	}
	if n >= 1000 {
		val := float64(n) / 1000.0
		if val >= 10.0 {
			return fmt.Sprintf("%.2fK", val)
		}
		return fmt.Sprintf("%.1fK", val)
	}
	return fmt.Sprintf("%d", n)
}

func Clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func EstimateTokens(text string) int {
	words := len(strings.Fields(text))
	if words == 0 {
		return 0
	}
	return int(float64(words)*1.3) + 1
}

func FormatTimestamp(ts string) string {
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return ts
	}
	now := time.Now()
	if t.Year() != now.Year() {
		return t.Format("02/01/2006 15:04")
	}
	if t.YearDay() != now.YearDay() {
		return t.Format("02/01 15:04")
	}
	return t.Format("15:04")
}
