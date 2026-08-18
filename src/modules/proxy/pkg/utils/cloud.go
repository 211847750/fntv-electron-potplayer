package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"proxy/pkg/logger"

	"github.com/allegro/bigcache/v3"
	"github.com/gin-gonic/gin"
)

var (
	// 全局文件大小缓存
	cache               *bigcache.BigCache
	cacheOnce           sync.Once
	cloudClient         = &http.Client{Timeout: 60 * time.Second, Transport: sharedProxyTransport(false)}
	insecureCloudClient = &http.Client{Timeout: 60 * time.Second, Transport: sharedProxyTransport(true)}
)

const (
	// ChunkSize 固定分块大小 10MiB
	ChunkSize = 10 * 1024 * 1024
	// MaxConcurrentChunks 最大并发分块数 - 串行请求防止网盘风控
	MaxConcurrentChunks = 1
	// PreloadChunks 预加载分块数 - 减少预加载避免过多请求
	PreloadChunks = 1
)

func init() {
	initFileSizeCache()
}

// initFileSizeCache 初始化文件大小缓存
func initFileSizeCache() {
	cacheOnce.Do(func() {
		config := bigcache.DefaultConfig(200 * time.Minute) // 文件大小缓存200分钟
		config.Shards = 1024
		config.MaxEntriesInWindow = 1000 * 10 * 60
		config.MaxEntrySize = 500
		config.HardMaxCacheSize = 512

		var err error
		cache, err = bigcache.New(context.TODO(), config)
		if err != nil {
			logger.Warnf("初始化文件大小缓存失败: %v，使用默认配置", err)
			cache, _ = bigcache.New(context.TODO(), bigcache.DefaultConfig(30*time.Minute))
		}
		logger.Info("文件大小缓存初始化完成")
	})
}

// FileMetaInfo 文件元信息
type FileMetaInfo struct {
	Size        int64
	ContentType string
}

// setCachedFileInfo 缓存文件信息
func setCachedFileInfo(url string, info *FileMetaInfo) error {
	return cache.Set(url, JsonPrintBytes(info))
}

// getCachedFileInfo 获取缓存的文件信息
func getCachedFileInfo(url string) *FileMetaInfo {
	data, err := cache.Get(url)
	if err != nil {
		return nil
	}

	info := &FileMetaInfo{}
	err = json.Unmarshal(data, info)
	if err != nil {
		return nil
	}

	return info
}

// CloudStorageHandler 云盘存储处理器
type CloudStorageHandler struct {
	targetURL string
	headers   map[string]string
	client    *http.Client
}

// NewCloudStorageHandler 创建云盘存储处理器
func NewCloudStorageHandler(targetURL string, headers map[string]string, skipVerify bool) *CloudStorageHandler {
	client := cloudClient
	if skipVerify {
		client = insecureCloudClient
	}
	return &CloudStorageHandler{
		targetURL: targetURL,
		headers:   headers,
		client:    client,
	}
}

// RangeRequest 表示Range请求
type RangeRequest struct {
	Start int64
	End   int64 // -1表示到文件末尾
}

// RangeResponse 表示Range响应
type RangeResponse struct {
	Start      int64
	End        int64
	TotalSize  int64
	Data       []byte
	StatusCode int
}

// parseRange 解析Range头
func parseRange(rangeHeader string) (*RangeRequest, error) {
	if rangeHeader == "" {
		return &RangeRequest{Start: 0, End: -1}, nil
	}

	if !strings.HasPrefix(rangeHeader, "bytes=") {
		return nil, fmt.Errorf("unsupported range format: %s", rangeHeader)
	}

	rangeStr := strings.TrimPrefix(rangeHeader, "bytes=")
	startText, endText, ok := strings.Cut(rangeStr, "-")
	if !ok || strings.Contains(endText, "-") || strings.Contains(rangeStr, ",") {
		return nil, fmt.Errorf("invalid range format: %s", rangeStr)
	}

	start, err := strconv.ParseInt(startText, 10, 64)
	if err != nil || start < 0 {
		return nil, fmt.Errorf("invalid start position: %s", startText)
	}

	var end int64 = -1
	if endText != "" {
		end, err = strconv.ParseInt(endText, 10, 64)
		if err != nil || end < start {
			return nil, fmt.Errorf("invalid end position: %s", endText)
		}
	}

	return &RangeRequest{Start: start, End: end}, nil
}

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

// getMetaInfo 获取文件大小
func (h *CloudStorageHandler) getMetaInfo() (*FileMetaInfo, error) {
	// 先尝试从缓存获取
	cachedInfo := getCachedFileInfo(h.targetURL)
	if cachedInfo != nil {
		logger.Debugf("从缓存获取文件大小: %d", cachedInfo.Size)
		return cachedInfo, nil
	}

	req, err := http.NewRequest(http.MethodGet, h.targetURL, nil)
	if err != nil {
		return nil, err
	}

	// 设置Range为0-0来获取文件大小
	for k, v := range h.headers {
		req.Header.Set(k, v)
	}
	req.Header.Set("Range", "bytes=0-0")

	resp, err := h.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusPartialContent {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	contentRange := resp.Header.Get("Content-Range")
	start, end, totalSize, err := parseContentRange(contentRange)
	if err != nil {
		return nil, fmt.Errorf("invalid metadata Content-Range %q: %w", contentRange, err)
	}
	if start != 0 || end != 0 {
		return nil, fmt.Errorf("unexpected metadata Content-Range %q", contentRange)
	}
	_, _ = io.CopyN(io.Discard, resp.Body, 1)

	// 返回Content-Type
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	info := &FileMetaInfo{
		Size:        totalSize,
		ContentType: contentType,
	}

	// 缓存文件信息
	_ = setCachedFileInfo(h.targetURL, info)

	logger.Infof("文件大小: %d bytes, 文件Content-Type: %s", totalSize, contentType)

	return info, nil
}

// sendRangeError 发送416错误
func (h *CloudStorageHandler) sendRangeError(c *gin.Context, totalSize int64) {
	c.Header("Content-Range", fmt.Sprintf("bytes */%d", totalSize))
	c.JSON(http.StatusRequestedRangeNotSatisfiable, gin.H{"error": "Range not satisfiable"})
}

func (h *CloudStorageHandler) HandleRequest(c *gin.Context) {
	h.serveMPVRangeSimple(c)
}

// passthroughHeaders 复制上游部分响应头到下游
func passthroughHeaders(dst gin.ResponseWriter, src *http.Response) {
	for k, vv := range src.Header {
		for _, v := range vv {
			dst.Header().Add(k, v)
		}
	}
}

func (h *CloudStorageHandler) serveMPVRangeSimple(c *gin.Context) {
	started := time.Now()
	requestID := proxyRequestCount.Add(1)
	requestedRange := c.GetHeader("Range")
	var (
		contentRange   string
		upstreamStatus int
		proxyErr       error
	)
	defer func() {
		logProxyResult(requestID, "chunked", requestedRange, contentRange, upstreamStatus, c.Writer.Status(), c.Writer.Size(), time.Since(started), proxyErr)
	}()

	// 1) 获取总大小
	info, err := h.getMetaInfo()
	if err != nil {
		proxyErr = err
		logger.Errorf("getMetaInfo failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get meta"})
		return
	}

	// 2) 解析客户端 Range；无 Range 时按 0- 处理
	rr, err := parseRange(c.GetHeader("Range"))
	if err != nil {
		proxyErr = err
		// 容错：当 Range 不合法，返回 416
		logger.Warnf("invalid Range: %v", err)
		h.sendRangeError(c, info.Size)
		return
	}

	start := rr.Start
	if start < 0 {
		start = 0
	}
	if start >= info.Size {
		h.sendRangeError(c, info.Size)
		return
	}

	end := rr.End
	if end < 0 || end > start+ChunkSize-1 {
		end = start + ChunkSize - 1
	}
	if end >= info.Size {
		end = info.Size - 1
	}

	// 3) 直连上游（仅调整我们发给上游的 Range）
	req, err := http.NewRequest(http.MethodGet, h.targetURL, nil)
	if err != nil {
		proxyErr = err
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid upstream URL"})
		return
	}
	for k, v := range h.headers {
		req.Header.Set(k, v)
	}
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, end))

	// 建议：拉长超时或置 0 以适配慢速网络的小机器
	// 若你保持原有 h.client 30s 也可，因为 10MiB 通常很快
	resp, err := h.client.Do(req)
	if err != nil {
		proxyErr = err
		logger.Errorf("upstream error: %v", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "upstream unavailable"})
		return
	}
	defer resp.Body.Close()
	upstreamStatus = resp.StatusCode
	contentRange = resp.Header.Get("Content-Range")

	if resp.StatusCode != http.StatusPartialContent {
		proxyErr = fmt.Errorf("upstream ignored or rejected Range: status=%d", resp.StatusCode)
		// 其它状态直接转发（如 4xx/5xx）
		logger.Warnf("unexpected upstream status: %d", resp.StatusCode)
		c.JSON(http.StatusBadGateway, gin.H{"error": "upstream does not support byte ranges"})
		return
	}

	upstreamStart, upstreamEnd, _, err := parseContentRange(contentRange)
	if err != nil {
		proxyErr = fmt.Errorf("unexpected upstream Content-Range %q: %w", contentRange, err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "invalid upstream byte range"})
		return
	}
	if upstreamStart != start || upstreamEnd != end {
		proxyErr = fmt.Errorf("unexpected upstream Content-Range %q for bytes=%d-%d", contentRange, start, end)
		c.JSON(http.StatusBadGateway, gin.H{"error": "invalid upstream byte range"})
		return
	}

	length := end - start + 1
	passthroughHeaders(c.Writer, resp)
	c.Header("Accept-Ranges", "bytes")
	c.Header("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, info.Size))
	c.Header("Content-Length", strconv.FormatInt(length, 10))
	c.Status(http.StatusPartialContent)

	if f, ok := c.Writer.(http.Flusher); ok {
		f.Flush()
	}
	if _, err := io.CopyN(c.Writer, resp.Body, length); err != nil {
		proxyErr = err
		// 客户端中断通常会到这里，不必视作错误
		logger.Debugf("client closed / copy ended: %v", err)
		return
	}
}
