//go:build !windows

package cache

import "os"

func replaceFile(source, destination string) error {
	return os.Rename(source, destination)
}
