//go:build windows

package virtualfs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// FileEntry represents a video and optional subtitle to expose on the virtual drive.
type FileEntry struct {
	Name         string
	VideoURL     string
	VideoHeaders map[string]string
	SubPath      string
	SubExt       string
	Title        string
	Start        bool
	StartSec     int64
	DurationSec  int64
}

// Setup mounts a read-only virtual filesystem and returns a DPL file that points to it.
func Setup(originalDPLPath string, drive string, entries []FileEntry) (vfsDPLPath string, cleanup func(), err error) {
	if drive == "" || len(entries) == 0 {
		return originalDPLPath, func() {}, nil
	}

	normalizedDrive := strings.TrimRight(drive, `\/`)
	vfs := NewVFS()
	vfsEntries := make([]FileEntry, 0, len(entries))

	for i := range entries {
		e := entries[i]
		base := safeBaseName(e.Name)
		if base == "" {
			base = safeBaseName(e.Title)
		}
		if base == "" {
			base = fmt.Sprintf("episode_%02d", i+1)
		}

		videoName := base + videoExtension(e.VideoURL)
		subtitleName := ""
		if e.SubPath != "" {
			subExt := strings.ToLower(e.SubExt)
			if subExt == "" {
				subExt = strings.ToLower(filepath.Ext(e.SubPath))
			}
			if subExt == "" {
				subExt = ".srt"
			}
			subtitleName = base + subExt
		}

		vfs.AddFile(&MediaFile{
			Name:         videoName,
			SubtitleName: subtitleName,
			SubPath:      e.SubPath,
			VideoURL:     e.VideoURL,
			VideoHeaders: e.VideoHeaders,
		})

		e.Name = videoName
		vfsEntries = append(vfsEntries, e)
	}

	ready, unmount, err := vfs.Mount(normalizedDrive)
	if err != nil {
		return "", nil, fmt.Errorf("mount VFS at %s: %w", normalizedDrive, err)
	}
	select {
	case mountErr := <-ready:
		unmount()
		return "", nil, fmt.Errorf("mount VFS at %s: %w", normalizedDrive, mountErr)
	case <-time.After(500 * time.Millisecond):
	}

	dplPath, err := generateDPL(normalizedDrive, vfsEntries)
	if err != nil {
		unmount()
		return "", nil, fmt.Errorf("generate VFS DPL: %w", err)
	}
	fmt.Fprintf(os.Stderr, "[VFS] mounted drive=%s entries=%d dpl=%q\n", normalizedDrive, len(vfsEntries), dplPath)

	cleanup = func() {
		fmt.Fprintf(os.Stderr, "[VFS] unmount drive=%s\n", normalizedDrive)
		unmount()
		os.Remove(dplPath)
	}

	return dplPath, cleanup, nil
}

func generateDPL(drive string, entries []FileEntry) (string, error) {
	tmpDir := filepath.Join(os.TempDir(), "fntv-playlists")
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return "", err
	}

	var sb strings.Builder
	sb.WriteString("\uFEFFDAUMPLAYLIST\n")
	startIndex := 0
	for i := range entries {
		if entries[i].Start {
			startIndex = i
			break
		}
	}

	if len(entries) > 0 {
		sb.WriteString("playname=" + drive + `\` + entries[startIndex].Name + "\n")
		sb.WriteString(fmt.Sprintf("playtime=%d\n", entries[startIndex].StartSec*1000))
	} else {
		sb.WriteString("playtime=0\n")
	}

	for i, e := range entries {
		idx := i + 1
		vpath := drive + `\` + e.Name

		sb.WriteString(fmt.Sprintf("%d*file*%s\n", idx, vpath))
		if e.Title != "" {
			sb.WriteString(fmt.Sprintf("%d*title*%s\n", idx, dplValue(e.Title)))
		}
		sb.WriteString(fmt.Sprintf("%d*played*0\n", idx))
		if e.DurationSec > 0 {
			sb.WriteString(fmt.Sprintf("%d*duration2*%d\n", idx, e.DurationSec))
		}
		if i == startIndex && e.StartSec > 0 {
			sb.WriteString(fmt.Sprintf("%d*start*%d\n", idx, e.StartSec))
		}
	}

	dplPath := filepath.Join(tmpDir, fmt.Sprintf("playlist_vfs_%d.dpl", os.Getpid()))
	if err := os.WriteFile(dplPath, []byte(sb.String()), 0644); err != nil {
		return "", err
	}
	return dplPath, nil
}

func safeBaseName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}

	replacer := strings.NewReplacer(
		"<", " ",
		">", " ",
		":", " ",
		`"`, " ",
		"/", " ",
		`\`, " ",
		"|", " ",
		"?", " ",
		"*", " ",
		"\r", " ",
		"\n", " ",
	)
	value = replacer.Replace(value)
	value = strings.Join(strings.Fields(value), " ")
	if len([]rune(value)) > 120 {
		runes := []rune(value)
		value = string(runes[:120])
	}
	return strings.TrimSpace(value)
}

func dplValue(value string) string {
	return strings.TrimSpace(strings.NewReplacer("\r", " ", "\n", " ").Replace(value))
}
