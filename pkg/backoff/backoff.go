package backoff

import (
	"fmt"
	"time"
)

// Retry tries a maximum of maxTries times, unless canRetry returns false and
// the error is returned immediately.
func Retry(maxTries int, canRetry func(error) bool, body func() error) error {
	var err error
	sleepDur := 1 * time.Millisecond
	for i := 0; i < maxTries; i++ {
		err = body()
		if err == nil {
			return nil
		}
		if !canRetry(err) {
			return err
		}
		time.Sleep(sleepDur)
		sleepDur *= 2
	}
	return fmt.Errorf("max retries: %w", err)
}
