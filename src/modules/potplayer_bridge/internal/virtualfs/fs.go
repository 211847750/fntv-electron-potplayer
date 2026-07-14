//go:build windows

package virtualfs

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/winfsp/cgofuse/fuse"
)

const defaultVideoSize int64 = 1 << 40

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

	req, err := http.NewRequest(http.MethodGet, mf.VideoURL, nil)
	if err != nil {
		return -fuse.EIO
	}

	end := ofst + int64(len(buff)) - 1
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", ofst, end))
	for k, v := range mf.VideoHeaders {
		req.Header.Set(k, v)
	}
	mf.logRead(ofst, len(buff))

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return -fuse.EIO
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusPartialContent && resp.StatusCode != http.StatusOK {
		return -fuse.EIO
	}

	if resp.StatusCode == http.StatusPartialContent {
		if size := parseContentRangeSize(resp.Header.Get("Content-Range")); size > 0 {
			mf.mu.Lock()
			if mf.videoSize <= 0 || mf.videoSize == defaultVideoSize {
				mf.videoSize = size
			}
			mf.mu.Unlock()
		}
	}

	n, err := io.ReadFull(resp.Body, buff)
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		return -fuse.EIO
	}
	return n
}

func (mf *MediaFile) logRead(offset int64, length int) {
	mf.mu.Lock()
	mf.readCount++
	mf.readBytes += int64(length)
	count := mf.readCount
	totalBytes := mf.readBytes
	mf.mu.Unlock()

	if count <= 20 || count%50 == 0 {
		fmt.Fprintf(os.Stderr, "[VFS] read name=%q offset=%d length=%d request=%d totalMiB=%.2f\n",
			mf.Name, offset, length, count, float64(totalBytes)/(1024*1024))
	}
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

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusPartialContent {
		if size := parseContentRangeSize(resp.Header.Get("Content-Range")); size > 0 {
			return size, nil
		}
	}

	if resp.ContentLength > 0 {
		return resp.ContentLength, nil
	}
	return 0, fmt.Errorf("video size unavailable: status=%d", resp.StatusCode)
}

func parseContentRangeSize(contentRange string) int64 {
	_, total, ok := strings.Cut(contentRange, "/")
	if !ok {
		return 0
	}
	total = strings.TrimSpace(total)
	if total == "" || total == "*" {
		return 0
	}

	size, err := strconv.ParseInt(total, 10, 64)
	if err != nil || size <= 0 {
		return 0
	}
	return size
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
