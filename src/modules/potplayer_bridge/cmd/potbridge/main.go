package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"fn-potplayer-bridge/internal/potplayer"
	"fn-potplayer-bridge/internal/protocol"
)

type multiFlag []string

func (m *multiFlag) String() string {
	return strings.Join(*m, ",")
}

func (m *multiFlag) Set(value string) error {
	*m = append(*m, value)
	return nil
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	switch os.Args[1] {
	case "play":
		os.Exit(runPlay(os.Args[2:]))
	default:
		usage()
		os.Exit(2)
	}
}

func runPlay(args []string) int {
	var subtitles multiFlag
	var extraArgs multiFlag

	fs := flag.NewFlagSet("play", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	potplayerPath := fs.String("potplayer", "", "PotPlayer executable path")
	url := fs.String("url", "", "media URL or local file path")
	title := fs.String("title", "", "display title appended with PotPlayer URL title syntax")
	seek := fs.Int64("seek", 0, "initial seek position in seconds")
	interval := fs.Duration("interval", potplayer.DefaultInterval, "progress event interval")
	startupTimeout := fs.Duration("startup-timeout", potplayer.DefaultStartupTimeout, "window bind timeout")
	playlistPath := fs.String("playlist", "", "DPL playlist file path")
	fs.Var(&subtitles, "sub", "subtitle path, can be repeated")
	fs.Var(&extraArgs, "arg", "extra PotPlayer argument, can be repeated")

	if err := fs.Parse(args); err != nil {
		return 2
	}

	events := protocol.NewWriter(os.Stdout)
	req := potplayer.PlayRequest{
		PotPlayerPath:  *potplayerPath,
		URL:            *url,
		Title:          *title,
		SeekSeconds:    *seek,
		SubtitlePaths:  subtitles,
		ExtraArgs:      extraArgs,
		Interval:       *interval,
		StartupTimeout: *startupTimeout,
		PlaylistPath:   *playlistPath,
	}

	return potplayer.PlayAndMonitor(req, events)
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: potbridge play --potplayer <path> [--url <url> | --playlist <path>] [--title <title>] [--seek <seconds>] [--sub <path>]")
}
