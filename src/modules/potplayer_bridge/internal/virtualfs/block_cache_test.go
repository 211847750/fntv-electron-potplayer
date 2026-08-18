package virtualfs

import (
	"bytes"
	"testing"
)

func TestMediaBlockCacheReadAt(t *testing.T) {
	var cache mediaBlockCache
	data := []byte("0123456789")
	cache.store(100, data)

	dst := make([]byte, 4)
	if n := cache.readAt(dst, 103); n != 4 {
		t.Fatalf("readAt() = %d, want 4", n)
	}
	if !bytes.Equal(dst, []byte("3456")) {
		t.Fatalf("readAt() data = %q, want %q", dst, "3456")
	}

	dst = make([]byte, 4)
	if n := cache.readAt(dst, 108); n != 2 {
		t.Fatalf("readAt() at block end = %d, want 2", n)
	}
	if !bytes.Equal(dst[:2], []byte("89")) {
		t.Fatalf("readAt() at block end data = %q, want %q", dst[:2], "89")
	}

	if n := cache.readAt(dst, 99); n != 0 {
		t.Fatalf("readAt() before block = %d, want 0", n)
	}
}

func TestMediaBlockCacheEvictsLeastRecentlyUsed(t *testing.T) {
	var cache mediaBlockCache
	cache.store(0, []byte("a"))
	cache.store(mediaBlockSize, []byte("b"))

	dst := make([]byte, 1)
	if n := cache.readAt(dst, 0); n != 1 {
		t.Fatalf("readAt() first block = %d, want 1", n)
	}

	cache.store(2*mediaBlockSize, []byte("c"))
	if n := cache.readAt(dst, mediaBlockSize); n != 0 {
		t.Fatalf("readAt() evicted block = %d, want 0", n)
	}
	if n := cache.readAt(dst, 0); n != 1 || dst[0] != 'a' {
		t.Fatalf("readAt() retained block = %d %q, want 1 %q", n, dst[0], 'a')
	}
	if n := cache.readAt(dst, 2*mediaBlockSize); n != 1 || dst[0] != 'c' {
		t.Fatalf("readAt() newest block = %d %q, want 1 %q", n, dst[0], 'c')
	}
}

func TestMediaBlockStart(t *testing.T) {
	tests := []struct {
		offset int64
		want   int64
	}{
		{offset: 0, want: 0},
		{offset: 1, want: 0},
		{offset: mediaBlockSize - 1, want: 0},
		{offset: mediaBlockSize, want: mediaBlockSize},
		{offset: mediaBlockSize + 123, want: mediaBlockSize},
	}

	for _, tt := range tests {
		if got := mediaBlockStart(tt.offset); got != tt.want {
			t.Fatalf("mediaBlockStart(%d) = %d, want %d", tt.offset, got, tt.want)
		}
	}
}
