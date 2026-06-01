package potplayer

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"fn-potplayer-bridge/internal/protocol"
)

func TestBuildEpisodeChangedEventFromDPLTitle(t *testing.T) {
	playlist := "\ufeffDAUMPLAYLIST\n" +
		"1*file*http://127.0.0.1:22345/api/v1/playvideo/guid-a?token=x\n" +
		"1*title*测试剧 - S1E1: 第一集\n" +
		"2*file*http://127.0.0.1:22345/api/v1/playvideo/guid-b?token=x\n" +
		"2*title*测试剧 - S1E2: 第二集\n"

	file, err := os.CreateTemp("", "fntv-playlist-*.dpl")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(file.Name())

	if _, err := file.WriteString(playlist); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	entries := readPlaylistEntries(file.Name())
	event := buildEpisodeChangedEvent("测试剧 - S1E2: 第二集 - PotPlayer", entries)

	if event.Index == nil || *event.Index != 1 {
		t.Fatalf("expected index 1, got %#v", event.Index)
	}
	if event.EpisodeID != "guid-b" {
		t.Fatalf("expected guid-b, got %q", event.EpisodeID)
	}
	if event.Message != "测试剧 - S1E2: 第二集" {
		t.Fatalf("expected playlist title, got %q", event.Message)
	}
}

func TestBuildEpisodeChangedEventKeepsZeroIndex(t *testing.T) {
	entries := []playlistEntry{{
		Index:     0,
		Title:     "测试剧 - S1E1: 第一集",
		URL:       "http://127.0.0.1:22345/api/v1/playvideo/guid-a?token=x",
		EpisodeID: "guid-a",
	}}

	event := buildEpisodeChangedEvent("测试剧 - S1E1: 第一集 - PotPlayer", entries)
	if event.Index == nil || *event.Index != 0 {
		t.Fatalf("expected index 0, got %#v", event.Index)
	}
}

func TestHandleStopCommandWritesClosedAndRequestsExit(t *testing.T) {
	var buf bytes.Buffer
	events := protocol.NewWriter(&buf)
	seek := newSeekRetry(0)

	if !handleCommand(0, Command{Action: "stop"}, events, &seek) {
		t.Fatal("expected stop command to request monitor exit")
	}

	if got := strings.TrimSpace(buf.String()); got != `{"type":"closed"}` {
		t.Fatalf("expected closed event, got %q", got)
	}
}

func TestHandleSeekCommandKeepsMonitorRunning(t *testing.T) {
	var buf bytes.Buffer
	events := protocol.NewWriter(&buf)
	seek := newSeekRetry(0)

	if handleCommand(0, Command{Action: "seek", PosMs: 42_000}, events, &seek) {
		t.Fatal("seek command must not request monitor exit")
	}
	if seek.targetMs != 42_000 {
		t.Fatalf("expected targetMs 42000, got %d", seek.targetMs)
	}
	if buf.Len() != 0 {
		t.Fatalf("expected no event for seek command, got %q", buf.String())
	}
}
