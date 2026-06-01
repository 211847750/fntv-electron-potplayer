//go:build windows

package potplayer

import (
	"errors"
	"path/filepath"
	"syscall"
	"unsafe"
)

const (
	wmUser           = 0x0400
	potGetTotalTime  = 0x5002
	potGetCurrent    = 0x5004
	potSetCurrent    = 0x5005
	potGetPlayStatus = 0x5006
)

var (
	user32                    = syscall.NewLazyDLL("user32.dll")
	kernel32                  = syscall.NewLazyDLL("kernel32.dll")
	shell32                   = syscall.NewLazyDLL("shell32.dll")
	procEnumWindows           = user32.NewProc("EnumWindows")
	procIsWindowVisible       = user32.NewProc("IsWindowVisible")
	procGetClassNameW         = user32.NewProc("GetClassNameW")
	procGetWindowTextW        = user32.NewProc("GetWindowTextW")
	procGetWindowTextLengthW  = user32.NewProc("GetWindowTextLengthW")
	procSendMessageW          = user32.NewProc("SendMessageW")
	procIsWindow              = user32.NewProc("IsWindow")
	procDragFinish            = shell32.NewProc("DragFinish")
	procGlobalAlloc           = kernel32.NewProc("GlobalAlloc")
	procGlobalLock            = kernel32.NewProc("GlobalLock")
	procGlobalUnlock          = kernel32.NewProc("GlobalUnlock")
	procGlobalFree            = kernel32.NewProc("GlobalFree")
	errEnumWindowsUnavailable = errors.New("EnumWindows unavailable")
)

func EnumPotPlayerWindows() ([]uintptr, error) {
	var windows []uintptr

	callback := syscall.NewCallback(func(hwnd uintptr, _ uintptr) uintptr {
		if !IsWindowVisible(hwnd) {
			return 1
		}

		className, err := GetClassName(hwnd)
		if err == nil && (className == "PotPlayer64" || className == "PotPlayer") {
			windows = append(windows, hwnd)
		}

		return 1
	})

	ret, _, err := procEnumWindows.Call(callback, 0)
	if ret == 0 {
		if err != syscall.Errno(0) {
			return nil, err
		}
		return nil, errEnumWindowsUnavailable
	}

	return windows, nil
}

func IsWindowVisible(hwnd uintptr) bool {
	ret, _, _ := procIsWindowVisible.Call(hwnd)
	return ret != 0
}

func IsWindowHandle(hwnd uintptr) bool {
	ret, _, _ := procIsWindow.Call(hwnd)
	return ret != 0
}

func GetClassName(hwnd uintptr) (string, error) {
	buf := make([]uint16, 256)
	ret, _, err := procGetClassNameW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if ret == 0 {
		if err != syscall.Errno(0) {
			return "", err
		}
		return "", errors.New("GetClassName returned empty class")
	}

	return syscall.UTF16ToString(buf[:ret]), nil
}

func ReadState(hwnd uintptr) (State, error) {
	if hwnd == 0 || !IsWindowHandle(hwnd) {
		return State{}, errors.New("potplayer window is closed")
	}

	return State{
		PosMs:  sendMessage(hwnd, wmUser, potGetCurrent, 0),
		DurMs:  sendMessage(hwnd, wmUser, potGetTotalTime, 0),
		Status: sendMessage(hwnd, wmUser, potGetPlayStatus, 0),
	}, nil
}

func sendMessage(hwnd uintptr, msg uintptr, wParam uintptr, lParam uintptr) int64 {
	ret, _, _ := procSendMessageW.Call(hwnd, msg, wParam, lParam)
	return int64(ret)
}

func SendSeek(hwnd uintptr, posMs int64) {
	if hwnd == 0 || posMs <= 0 || !IsWindowHandle(hwnd) {
		return
	}

	sendMessage(hwnd, wmUser, potSetCurrent, uintptr(posMs))
}

func GetWindowText(hwnd uintptr) (string, error) {
	if hwnd == 0 || !IsWindowHandle(hwnd) {
		return "", errors.New("invalid window handle")
	}

	length, _, _ := procGetWindowTextLengthW.Call(hwnd)
	if length == 0 {
		return "", nil
	}

	buf := make([]uint16, length+1)
	ret, _, _ := procGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if ret == 0 {
		return "", errors.New("GetWindowText failed")
	}

	return syscall.UTF16ToString(buf[:ret]), nil
}

const (
	wmCommand           = 0x0111
	potPreviousTrack    = 10123
	potNextTrack        = 10124
	potNextPlaylistItem = 10068
)

func SendCommand(hwnd uintptr, command uintptr) {
	if hwnd == 0 || !IsWindowHandle(hwnd) {
		return
	}

	sendMessage(hwnd, wmCommand, command, 0)
}

func SendNextTrack(hwnd uintptr) {
	SendCommand(hwnd, potNextTrack)
}

func SendPreviousTrack(hwnd uintptr) {
	SendCommand(hwnd, potPreviousTrack)
}

const wmDropFiles = 0x0233

type dropfiles struct {
	pFiles uint32
	pt     struct{ x, y int32 }
	fNC    int32
	fWide  uint32
}

func loadSubtitle(hwnd uintptr, subPath string) {
	if hwnd == 0 || !IsWindowHandle(hwnd) || subPath == "" {
		return
	}
	absPath, err := filepath.Abs(subPath)
	if err != nil {
		return
	}

	utf16Path, _ := syscall.UTF16FromString(absPath)
	pathBytes := len(utf16Path) * 2
	totalSize := unsafe.Sizeof(dropfiles{}) + uintptr(pathBytes)

	hMem, _, _ := procGlobalAlloc.Call(0x0042, totalSize)
	if hMem == 0 {
		return
	}

	p, _, _ := procGlobalLock.Call(hMem)
	if p == 0 {
		procGlobalFree.Call(hMem)
		return
	}

	df := (*dropfiles)(unsafe.Pointer(p))
	df.pFiles = uint32(unsafe.Sizeof(dropfiles{}))
	df.fWide = 1

	dst := (*[1 << 20]uint16)(unsafe.Pointer(p + unsafe.Sizeof(dropfiles{})))
	copy(dst[:len(utf16Path)], utf16Path)

	procGlobalUnlock.Call(hMem)
	sendMessage(hwnd, wmDropFiles, hMem, 0)
	procDragFinish.Call(hMem)
}
