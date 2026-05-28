package potplayer

import "time"

const (
	DefaultInterval       = 5 * time.Second
	DefaultStartupTimeout = 30 * time.Second
)

type PlayRequest struct {
	PotPlayerPath  string
	URL            string
	Title          string
	SeekSeconds    int64
	SubtitlePaths  []string
	ExtraArgs      []string
	Interval       time.Duration
	StartupTimeout time.Duration
	PlaylistPath   string
}

type State struct {
	PosMs  int64
	DurMs  int64
	Status int64
	Title  string
}

type Command struct {
	Action string `json:"command"`
	Index  int    `json:"index,omitempty"`
	PosMs  int64  `json:"posMs,omitempty"`
	Path   string `json:"path,omitempty"`
}

type PlaylistItem struct {
	Index            int     `json:"index"`
	EpisodeID        string  `json:"episodeId"`
	Title            string  `json:"title"`
	URL              string  `json:"url"`
	SubtitlePath     string  `json:"subtitlePath,omitempty"`
	HistoryProgressSec int64 `json:"historyProgressSec,omitempty"`
	IntroEndSec      int64   `json:"introEndSec,omitempty"`
	OutroStartSec    int64   `json:"outroStartSec,omitempty"`
}
