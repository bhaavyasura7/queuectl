package queue

import (
	"math"
	"time"
)

func NextAttemptAt(now time.Time, backoffBase float64, attempts int) time.Time {
	delay := time.Duration(math.Pow(backoffBase, float64(attempts)) * float64(time.Second))
	return now.Add(delay)
}
