package utils

import (
	"bytes"
	"crypto/sha1"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync/atomic"
	"time"

	"proxy/pkg/logger"

	"github.com/gin-gonic/gin"
)

var (
	proxyRequestCount      atomic.Uint64
	secureProxyTransport   = newProxyTransport(false)
	insecureProxyTransport = newProxyTransport(true)
)

func newProxyTransport(skipVerify bool) *http.Transport {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.MaxIdleConns = 100
	transport.MaxIdleConnsPerHost = 32
	transport.IdleConnTimeout = 90 * time.Second
	transport.ResponseHeaderTimeout = 120 * time.Second
	transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: skipVerify}
	return transport
}

func sharedProxyTransport(skipVerify bool) *http.Transport {
	if skipVerify {
		return insecureProxyTransport
	}
	return secureProxyTransport
}

func logProxyResult(requestID uint64, mode, requestedRange, contentRange string, upstreamStatus, downstreamStatus, bytes int, elapsed time.Duration, err error) {
	if bytes < 0 {
		bytes = 0
	}
	rateMiB := 0.0
	if elapsed > 0 {
		rateMiB = float64(bytes) / (1024 * 1024) / elapsed.Seconds()
	}

	format := "proxy request=%d mode=%s range=%q contentRange=%q upstream=%d downstream=%d bytes=%d durationMs=%d rateMiBps=%.2f"
	args := []interface{}{requestID, mode, requestedRange, contentRange, upstreamStatus, downstreamStatus, bytes, elapsed.Milliseconds(), rateMiB}
	if err != nil || upstreamStatus >= http.StatusBadRequest || downstreamStatus >= http.StatusBadRequest {
		if err != nil {
			format += " error=%q"
			args = append(args, err.Error())
		}
		logger.Warnf(format, args...)
		return
	}

	if requestID <= 20 || requestID%100 == 0 {
		logger.Infof(format, args...)
	}
}

func JsonPrintBytes(v any) []byte {
	if v == nil {
		return []byte("{}")
	}

	b, err := json.Marshal(v)
	if err != nil {
		return []byte("{}")
	}

	return b
}

// JsonPrint 打印JSON
func JsonPrint(v any) string {
	return string(JsonPrintBytes(v))
}

// StringToUUID 将字符串转换为UUID格式
func StringToUUID(s string) string {
	if len(s) == 0 {
		return "00000000-0000-0000-0000-000000000000"
	}

	hash := sha1.Sum([]byte(s))
	hexStr := hex.EncodeToString(hash[:16])

	return hexStr[:8] + "-" + hexStr[8:12] + "-" + hexStr[12:16] + "-" + hexStr[16:20] + "-" + hexStr[20:]
}

// PassthroughHeaders 从HTTP请求中提取需要传递的头信息
func PassthroughHeaders(req *http.Request) map[string]string {
	headers := make(map[string]string)

	// 需要传递的头列表
	passHeaders := []string{
		"User-Agent",
		"Accept",
		"Accept-Language",
		"Accept-Encoding",
		"Cache-Control",
		"Pragma",
		"Range",
		"If-Range",
		"If-Modified-Since",
		"If-None-Match",
	}

	for _, header := range passHeaders {
		if value := req.Header.Get(header); value != "" {
			headers[header] = value
		}
	}

	return headers
}

// DynamicProxy 透明代理
func DynamicProxy(c *gin.Context, targetURL string, extraHeaders map[string]string, skipVerify bool) {
	started := time.Now()
	requestID := proxyRequestCount.Add(1)
	requestedRange := c.Request.Header.Get("Range")
	if requestedRange == "" {
		requestedRange = extraHeaders["Range"]
	}
	var (
		contentRange   string
		upstreamStatus int
		proxyErr       error
	)

	defer func() {
		recovered := recover()
		if recovered == http.ErrAbortHandler {
			proxyErr = http.ErrAbortHandler
		}
		logProxyResult(requestID, "transparent", requestedRange, contentRange, upstreamStatus, c.Writer.Status(), c.Writer.Size(), time.Since(started), proxyErr)
		if recovered != nil && recovered != http.ErrAbortHandler {
			panic(recovered)
		}
	}()

	// 解析目标URL
	target, err := url.Parse(targetURL)
	if err != nil {
		proxyErr = err
		c.JSON(500, gin.H{"error": "Invalid target URL"})
		return
	}

	// 兼容webdav 取出 userinfo（如果有）
	var (
		uName   string
		uPass   string
		hasUser bool
	)
	if target.User != nil {
		uName = target.User.Username()
		if p, ok := target.User.Password(); ok {
			uPass = p
		}
		hasUser = (uName != "")
	}

	// 清除URL中的用户信息，避免泄露
	targetNoUser := *target
	targetNoUser.User = nil

	// 创建反向代理
	proxy := httputil.NewSingleHostReverseProxy(&targetNoUser)

	proxy.Transport = sharedProxyTransport(skipVerify)

	// 修改请求前的处理
	proxy.Director = func(req *http.Request) {
		// 设置原始请求信息
		req.URL.Scheme = target.Scheme
		req.URL.Host = target.Host
		req.URL.Path = target.Path
		req.URL.RawQuery = target.RawQuery
		req.Host = target.Host

		// 复制原始请求的头部
		for key, values := range c.Request.Header {
			for _, value := range values {
				req.Header.Set(key, value)
			}
		}

		// 添加额外的头部信息
		for key, value := range extraHeaders {
			req.Header.Set(key, value)
		}

		// 如果客户端没带 Authorization，但 URL 有 userinfo，就补上 BasicAuth
		if req.Header.Get("Authorization") == "" && hasUser {
			req.SetBasicAuth(uName, uPass)
		}

		// 设置请求方法
		req.Method = c.Request.Method

		// 如果有请求体，复制它
		if c.Request.Body != nil {
			bodyBytes, err := io.ReadAll(c.Request.Body)
			if err == nil {
				req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
				req.ContentLength = int64(len(bodyBytes))
			}
		}
	}

	// 修改响应后的处理
	proxy.ModifyResponse = func(resp *http.Response) error {
		upstreamStatus = resp.StatusCode
		contentRange = resp.Header.Get("Content-Range")
		return nil
	}

	// 处理错误 - 修复：避免重复写入响应头
	proxy.ErrorHandler = func(w http.ResponseWriter, req *http.Request, err error) {
		proxyErr = err
		// 检查响应是否已经开始写入
		if c.Writer.Written() {
			return
		}
		c.JSON(500, gin.H{"error": "Proxy error", "details": err.Error()})
	}

	// 执行代理
	proxy.ServeHTTP(c.Writer, c.Request)
}
