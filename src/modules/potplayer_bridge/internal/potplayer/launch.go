package potplayer

import (
	"errors"
	"os/exec"
	"strings"
)

func Launch(req PlayRequest) error {
	if strings.TrimSpace(req.PotPlayerPath) == "" {
		return errors.New("potplayer path is empty")
	}
	if strings.TrimSpace(req.URL) == "" {
		return errors.New("play url is empty")
	}

	args := []string{"/new"}
	if req.SeekSeconds > 0 {
		args = append(args, "/seek="+formatSeek(req.SeekSeconds))
	}
	if len(req.SubtitlePaths) > 0 && strings.TrimSpace(req.SubtitlePaths[0]) != "" {
		args = append(args, "/sub="+req.SubtitlePaths[0])
	}
	args = append(args, req.ExtraArgs...)
	args = append(args, formatURLWithTitle(req.URL, req.Title))

	cmd := exec.Command(req.PotPlayerPath, args...)
	return cmd.Start()
}

func formatURLWithTitle(url, title string) string {
	title = strings.TrimSpace(strings.NewReplacer(`"`, " ", `\`, " ", "\r", " ", "\n", " ").Replace(title))
	if title == "" {
		return url
	}
	return url + `\` + strings.Join(strings.Fields(title), " ")
}

func formatSeek(seconds int64) string {
	if seconds < 0 {
		seconds = 0
	}

	hours := seconds / 3600
	minutes := (seconds % 3600) / 60
	secs := seconds % 60
	return zeroPad2(hours) + ":" + zeroPad2(minutes) + ":" + zeroPad2(secs) + ".000"
}

func zeroPad2(value int64) string {
	if value < 10 {
		return "0" + strconvFormatInt(value)
	}
	return strconvFormatInt(value)
}
