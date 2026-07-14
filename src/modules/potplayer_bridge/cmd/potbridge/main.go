package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"fn-potplayer-bridge/internal/potplayer"
	"fn-potplayer-bridge/internal/protocol"
	"fn-potplayer-bridge/internal/virtualfs"
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
	virtualDrive := fs.String("virtual-drive", "", "WinFsp virtual drive letter (e.g. V:)")
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
		VirtualDrive:   *virtualDrive,
	}
	if *virtualDrive != "" && *playlistPath != "" {
		entries := potplayer.ReadPlaylistEntries(*playlistPath)
		vfsEntries := make([]virtualfs.FileEntry, 0, len(entries))
		for i, e := range entries {
			fe := virtualfs.FileEntry{
				Name:        e.Title,
				VideoURL:    e.URL,
				Title:       e.Title,
				Start:       e.Start,
				StartSec:    e.StartSec,
				DurationSec: e.Duration,
			}
			if fe.Name == "" {
				fe.Name = fmt.Sprintf("episode_%02d", i+1)
			}
			if i < len(subtitles) {
				fe.SubPath = subtitles[i]
				fe.SubExt = filepath.Ext(subtitles[i])
			}
			vfsEntries = append(vfsEntries, fe)
		}

		vfsDPL, cleanup, err := virtualfs.Setup(*playlistPath, *virtualDrive, vfsEntries)
		if err != nil {
			fmt.Fprintf(os.Stderr, "VFS error: %v\n", err)
			return 1
		}
		defer cleanup()
		req.PlaylistPath = vfsDPL
		req.SubtitlePaths = nil
	}

	return potplayer.PlayAndMonitor(req, events)
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: potbridge play --potplayer <path> [--url <url> | --playlist <path>] [--virtual-drive <X:>] [--sub <path>]...")
}
