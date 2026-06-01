package potplayer

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"sort"
	"strconv"
	"strings"
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
	seek := newSeekRetry(req.SeekSeconds * 1000)
	ticker := time.NewTicker(req.Interval)
	defer ticker.Stop()

	cmdChan := make(chan Command, 16)
	go readStdinCommands(cmdChan)

	playlistEntries := readPlaylistEntries(req.PlaylistPath)
	var lastTitle string

	for {
		select {
		case cmd := <-cmdChan:
			if handleCommand(hwnd, cmd, events, &seek) {
				return 0
			}
		case <-ticker.C:
			state, err := ReadState(hwnd)
			if err != nil {
				if IsWindowHandle(hwnd) {
					events.Write(protocol.Event{Type: protocol.EventError, Message: err.Error(), PosMs: last.PosMs, DurMs: last.DurMs, Status: last.Status})
					continue
				}

				events.Write(protocol.Event{Type: protocol.EventClosed, PosMs: last.PosMs, DurMs: last.DurMs, Status: -1})
				return 0
			}

			state.Title = readWindowTitle(hwnd)
			seek.Apply(hwnd, state)

			if state.Title != lastTitle && state.Title != "" {
				lastTitle = state.Title
				events.Write(buildEpisodeChangedEvent(state.Title, playlistEntries))
			}

			last = state
			writeStateEvent(events, state)
		}
	}
}

type playlistEntry struct {
	Index     int
	Title     string
	URL       string
	EpisodeID string
}

func readPlaylistEntries(playlistPath string) []playlistEntry {
	if strings.TrimSpace(playlistPath) == "" {
		return nil
	}

	file, err := os.Open(playlistPath)
	if err != nil {
		return nil
	}
	defer file.Close()

	entries := map[int]*playlistEntry{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(strings.TrimPrefix(scanner.Text(), "\ufeff"))
		parts := strings.SplitN(line, "*", 3)
		if len(parts) != 3 {
			continue
		}

		playlistIndex, err := strconv.Atoi(parts[0])
		if err != nil || playlistIndex <= 0 {
			continue
		}

		entry := entries[playlistIndex]
		if entry == nil {
			entry = &playlistEntry{Index: playlistIndex - 1}
			entries[playlistIndex] = entry
		}

		switch parts[1] {
		case "file":
			entry.URL = strings.TrimSpace(parts[2])
			entry.EpisodeID = extractEpisodeID(entry.URL)
		case "title":
			entry.Title = strings.TrimSpace(parts[2])
		}
	}
	if scanner.Err() != nil {
		return nil
	}

	result := make([]playlistEntry, 0, len(entries))
	for _, entry := range entries {
		result = append(result, *entry)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Index < result[j].Index
	})
	return result
}

func extractEpisodeID(rawURL string) string {
	marker := "/playvideo/"
	_, value, ok := strings.Cut(rawURL, marker)
	if !ok {
		return ""
	}

	if end := strings.IndexAny(value, "?/\\"); end >= 0 {
		value = value[:end]
	}
	return strings.TrimSpace(value)
}

func buildEpisodeChangedEvent(windowTitle string, entries []playlistEntry) protocol.Event {
	entry := matchPlaylistEntry(windowTitle, entries)
	if entry == nil {
		return protocol.Event{Type: protocol.EventEpisodeChanged, Message: windowTitle}
	}

	message := entry.Title
	if message == "" {
		message = windowTitle
	}
	index := entry.Index
	return protocol.Event{
		Type:      protocol.EventEpisodeChanged,
		Message:   message,
		Index:     &index,
		EpisodeID: entry.EpisodeID,
	}
}

func matchPlaylistEntry(windowTitle string, entries []playlistEntry) *playlistEntry {
	normalizedTitle := normalizeTitle(windowTitle)
	if normalizedTitle == "" {
		return nil
	}

	for i := range entries {
		entryTitle := normalizeTitle(entries[i].Title)
		if entryTitle != "" && (strings.Contains(normalizedTitle, entryTitle) || strings.Contains(entryTitle, normalizedTitle)) {
			return &entries[i]
		}
	}

	for i := range entries {
		if entries[i].EpisodeID != "" && strings.Contains(windowTitle, entries[i].EpisodeID) {
			return &entries[i]
		}
		if entries[i].URL != "" && strings.Contains(windowTitle, entries[i].URL) {
			return &entries[i]
		}
	}

	return nil
}

func normalizeTitle(title string) string {
	return strings.ToLower(strings.TrimSpace(title))
}

func readStdinCommands(ch chan<- Command) {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var cmd Command
		if err := json.Unmarshal([]byte(line), &cmd); err != nil {
			continue
		}
		ch <- cmd
	}
	_ = scanner.Err()
}

func handleCommand(hwnd uintptr, cmd Command, events *protocol.Writer, seek *seekRetry) bool {
	switch cmd.Action {
	case "next":
		SendNextTrack(hwnd)
	case "previous":
		SendPreviousTrack(hwnd)
	case "seek":
		*seek = newSeekRetry(cmd.PosMs)
	case "loadSubtitle":
		loadSubtitle(hwnd, cmd.Path)
		events.Write(protocol.Event{Type: protocol.EventSubtitleLoaded})
	case "stop":
		events.Write(protocol.Event{Type: protocol.EventClosed})
		return true
	}
	return false
}

func readWindowTitle(hwnd uintptr) string {
	title, err := GetWindowText(hwnd)
	if err != nil {
		return ""
	}
	return title
}

const (
	seekRetryToleranceBefore = 10 * time.Second
	seekRetryMinInterval     = 1500 * time.Millisecond
	seekRetryMaxAttempts     = 3
)

type seekRetry struct {
	targetMs    int64
	until       time.Time
	done        bool
	attempts    int
	lastAttempt time.Time
}

func newSeekRetry(targetMs int64) seekRetry {
	if targetMs <= 0 {
		return seekRetry{done: true}
	}

	return seekRetry{
		targetMs: targetMs,
		until:    time.Now().Add(20 * time.Second),
	}
}

func (s *seekRetry) Apply(hwnd uintptr, state State) {
	if s.done || time.Now().After(s.until) {
		s.done = true
		return
	}

	// PotPlayer may settle on an earlier keyframe, and playback can move past
	// the exact target before the next poll. Treat "near enough or already
	// past target" as success; otherwise retrying pins playback at the intro
	// boundary.
	if state.PosMs >= s.targetMs-int64(seekRetryToleranceBefore/time.Millisecond) {
		s.done = true
		return
	}

	if state.DurMs <= 0 && state.Status == 0 {
		return
	}

	if s.attempts >= seekRetryMaxAttempts {
		s.done = true
		return
	}

	now := time.Now()
	if s.attempts > 0 && now.Sub(s.lastAttempt) < seekRetryMinInterval {
		return
	}

	SendSeek(hwnd, s.targetMs)
	s.attempts++
	s.lastAttempt = now
}

func writeStateEvent(events *protocol.Writer, state State) {
	events.Write(protocol.Event{
		Type:   protocol.EventProgress,
		PosMs:  state.PosMs,
		DurMs:  state.DurMs,
		Status: state.Status,
	})
}

func WaitForWindow(existing map[uintptr]bool, timeout time.Duration) (uintptr, error) {
	startedAt := time.Now()
	deadline := time.Now().Add(timeout)
	fallbackDelay := min(timeout, 3*time.Second)
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
