package potplayer

import "testing"

func TestFormatSeek(t *testing.T) {
	cases := map[int64]string{
		0:    "00:00:00.000",
		1:    "00:00:01.000",
		61:   "00:01:01.000",
		3723: "01:02:03.000",
	}

	for input, expected := range cases {
		if actual := formatSeek(input); actual != expected {
			t.Fatalf("formatSeek(%d) = %q, want %q", input, actual, expected)
		}
	}
}
