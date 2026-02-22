package protocol

import (
	"bytes"
	"testing"
)

func BenchmarkWriteFrame(b *testing.B) {
	var buf bytes.Buffer
	frame := &Frame{
		Type:    TypeRequest,
		Flags:   0,
		Headers: []byte(`{"method":"GET","uri":"/","headers":{}}`),
		Payload: []byte("Hello, World!"),
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		buf.Reset()
		WriteFrame(&buf, frame)
	}
}

func BenchmarkReadFrame(b *testing.B) {
	frame := &Frame{
		Type:    TypeResponse,
		Flags:   0,
		Headers: []byte(`{"status":200,"headers":{"Content-Type":"text/html"}}`),
		Payload: bytes.Repeat([]byte("a"), 4096),
	}

	var buf bytes.Buffer
	WriteFrame(&buf, frame)
	data := buf.Bytes()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		reader := bytes.NewReader(data)
		ReadFrame(reader)
	}
}

func BenchmarkWriteReadRoundtrip(b *testing.B) {
	frame := &Frame{
		Type:    TypeRequest,
		Flags:   0,
		Headers: []byte(`{"method":"POST","uri":"/api/data","query_string":"page=1","headers":{"Content-Type":"application/json"},"remote_addr":"127.0.0.1:5000","server_name":"localhost","server_port":"8080","protocol":"HTTP/1.1"}`),
		Payload: []byte(`{"name":"test","value":42}`),
	}

	var buf bytes.Buffer

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		buf.Reset()
		WriteFrame(&buf, frame)
		ReadFrame(&buf)
	}
}

func BenchmarkEncodeRequest(b *testing.B) {
	header := &RequestHeader{
		Method:      "GET",
		URI:         "/api/users",
		QueryString: "page=1&limit=20",
		Headers: map[string]string{
			"Accept":       "application/json",
			"Authorization": "Bearer token123",
			"User-Agent":   "maboo-bench/1.0",
		},
		RemoteAddr: "192.168.1.100:54321",
		ServerName: "api.example.com",
		ServerPort: "443",
		Protocol:   "HTTP/2.0",
	}
	body := []byte(`{"query":"SELECT * FROM users"}`)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		EncodeRequest(header, body)
	}
}

func BenchmarkDecodeResponse(b *testing.B) {
	resp := &ResponseHeader{
		Status: 200,
		Headers: map[string]string{
			"Content-Type":   "application/json",
			"Cache-Control":  "no-cache",
			"X-Request-ID":   "abc123",
		},
	}
	body := bytes.Repeat([]byte(`{"id":1,"name":"test"}`), 100)
	frame, _ := EncodeResponse(resp, body)
	data := new(bytes.Buffer)
	WriteFrame(data, frame)
	frameData := data.Bytes()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		reader := bytes.NewReader(frameData)
		f, _ := ReadFrame(reader)
		DecodeResponse(f)
	}
}

func BenchmarkLargePayload(b *testing.B) {
	sizes := []struct {
		name string
		size int
	}{
		{"1KB", 1024},
		{"4KB", 4096},
		{"64KB", 64 * 1024},
		{"256KB", 256 * 1024},
		{"1MB", 1024 * 1024},
	}

	for _, s := range sizes {
		b.Run(s.name, func(b *testing.B) {
			frame := &Frame{
				Type:    TypeResponse,
				Flags:   0,
				Headers: []byte(`{"status":200}`),
				Payload: bytes.Repeat([]byte("x"), s.size),
			}

			var buf bytes.Buffer
			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				buf.Reset()
				WriteFrame(&buf, frame)
				ReadFrame(&buf)
			}
		})
	}
}

func BenchmarkMsgpackEncode(b *testing.B) {
	data := map[string]interface{}{
		"method":       "POST",
		"uri":          "/api/submit",
		"query_string": "ref=dashboard",
		"headers": map[string]interface{}{
			"Content-Type":  "application/json",
			"Authorization": "Bearer eyJhbGciOiJIUzI1NiJ9",
			"Accept":        "application/json",
			"User-Agent":    "Mozilla/5.0 (X11; Linux x86_64) Chrome/120.0",
		},
		"remote_addr": "10.0.0.1:12345",
		"server_name": "api.maboo.dev",
		"server_port": "8443",
		"protocol":    "HTTP/2.0",
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		MsgpackEncode(data)
	}
}

func BenchmarkMsgpackDecode(b *testing.B) {
	data := map[string]interface{}{
		"status": int64(200),
		"headers": map[string]interface{}{
			"Content-Type": "text/html; charset=utf-8",
			"Set-Cookie":   "session=abc; Path=/; HttpOnly",
		},
	}
	encoded, _ := MsgpackEncode(data)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		MsgpackDecode(encoded)
	}
}

func BenchmarkPingPong(b *testing.B) {
	var buf bytes.Buffer

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		buf.Reset()
		WriteFrame(&buf, NewPingFrame())
		ReadFrame(&buf)
	}
}
