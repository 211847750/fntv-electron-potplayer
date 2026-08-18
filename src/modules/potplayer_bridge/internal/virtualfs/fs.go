//go:build windows

package virtualfs

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/winfsp/cgofuse/fuse"
)

const defaultVideoSize int64 = 1 << 40

var (
	mediaHTTPClient = &http.Client{Timeout: 60 * time.Second}
	probeHTTPClient = &http.Client{Timeout: 15 * time.Second}
)

type fileKind int

const (
	fileKindVideo fileKind = iota
	fileKindSubtitle
)

// MediaFile represents one logical video and its optional sidecar subtitle.
type MediaFile struct {
	Name         string
	SubtitleName string
	SubPath      string
	VideoURL     string
	VideoHeaders map[string]string

	mu        sync.Mutex
	videoSize int64
	readCount uint64
	readBytes int64
	cache     mediaBlockCache
}

type virtualFile struct {
	media *MediaFile
	kind  fileKind
	name  string
}

// VFS exposes media URLs and local subtitles as read-only files.
type VFS struct {
	fuse.FileSystemBase

	mu    sync.RWMutex
	files map[string]*virtualFile
	names []string
}

// NewVFS creates a new virtual filesystem.
func NewVFS() *VFS {
	return &VFS{
		files: make(map[string]*virtualFile),
	}
}

// AddFile adds a media file and its optional subtitle sidecar.
func (v *VFS) AddFile(mf *MediaFile) {
	v.mu.Lock()
	defer v.mu.Unlock()

	videoPath := "/" + mf.Name
	v.files[videoPath] = &virtualFile{media: mf, kind: fileKindVideo, name: mf.Name}

	if mf.SubtitleName != "" && mf.SubPath != "" {
		subPath := "/" + mf.SubtitleName
		v.files[subPath] = &virtualFile{media: mf, kind: fileKindSubtitle, name: mf.SubtitleName}
	}

	v.rebuildNamesLocked()
}

func (v *VFS) rebuildNamesLocked() {
	v.names = v.names[:0]
	for _, file := range v.files {
		v.names = append(v.names, file.name)
	}
	sort.Strings(v.names)
}

func (v *VFS) getFile(filePath string) *virtualFile {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.files[filePath]
}

func (v *VFS) Init() {}

func (v *VFS) Destroy() {}

func (v *VFS) Statfs(_ string, stat *fuse.Statfs_t) int {
	stat.Bsize = 4096
	stat.Blocks = 1000000
	stat.Bfree = 900000
	stat.Bavail = 900000
	stat.Files = uint64(len(v.files) + 1)
	stat.Ffree = 100
	stat.Namemax = 255
	return 0
}

func (v *VFS) Getattr(filePath string, stat *fuse.Stat_t, _ uint64) int {
	if filePath == "/" {
		stat.Mode = fuse.S_IFDIR | 0555
		return 0
	}

	file := v.getFile(filePath)
	if file == nil {
		return -fuse.ENOENT
	}

	stat.Mode = fuse.S_IFREG | 0444
	switch file.kind {
	case fileKindSubtitle:
		info, err := os.Stat(file.media.SubPath)
		if err != nil {
			return -fuse.ENOENT
		}
		stat.Size = info.Size()
	default:
		stat.Size = file.media.VideoSize()
	}
	stat.Blocks = (stat.Size + 511) / 512
	return 0
}

func (v *VFS) Readdir(filePath string, fill func(name string, stat *fuse.Stat_t, ofst int64) bool, _ int64, _ uint64) int {
	if filePath != "/" {
		return -fuse.ENOENT
	}

	fill(".", nil, 0)
	fill("..", nil, 0)

	v.mu.RLock()
	names := append([]string(nil), v.names...)
	v.mu.RUnlock()

	for _, name := range names {
		fill(name, nil, 0)
	}
	return 0
}

func (v *VFS) Open(filePath string, flags int) (int, uint64) {
	if flags&3 != os.O_RDONLY {
		return -fuse.EACCES, ^uint64(0)
	}
	if v.getFile(filePath) == nil {
		return -fuse.ENOENT, ^uint64(0)
	}
	return 0, 0
}

func (v *VFS) Read(filePath string, buff []byte, ofst int64, _ uint64) int {
	file := v.getFile(filePath)
	if file == nil {
		return -fuse.ENOENT
	}
	if ofst < 0 {
		return -fuse.EINVAL
	}

	if file.kind == fileKindSubtitle {
		return readSubtitle(file.media.SubPath, buff, ofst)
	}
	return file.media.ReadVideo(buff, ofst)
}

func readSubtitle(subPath string, buff []byte, ofst int64) int {
	f, err := os.Open(subPath)
	if err != nil {
		return -fuse.ENOENT
	}
	defer f.Close()

	n, err := f.ReadAt(buff, ofst)
	if err != nil && err != io.EOF {
		return -fuse.EIO
	}
	return n
}

func (mf *MediaFile) VideoSize() int64 {
	mf.mu.Lock()
	defer mf.mu.Unlock()

	if mf.videoSize > 0 {
		return mf.videoSize
	}

	size, err := probeVideoSize(mf.VideoURL, mf.VideoHeaders)
	if err != nil || size <= 0 {
		errText := "size unavailable"
		if err != nil {
			errText = err.Error()
		}
		fmt.Fprintf(os.Stderr, "[VFS] size probe failed name=%q error=%q fallback=%d\n", mf.Name, errText, defaultVideoSize)
		mf.videoSize = defaultVideoSize
		return mf.videoSize
	}
	mf.videoSize = size
	return size
}

func (mf *MediaFile) ReadVideo(buff []byte, ofst int64) int {
	if len(buff) == 0 {
		return 0
	}
	started := time.Now()
	stats := videoReadStats{}
	finish := func(actual int, readErr error) int {
		mf.logRead(ofst, len(buff), actual, stats, time.Since(started), readErr)
		if readErr != nil {
			return -fuse.EIO
		}
		return actual
	}

	actual := 0
	for actual < len(buff) {
		currentOffset := ofst + int64(actual)
		if n := mf.cache.readAt(buff[actual:], currentOffset); n > 0 {
			actual += n
			stats.cacheBytes += n
			continue
		}

		blockStart := mediaBlockStart(currentOffset)
		blockEnd := blockStart + mediaBlockSize - 1
		if size := mf.cachedVideoSize(); size > 0 && size != defaultVideoSize {
			if blockStart >= size {
				return finish(actual, nil)
			}
			if blockEnd >= size {
				blockEnd = size - 1
			}
		}

		data, statusCode, contentRange, err := mf.fetchVideoRange(blockStart, blockEnd)
		stats.upstreamRequests++
		stats.fetchedBytes += len(data)
		stats.statusCode = statusCode
		stats.contentRange = contentRange
		if err != nil {
			return finish(actual, err)
		}
		if len(data) == 0 {
			return finish(actual, nil)
		}

		mf.cache.store(blockStart, data)
		blockOffset := currentOffset - blockStart
		if blockOffset < 0 || blockOffset >= int64(len(data)) {
			return finish(actual, fmt.Errorf("fetched block bytes=%d-%d does not contain offset %d", blockStart, blockStart+int64(len(data))-1, currentOffset))
		}
		n := copy(buff[actual:], data[blockOffset:])
		if n == 0 {
			return finish(actual, fmt.Errorf("fetched block made no read progress at offset %d", currentOffset))
		}
		actual += n
	}
	return finish(actual, nil)
}

func (mf *MediaFile) cachedVideoSize() int64 {
	mf.mu.Lock()
	defer mf.mu.Unlock()
	return mf.videoSize
}

func (mf *MediaFile) fetchVideoRange(ofst, end int64) ([]byte, int, string, error) {
	req, err := http.NewRequest(http.MethodGet, mf.VideoURL, nil)
	if err != nil {
		return nil, 0, "", err
	}

	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", ofst, end))
	for k, v := range mf.VideoHeaders {
		req.Header.Set(k, v)
	}
	resp, err := mediaHTTPClient.Do(req)
	if err != nil {
		return nil, 0, "", err
	}
	defer resp.Body.Close()
	statusCode := resp.StatusCode
	contentRange := resp.Header.Get("Content-Range")

	readLength := end - ofst + 1
	switch resp.StatusCode {
	case http.StatusPartialContent:
		rangeEnd, size, rangeErr := validateContentRange(contentRange, ofst, end)
		if rangeErr != nil {
			return nil, statusCode, contentRange, rangeErr
		}
		readLength = rangeEnd - ofst + 1
		if readLength <= 0 || readLength > end-ofst+1 {
			return nil, statusCode, contentRange, fmt.Errorf("invalid response length %d for bytes=%d-%d", readLength, ofst, end)
		}
		if size > 0 {
			mf.mu.Lock()
			if mf.videoSize <= 0 || mf.videoSize == defaultVideoSize {
				mf.videoSize = size
			}
			mf.mu.Unlock()
		}
	case http.StatusOK:
		if ofst != 0 {
			return nil, statusCode, contentRange, fmt.Errorf("upstream ignored nonzero Range bytes=%d-%d", ofst, end)
		}
		if resp.ContentLength >= 0 && resp.ContentLength < readLength {
			readLength = resp.ContentLength
		}
	default:
		return nil, statusCode, contentRange, fmt.Errorf("unexpected HTTP status %d", resp.StatusCode)
	}

	if readLength == 0 {
		return nil, statusCode, contentRange, nil
	}
	if readLength > int64(maxInt()) {
		return nil, statusCode, contentRange, fmt.Errorf("response length %d exceeds platform limit", readLength)
	}
	data := make([]byte, int(readLength))
	n, err := io.ReadFull(resp.Body, data)
	if err != nil {
		return data[:n], statusCode, contentRange, fmt.Errorf("incomplete Range body: got=%d want=%d: %w", n, readLength, err)
	}
	return data, statusCode, contentRange, nil
}

type videoReadStats struct {
	cacheBytes       int
	fetchedBytes     int
	upstreamRequests int
	statusCode       int
	contentRange     string
}

func (mf *MediaFile) logRead(offset int64, requested, actual int, stats videoReadStats, elapsed time.Duration, readErr error) {
	mf.mu.Lock()
	mf.readCount++
	mf.readBytes += int64(actual)
	count := mf.readCount
	totalBytes := mf.readBytes
	mf.mu.Unlock()

	if readErr != nil || count <= 20 || count%50 == 0 {
		errText := ""
		if readErr != nil {
			errText = readErr.Error()
		}
		fetchRateMiB := 0.0
		if elapsed > 0 && stats.fetchedBytes > 0 {
			fetchRateMiB = float64(stats.fetchedBytes) / (1024 * 1024) / elapsed.Seconds()
		}
		fmt.Fprintf(os.Stderr, "[VFS] read name=%q offset=%d requested=%d actual=%d cacheBytes=%d upstreamRequests=%d fetchedBytes=%d status=%d contentRange=%q request=%d totalMiB=%.2f durationMs=%d fetchRateMiBps=%.2f error=%q\n",
			mf.Name, offset, requested, actual, stats.cacheBytes, stats.upstreamRequests, stats.fetchedBytes, stats.statusCode, stats.contentRange, count, float64(totalBytes)/(1024*1024), elapsed.Milliseconds(), fetchRateMiB, errText)
	}
}

func maxInt() int {
	return int(^uint(0) >> 1)
}

func probeVideoSize(rawURL string, headers map[string]string) (int64, error) {
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Range", "bytes=0-0")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := probeHTTPClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusPartialContent {
		if size := parseContentRangeSize(resp.Header.Get("Content-Range")); size > 0 {
			_, _ = io.CopyN(io.Discard, resp.Body, 1)
			return size, nil
		}
		return 0, fmt.Errorf("invalid Content-Range for size probe: %q", resp.Header.Get("Content-Range"))
	}

	if resp.StatusCode == http.StatusOK && resp.ContentLength > 0 {
		return resp.ContentLength, nil
	}
	return 0, fmt.Errorf("video size unavailable: status=%d", resp.StatusCode)
}

func videoExtension(rawURL string) string {
	ext := strings.ToLower(path.Ext(strings.Split(rawURL, "?")[0]))
	switch ext {
	case ".avi", ".flv", ".m2ts", ".m4v", ".mkv", ".mov", ".mp4", ".mpeg", ".mpg", ".rmvb", ".ts", ".webm", ".wmv":
		return ext
	default:
		return ".mp4"
	}
}
