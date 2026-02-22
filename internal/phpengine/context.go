package phpengine

import (
	"net/http"
	"path/filepath"
	"strings"
)

// Context holds the PHP superglobals and execution context.
type Context struct {
	// PHP superglobals
	Server  map[string]string
	Get     map[string]string
	Post    map[string]string
	Cookies map[string]string
	Files   map[string]File
	Env     map[string]string

	// Execution info
	ScriptFilename string
	DocumentRoot   string
}

// File represents an uploaded file.
type File struct {
	Name     string
	Type     string
	Size     int64
	TempName string
}

// NewContext creates a PHP context from an HTTP request.
func NewContext(req *http.Request, docRoot, entryPoint string) *Context {
	ctx := &Context{
		Server:         make(map[string]string),
		Get:            make(map[string]string),
		Post:           make(map[string]string),
		Cookies:        make(map[string]string),
		Files:          make(map[string]File),
		Env:            make(map[string]string),
		DocumentRoot:   docRoot,
		ScriptFilename: filepath.Join(docRoot, entryPoint),
	}

	// Populate $_SERVER (CGI-compatible)
	ctx.Server["REQUEST_METHOD"] = req.Method
	ctx.Server["REQUEST_URI"] = req.URL.Path
	ctx.Server["QUERY_STRING"] = req.URL.RawQuery
	ctx.Server["SERVER_PROTOCOL"] = "HTTP/1.1"
	ctx.Server["SERVER_NAME"] = req.Host
	ctx.Server["DOCUMENT_ROOT"] = docRoot
	ctx.Server["SCRIPT_NAME"] = "/" + entryPoint
	ctx.Server["SCRIPT_FILENAME"] = ctx.ScriptFilename
	ctx.Server["PHP_SELF"] = "/" + entryPoint
	ctx.Server["REMOTE_ADDR"] = strings.Split(req.RemoteAddr, ":")[0]
	ctx.Server["CONTENT_TYPE"] = req.Header.Get("Content-Type")
	ctx.Server["CONTENT_LENGTH"] = req.Header.Get("Content-Length")

	// HTTPS
	if req.TLS != nil {
		ctx.Server["HTTPS"] = "on"
	}

	// Headers as HTTP_*
	for key, values := range req.Header {
		httpKey := "HTTP_" + strings.ToUpper(strings.ReplaceAll(key, "-", "_"))
		if httpKey != "HTTP_CONTENT_TYPE" && httpKey != "HTTP_CONTENT_LENGTH" {
			ctx.Server[httpKey] = values[0]
		}
	}

	// $_GET
	for key, values := range req.URL.Query() {
		ctx.Get[key] = values[0]
	}

	// $_POST (if applicable)
	if req.Method == "POST" {
		req.ParseForm()
		for key, values := range req.PostForm {
			ctx.Post[key] = values[0]
		}
	}

	// $_COOKIE
	for _, cookie := range req.Cookies() {
		ctx.Cookies[cookie.Name] = cookie.Value
	}

	return ctx
}
