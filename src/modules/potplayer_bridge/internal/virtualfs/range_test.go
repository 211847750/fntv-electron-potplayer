package virtualfs

import "testing"

func TestValidateContentRange(t *testing.T) {
	tests := []struct {
		name           string
		contentRange   string
		requestedStart int64
		requestedEnd   int64
		wantEnd        int64
		wantTotal      int64
		wantError      bool
	}{
		{name: "exact", contentRange: "bytes 100-199/1000", requestedStart: 100, requestedEnd: 199, wantEnd: 199, wantTotal: 1000},
		{name: "final partial block", contentRange: "bytes 900-999/1000", requestedStart: 900, requestedEnd: 1099, wantEnd: 999, wantTotal: 1000},
		{name: "wrong start", contentRange: "bytes 0-99/1000", requestedStart: 100, requestedEnd: 199, wantError: true},
		{name: "oversized response", contentRange: "bytes 100-299/1000", requestedStart: 100, requestedEnd: 199, wantError: true},
		{name: "missing header", requestedStart: 100, requestedEnd: 199, wantError: true},
		{name: "invalid total", contentRange: "bytes 100-199/150", requestedStart: 100, requestedEnd: 199, wantError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			end, total, err := validateContentRange(tt.contentRange, tt.requestedStart, tt.requestedEnd)
			if tt.wantError {
				if err == nil {
					t.Fatal("expected validation error")
				}
				return
			}
			if err != nil {
				t.Fatalf("validateContentRange() error = %v", err)
			}
			if end != tt.wantEnd || total != tt.wantTotal {
				t.Fatalf("validateContentRange() = end %d total %d, want end %d total %d", end, total, tt.wantEnd, tt.wantTotal)
			}
		})
	}
}
