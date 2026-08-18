package virtualfs

import (
	"fmt"
	"strconv"
	"strings"
)

func parseContentRange(contentRange string) (int64, int64, int64, error) {
	fields := strings.Fields(contentRange)
	if len(fields) != 2 || fields[0] != "bytes" {
		return 0, 0, 0, fmt.Errorf("invalid Content-Range format: %s", contentRange)
	}

	interval, totalText, ok := strings.Cut(fields[1], "/")
	if !ok {
		return 0, 0, 0, fmt.Errorf("invalid Content-Range format: %s", contentRange)
	}
	startText, endText, ok := strings.Cut(interval, "-")
	if !ok {
		return 0, 0, 0, fmt.Errorf("invalid Content-Range interval: %s", interval)
	}

	start, startErr := strconv.ParseInt(startText, 10, 64)
	end, endErr := strconv.ParseInt(endText, 10, 64)
	total, totalErr := strconv.ParseInt(totalText, 10, 64)
	if startErr != nil || endErr != nil || totalErr != nil || start < 0 || end < start || total <= end {
		return 0, 0, 0, fmt.Errorf("invalid Content-Range values: %s", contentRange)
	}
	return start, end, total, nil
}

func parseContentRangeSize(contentRange string) int64 {
	_, _, total, err := parseContentRange(contentRange)
	if err != nil {
		return 0
	}
	return total
}

func validateContentRange(contentRange string, requestedStart, requestedEnd int64) (int64, int64, error) {
	start, end, total, err := parseContentRange(contentRange)
	if err != nil {
		return 0, 0, err
	}
	if start != requestedStart || end > requestedEnd {
		return 0, 0, fmt.Errorf("unexpected Content-Range %q for bytes=%d-%d", contentRange, requestedStart, requestedEnd)
	}
	return end, total, nil
}
