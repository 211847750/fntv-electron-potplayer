package utils

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestParseRange(t *testing.T) {
	tests := []struct {
		value     string
		wantStart int64
		wantEnd   int64
		wantError bool
	}{
		{value: "", wantStart: 0, wantEnd: -1},
		{value: "bytes=10-19", wantStart: 10, wantEnd: 19},
		{value: "bytes=10-", wantStart: 10, wantEnd: -1},
		{value: "bytes=-10", wantError: true},
		{value: "bytes=20-10", wantError: true},
		{value: "bytes=0-1,4-5", wantError: true},
	}

	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			got, err := parseRange(tt.value)
			if tt.wantError {
				if err == nil {
					t.Fatal("expected parse error")
				}
				return
			}
			if err != nil {
				t.Fatalf("parseRange() error = %v", err)
			}
			if got.Start != tt.wantStart || got.End != tt.wantEnd {
				t.Fatalf("parseRange() = %d-%d, want %d-%d", got.Start, got.End, tt.wantStart, tt.wantEnd)
			}
		})
	}
}

func TestCloudStorageHandlerReturnsRequestedRange(t *testing.T) {
	gin.SetMode(gin.TestMode)
	content := make([]byte, 1024)
	for i := range content {
		content[i] = byte(i % 251)
	}

	var (
		mu     sync.Mutex
		ranges []string
	)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rangeHeader := r.Header.Get("Range")
		mu.Lock()
		ranges = append(ranges, rangeHeader)
		mu.Unlock()

		rr, err := parseRange(rangeHeader)
		if err != nil || rr.End < rr.Start {
			http.Error(w, "bad range", http.StatusRequestedRangeNotSatisfiable)
			return
		}
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", rr.Start, rr.End, len(content)))
		w.Header().Set("Content-Length", strconv.FormatInt(rr.End-rr.Start+1, 10))
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write(content[rr.Start : rr.End+1])
	}))
	defer upstream.Close()

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/video", nil)
	c.Request.Header.Set("Range", "bytes=100-199")
	NewCloudStorageHandler(upstream.URL, nil, false).HandleRequest(c)

	if recorder.Code != http.StatusPartialContent {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusPartialContent, recorder.Body.String())
	}
	if got := recorder.Header().Get("Content-Range"); got != "bytes 100-199/1024" {
		t.Fatalf("Content-Range = %q", got)
	}
	if got := recorder.Body.Bytes(); string(got) != string(content[100:200]) {
		t.Fatalf("response body length = %d, want 100", len(got))
	}
	mu.Lock()
	defer mu.Unlock()
	if len(ranges) != 2 || ranges[0] != "bytes=0-0" || ranges[1] != "bytes=100-199" {
		t.Fatalf("upstream ranges = %v", ranges)
	}
}

func TestCloudStorageHandlerRejectsIgnoredRange(t *testing.T) {
	gin.SetMode(gin.TestMode)
	requestCount := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if requestCount == 1 {
			w.Header().Set("Content-Range", "bytes 0-0/1024")
			w.WriteHeader(http.StatusPartialContent)
			_, _ = w.Write([]byte{0})
			return
		}
		w.Header().Set("Content-Length", "1024")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "not a valid partial response")
	}))
	defer upstream.Close()

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/video", nil)
	c.Request.Header.Set("Range", "bytes=100-199")
	NewCloudStorageHandler(upstream.URL, nil, false).HandleRequest(c)

	if recorder.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusBadGateway, recorder.Body.String())
	}
}

func TestDynamicProxyReusesUpstreamConnection(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var connections atomic.Int32
	upstream := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "ok")
	}))
	upstream.Config.ConnState = func(_ net.Conn, state http.ConnState) {
		if state == http.StateNew {
			connections.Add(1)
		}
	}
	upstream.Start()
	defer upstream.Close()

	router := gin.New()
	router.GET("/video", func(c *gin.Context) {
		DynamicProxy(c, upstream.URL, nil, false)
	})
	proxyServer := httptest.NewServer(router)
	defer proxyServer.Close()

	for i := 0; i < 2; i++ {
		resp, err := http.Get(proxyServer.URL + "/video")
		if err != nil {
			t.Fatalf("request %d: %v", i, err)
		}
		body, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil {
			t.Fatalf("request %d body: %v", i, readErr)
		}
		if resp.StatusCode != http.StatusOK || string(body) != "ok" {
			t.Fatalf("request %d: status=%d body=%q", i, resp.StatusCode, body)
		}
	}

	if got := connections.Load(); got != 1 {
		t.Fatalf("upstream connections = %d, want 1", got)
	}
}
