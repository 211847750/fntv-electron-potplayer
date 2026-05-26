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
}

type State struct {
	PosMs  int64
	DurMs  int64
	Status int64
}
