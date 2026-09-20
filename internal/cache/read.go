package cache

import "io"

func readCacheFile(path string) ([]byte, error) {
	f, err := openCacheFile(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(f)
}
