package protocol

import (
	"encoding/json"
	"fmt"
	"io"
	"sync"
)

const (
	EventReady          = "ready"
	EventProgress       = "progress"
	EventExit           = "exit"
	EventError          = "error"
	EventEpisodeChanged = "episodeChanged"
	EventSubtitleLoaded = "subtitleLoaded"
	EventClosed         = "closed"
)

type Event struct {
	Type      string  `json:"type"`
	HWND      uintptr `json:"hwnd,omitempty"`
	PosMs     int64   `json:"posMs,omitempty"`
	DurMs     int64   `json:"durMs,omitempty"`
	Status    int64   `json:"status,omitempty"`
	Message   string  `json:"message,omitempty"`
	Index     *int    `json:"index,omitempty"`
	EpisodeID string  `json:"episodeId,omitempty"`
}

type Writer struct {
	mu  sync.Mutex
	out io.Writer
}

func NewWriter(out io.Writer) *Writer {
	return &Writer{out: out}
}

func (w *Writer) Write(event Event) {
	w.mu.Lock()
	defer w.mu.Unlock()

	data, err := json.Marshal(event)
	if err != nil {
		fmt.Fprintf(w.out, `{"type":"error","message":"marshal event failed: %s"}`+"\n", err.Error())
		return
	}
	fmt.Fprintln(w.out, string(data))
}
