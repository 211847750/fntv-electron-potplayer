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

func GetWindowText(_ uintptr) (string, error) {
	return "", errors.New("potplayer bridge only supports Windows")
}

func SendCommand(_ uintptr, _ uintptr) {
}

func SendNextTrack(_ uintptr) {
}

func SendPreviousTrack(_ uintptr) {
}

func loadSubtitle(_ uintptr, _ string) {
}
