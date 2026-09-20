package cache

import (
	"os"
	"syscall"
)

func openCacheFile(path string) (*os.File, error) {
	name, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: path, Err: err}
	}
	var handle syscall.Handle
	err = retrySharing(func() error {
		var openErr error
		// Allow compatible READ/WRITE/DELETE handles from other processes.
		// This does not guarantee replacement of an open destination:
		// replaceFile may still need to wait until readers close.
		handle, openErr = syscall.CreateFile(name, syscall.GENERIC_READ,
			syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE|syscall.FILE_SHARE_DELETE,
			nil, syscall.OPEN_EXISTING, syscall.FILE_ATTRIBUTE_NORMAL, 0)
		return openErr
	})
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: path, Err: err}
	}
	return os.NewFile(uintptr(handle), path), nil
}
