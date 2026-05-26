//go:build !windows

package potplayer

import "errors"

func EnumPotPlayerWindows() ([]uintptr, error) {
	return nil, errors.New("potplayer bridge only supports Windows")
}

func IsWindowHandle(_ uintptr) bool {
	return false
}

func ReadState(_ uintptr) (State, error) {
	return State{}, errors.New("potplayer bridge only supports Windows")
}

func SendSeek(_ uintptr, _ int64) {
}
