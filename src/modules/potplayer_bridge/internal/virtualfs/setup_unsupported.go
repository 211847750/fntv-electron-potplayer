//go:build !windows

package virtualfs

import "errors"

type FileEntry struct {
	Name         string
	VideoURL     string
	VideoHeaders map[string]string
	SubPath      string
	SubExt       string
	Title        string
	Start        bool
	StartSec     int64
	DurationSec  int64
}

func Setup(originalDPLPath string, drive string, entries []FileEntry) (string, func(), error) {
	if drive == "" || len(entries) == 0 {
		return originalDPLPath, func() {}, nil
	}
	return "", nil, errors.New("virtual filesystem only supports Windows")
}
