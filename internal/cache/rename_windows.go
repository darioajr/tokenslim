package cache

import (
	"errors"
	"os"
	"syscall"
	"time"
)

// Windows readers may temporarily prevent replacing their open file. Retry the
// rename without removing the destination, so a failed write retains the cache.
// A bounded wait also leaves permanent permission errors visible to the caller.
func replaceFile(source, destination string) error {
	deadline := time.Now().Add(2 * time.Second)
	delay := 5 * time.Millisecond
	for {
		err := os.Rename(source, destination)
		const sharingViolation syscall.Errno = 32
		if !errors.Is(err, syscall.ERROR_ACCESS_DENIED) && !errors.Is(err, sharingViolation) {
			return err
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return err
		}
		time.Sleep(min(delay, remaining))
		delay = min(2*delay, 50*time.Millisecond)
	}
}
