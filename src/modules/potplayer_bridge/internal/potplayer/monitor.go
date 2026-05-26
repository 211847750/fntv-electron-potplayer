package potplayer

import (
	"errors"
	"time"

	"fn-potplayer-bridge/internal/protocol"
)

func PlayAndMonitor(req PlayRequest, events *protocol.Writer) int {
	if req.Interval <= 0 {
		req.Interval = DefaultInterval
	}
	if req.StartupTimeout <= 0 {
		req.StartupTimeout = DefaultStartupTimeout
	}

	existing, err := PotPlayerWindowSet()
	if err != nil {
		events.Write(protocol.Event{Type: protocol.EventError, Message: err.Error()})
		return 1
	}

	if err := Launch(req); err != nil {
		events.Write(protocol.Event{Type: protocol.EventError, Message: err.Error()})
		return 1
	}

	hwnd, err := WaitForWindow(existing, req.StartupTimeout)
	if err != nil {
		events.Write(protocol.Event{Type: protocol.EventError, Message: err.Error()})
		return 1
	}

	events.Write(protocol.Event{Type: protocol.EventReady, HWND: hwnd})

	var last State
	seek := newInitialSeek(req.SeekSeconds)
	ticker := time.NewTicker(req.Interval)
	defer ticker.Stop()

	for {
		state, err := ReadState(hwnd)
		if err != nil {
			if IsWindowHandle(hwnd) {
				events.Write(protocol.Event{Type: protocol.EventError, Message: err.Error(), PosMs: last.PosMs, DurMs: last.DurMs, Status: last.Status})
				<-ticker.C
				continue
			}

			events.Write(protocol.Event{Type: protocol.EventExit, PosMs: last.PosMs, DurMs: last.DurMs, Status: -1})
			return 0
		}

		last = state
		seek.Apply(hwnd, state)
		writeStateEvent(events, state)
		<-ticker.C
	}
}

type initialSeek struct {
	targetMs int64
	until    time.Time
	done     bool
}

func newInitialSeek(seconds int64) *initialSeek {
	if seconds <= 0 {
		return &initialSeek{done: true}
	}

	return &initialSeek{
		targetMs: seconds * 1000,
		until:    time.Now().Add(20 * time.Second),
	}
}

func (s *initialSeek) Apply(hwnd uintptr, state State) {
	if s.done || time.Now().After(s.until) {
		s.done = true
		return
	}

	if state.PosMs >= s.targetMs-3000 && state.PosMs <= s.targetMs+3000 {
		s.done = true
		return
	}

	if state.DurMs <= 0 && state.Status == 0 {
		return
	}

	SendSeek(hwnd, s.targetMs)
}

func writeStateEvent(events *protocol.Writer, state State) {
	events.Write(protocol.Event{Type: protocol.EventProgress, PosMs: state.PosMs, DurMs: state.DurMs, Status: state.Status})
}

func WaitForWindow(existing map[uintptr]bool, timeout time.Duration) (uintptr, error) {
	startedAt := time.Now()
	deadline := time.Now().Add(timeout)
	fallbackDelay := 3 * time.Second
	if timeout < fallbackDelay {
		fallbackDelay = timeout
	}
	var fallback uintptr

	for time.Now().Before(deadline) {
		windows, err := EnumPotPlayerWindows()
		if err != nil {
			return 0, err
		}

		for _, hwnd := range windows {
			if !existing[hwnd] {
				return hwnd, nil
			}
			if fallback == 0 {
				fallback = hwnd
			}
		}

		if fallback != 0 && time.Since(startedAt) >= fallbackDelay {
			return fallback, nil
		}

		time.Sleep(300 * time.Millisecond)
	}

	if fallback != 0 {
		return fallback, nil
	}

	return 0, errors.New("potplayer window not found")
}

func PotPlayerWindowSet() (map[uintptr]bool, error) {
	handles, err := EnumPotPlayerWindows()
	if err != nil {
		return nil, err
	}

	result := make(map[uintptr]bool, len(handles))
	for _, hwnd := range handles {
		result[hwnd] = true
	}
	return result, nil
}
