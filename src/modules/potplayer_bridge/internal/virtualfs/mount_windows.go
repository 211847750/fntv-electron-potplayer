//go:build windows

package virtualfs

import (
	"fmt"

	"github.com/winfsp/cgofuse/fuse"
)

func (v *VFS) Mount(drive string) (<-chan error, func(), error) {
	host := fuse.NewFileSystemHost(v)
	host.SetCapCaseInsensitive(true)
	host.SetCapReaddirPlus(true)

	ready := make(chan error, 1)

	unmount := func() {
		host.Unmount()
	}

	go func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				ready <- fmt.Errorf("mount panic: %v", recovered)
			}
		}()
		if ok := host.Mount(drive, nil); !ok {
			ready <- fmt.Errorf("mount returned false")
		}
	}()

	return ready, unmount, nil
}
