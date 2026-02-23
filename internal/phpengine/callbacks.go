package phpengine

/*
#include <stdlib.h>
#include <string.h>
*/
import "C"
import (
	"context"
	"log/slog"
	"strconv"
	"sync"
	"unsafe"
)

// requestContext holds per-request state for Go callbacks
type requestContext struct {
	output   []byte
	headers  map[string]string
	postData []byte
	cookies  map[string]string
	server   map[string]string
}

var (
	requestContexts = make(map[int32]*requestContext)
	contextMu       sync.RWMutex
	nextThreadID    int32
	threadIDMu      sync.Mutex

	phpLogger   *slog.Logger
	phpLoggerMu sync.RWMutex
)

// getThreadID returns a unique thread ID for this request
func getThreadID() int32 {
	threadIDMu.Lock()
	defer threadIDMu.Unlock()
	nextThreadID++
	return nextThreadID
}

// SetLogger sets logger for PHP callback logs.
func SetLogger(logger *slog.Logger) {
	phpLoggerMu.Lock()
	phpLogger = logger
	phpLoggerMu.Unlock()
}

func getLogger() *slog.Logger {
	phpLoggerMu.RLock()
	defer phpLoggerMu.RUnlock()
	return phpLogger
}

func logPHPMessage(msgType C.int, message string, threadID int32) {
	logger := getLogger()
	if logger == nil {
		return
	}

	level := slog.LevelInfo
	switch int(msgType) {
	case 0, 1, 2, 3:
		level = slog.LevelError
	case 4:
		level = slog.LevelWarn
	case 5, 6:
		level = slog.LevelInfo
	case 7:
		level = slog.LevelDebug
	}

	logger.Log(context.Background(), level, "php log",
		"thread_id", threadID,
		"php_syslog_type", int(msgType),
		"message", message,
	)
}

// setRequestContext stores context for a thread
func setRequestContext(threadID int32, ctx *requestContext) {
	contextMu.Lock()
	requestContexts[threadID] = ctx
	contextMu.Unlock()
}

// getRequestContext retrieves context for a thread
func getRequestContext(threadID int32) *requestContext {
	contextMu.RLock()
	defer contextMu.RUnlock()
	return requestContexts[threadID]
}

// clearRequestContext removes context for a thread
func clearRequestContext(threadID int32) {
	contextMu.Lock()
	delete(requestContexts, threadID)
	contextMu.Unlock()
}

// Export Go functions for C callbacks

//export go_ub_write
func go_ub_write(threadIdx C.int, str *C.char, length C.size_t) C.size_t {
	ctx := getRequestContext(int32(threadIdx))
	if ctx == nil {
		return 0
	}

	data := C.GoBytes(unsafe.Pointer(str), C.int(length))
	ctx.output = append(ctx.output, data...)
	return length
}

//export go_send_headers
func go_send_headers(threadIdx C.int, status C.int, headers *C.char, headersLen C.size_t) C.int {
	ctx := getRequestContext(int32(threadIdx))
	if ctx == nil {
		return -1
	}

	// Parse headers string into map
	headersStr := C.GoStringN(headers, C.int(headersLen))
	ctx.headers = parseHeaders(headersStr, int(status))

	return 0
}

//export go_read_post
func go_read_post(threadIdx C.int, buffer *C.char, countBytes C.size_t) C.size_t {
	ctx := getRequestContext(int32(threadIdx))
	if ctx == nil || len(ctx.postData) == 0 {
		return 0
	}

	// Copy POST data to buffer
	toCopy := countBytes
	if toCopy > C.size_t(len(ctx.postData)) {
		toCopy = C.size_t(len(ctx.postData))
	}

	C.memcpy(unsafe.Pointer(buffer), unsafe.Pointer(&ctx.postData[0]), toCopy)
	ctx.postData = ctx.postData[toCopy:]

	return toCopy
}

//export go_read_cookies
func go_read_cookies(threadIdx C.int) *C.char {
	ctx := getRequestContext(int32(threadIdx))
	if ctx == nil || len(ctx.cookies) == 0 {
		return nil
	}

	// Format cookies as HTTP header format: "name=value; name2=value2"
	cookieStr := formatCookies(ctx.cookies)
	return C.CString(cookieStr)
}

//export go_register_variables
func go_register_variables(threadIdx C.int, key *C.char, value *C.char) {
	ctx := getRequestContext(int32(threadIdx))
	if ctx == nil {
		return
	}

	// Store server variable
	if ctx.server == nil {
		ctx.server = make(map[string]string)
	}
	ctx.server[C.GoString(key)] = C.GoString(value)
}

//export go_log_message
func go_log_message(threadIdx C.int, message *C.char, msgType C.int) {
	if message == nil {
		return
	}
	logPHPMessage(msgType, C.GoString(message), int32(threadIdx))
}

// parseHeaders parses a headers string into a map
func parseHeaders(headersStr string, status int) map[string]string {
	headers := make(map[string]string)
	headers[":status"] = strconv.Itoa(status)

	// Parse "Key: Value\r\n" format
	for len(headersStr) > 0 {
		// Find end of line
		lineEnd := findCRLForLF(headersStr)
		if lineEnd == -1 {
			lineEnd = len(headersStr)
		}

		line := headersStr[:lineEnd]
		if lineEnd < len(headersStr) {
			if headersStr[lineEnd] == '\r' && lineEnd+1 < len(headersStr) && headersStr[lineEnd+1] == '\n' {
				headersStr = headersStr[lineEnd+2:]
			} else {
				headersStr = headersStr[lineEnd+1:]
			}
		} else {
			headersStr = ""
		}

		// Skip empty lines
		if line == "" {
			continue
		}

		// Parse "Key: Value"
		colon := findColon(line)
		if colon > 0 {
			key := line[:colon]
			value := line[colon+1:]
			// Trim leading space from value
			if len(value) > 0 && value[0] == ' ' {
				value = value[1:]
			}
			headers[key] = value
		}
	}

	return headers
}

// formatCookies formats cookies map to HTTP header format
func formatCookies(cookies map[string]string) string {
	if len(cookies) == 0 {
		return ""
	}

	result := ""
	first := true
	for name, value := range cookies {
		if !first {
			result += "; "
		}
		result += name + "=" + value
		first = false
	}
	return result
}

// findCRLForLF finds the index of \r\n or \n
func findCRLForLF(s string) int {
	for i := 0; i < len(s); i++ {
		if s[i] == '\r' && i+1 < len(s) && s[i+1] == '\n' {
			return i
		}
		if s[i] == '\n' {
			return i
		}
	}
	return -1
}

// findColon finds the first colon in a string
func findColon(s string) int {
	for i := 0; i < len(s); i++ {
		if s[i] == ':' {
			return i
		}
	}
	return -1
}
