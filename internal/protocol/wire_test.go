package protocol

import (
	"bytes"
	"testing"
)

func TestWriteReadFrameRoundtrip(t *testing.T) {
	tests := []struct {
		name  string
		frame *Frame
	}{
		{
			name: "request frame",
			frame: &Frame{
				Type:     TypeRequest,
				Flags:    0,
				StreamID: 0,
				Headers:  []byte(`{"method":"GET"}`),
				Payload:  []byte("hello"),
			},
		},
		{
			name: "response frame",
			frame: &Frame{
				Type:     TypeResponse,
				Flags:    0,
				StreamID: 0,
				Headers:  []byte(`{"status":200}`),
				Payload:  []byte("<html>OK</html>"),
			},
		},
		{
			name: "stream data frame",
			frame: &Frame{
				Type:     TypeStreamData,
				Flags:    0,
				StreamID: 42,
				Headers:  []byte(`{"conn_id":"abc"}`),
				Payload:  []byte("ws message"),
			},
		},
		{
			name: "worker ready",
			frame: NewWorkerReadyFrame(),
		},
		{
			name: "worker stop",
			frame: NewWorkerStopFrame(),
		},
		{
			name: "ping",
			frame: NewPingFrame(),
		},
		{
			name: "error",
			frame: NewErrorFrame("something went wrong"),
		},
		{
			name: "empty headers and payload",
			frame: &Frame{
				Type:     TypeWorkerReady,
				Flags:    0,
				StreamID: 0,
				Headers:  nil,
				Payload:  nil,
			},
		},
		{
			name: "with flags",
			frame: &Frame{
				Type:     TypeResponse,
				Flags:    FlagCompressed | FlagFinal,
				StreamID: 100,
				Headers:  []byte("hdr"),
				Payload:  []byte("compressed data"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := WriteFrame(&buf, tt.frame); err != nil {
				t.Fatalf("WriteFrame: %v", err)
			}

			got, err := ReadFrame(&buf)
			if err != nil {
				t.Fatalf("ReadFrame: %v", err)
			}

			if got.Type != tt.frame.Type {
				t.Errorf("Type: got %d, want %d", got.Type, tt.frame.Type)
			}
			if got.Flags != tt.frame.Flags {
				t.Errorf("Flags: got %d, want %d", got.Flags, tt.frame.Flags)
			}
			if got.StreamID != tt.frame.StreamID {
				t.Errorf("StreamID: got %d, want %d", got.StreamID, tt.frame.StreamID)
			}
			if !bytes.Equal(got.Headers, tt.frame.Headers) {
				t.Errorf("Headers: got %q, want %q", got.Headers, tt.frame.Headers)
			}
			if !bytes.Equal(got.Payload, tt.frame.Payload) {
				t.Errorf("Payload: got %q, want %q", got.Payload, tt.frame.Payload)
			}
		})
	}
}

func TestInvalidMagicBytes(t *testing.T) {
	data := make([]byte, FrameHeaderSize)
	data[0] = 0xFF
	data[1] = 0xFF
	data[2] = Version

	_, err := ReadFrame(bytes.NewReader(data))
	if err == nil {
		t.Error("expected error for invalid magic bytes")
	}
}

func TestInvalidVersion(t *testing.T) {
	data := make([]byte, FrameHeaderSize)
	data[0] = Magic[0]
	data[1] = Magic[1]
	data[2] = 0xFF // invalid version

	_, err := ReadFrame(bytes.NewReader(data))
	if err == nil {
		t.Error("expected error for invalid version")
	}
}

func TestLargePayload(t *testing.T) {
	payload := make([]byte, 1024*1024) // 1MB
	for i := range payload {
		payload[i] = byte(i % 256)
	}

	frame := &Frame{
		Type:    TypeResponse,
		Payload: payload,
	}

	var buf bytes.Buffer
	if err := WriteFrame(&buf, frame); err != nil {
		t.Fatalf("WriteFrame: %v", err)
	}

	got, err := ReadFrame(&buf)
	if err != nil {
		t.Fatalf("ReadFrame: %v", err)
	}

	if !bytes.Equal(got.Payload, payload) {
		t.Error("payload mismatch for large payload")
	}
}

func TestRequestEncodeDecodeRoundtrip(t *testing.T) {
	req := &RequestHeader{
		Method:      "POST",
		URI:         "/api/users",
		QueryString: "page=1&limit=10",
		Headers: map[string]string{
			"Content-Type":  "application/json",
			"Authorization": "Bearer token123",
		},
		RemoteAddr: "192.168.1.1:54321",
		ServerName: "localhost",
		ServerPort: "8080",
		Protocol:   "HTTP/1.1",
	}
	body := []byte(`{"name":"test"}`)

	frame, err := EncodeRequest(req, body)
	if err != nil {
		t.Fatalf("EncodeRequest: %v", err)
	}

	var buf bytes.Buffer
	if err := WriteFrame(&buf, frame); err != nil {
		t.Fatalf("WriteFrame: %v", err)
	}

	readFrame, err := ReadFrame(&buf)
	if err != nil {
		t.Fatalf("ReadFrame: %v", err)
	}

	gotReq, gotBody, err := DecodeRequest(readFrame)
	if err != nil {
		t.Fatalf("DecodeRequest: %v", err)
	}

	if gotReq.Method != "POST" {
		t.Errorf("Method: got %s, want POST", gotReq.Method)
	}
	if gotReq.URI != "/api/users" {
		t.Errorf("URI: got %s, want /api/users", gotReq.URI)
	}
	if gotReq.Headers["Content-Type"] != "application/json" {
		t.Errorf("Content-Type: got %s, want application/json", gotReq.Headers["Content-Type"])
	}
	if !bytes.Equal(gotBody, body) {
		t.Errorf("Body: got %s, want %s", gotBody, body)
	}
}

func TestResponseEncodeDecodeRoundtrip(t *testing.T) {
	resp := &ResponseHeader{
		Status: 201,
		Headers: map[string]string{
			"Content-Type": "application/json",
			"X-Request-Id": "abc-123",
		},
	}
	body := []byte(`{"id":1,"created":true}`)

	frame, err := EncodeResponse(resp, body)
	if err != nil {
		t.Fatalf("EncodeResponse: %v", err)
	}

	var buf bytes.Buffer
	if err := WriteFrame(&buf, frame); err != nil {
		t.Fatalf("WriteFrame: %v", err)
	}

	readFrame, err := ReadFrame(&buf)
	if err != nil {
		t.Fatalf("ReadFrame: %v", err)
	}

	gotResp, gotBody, err := DecodeResponse(readFrame)
	if err != nil {
		t.Fatalf("DecodeResponse: %v", err)
	}

	if gotResp.Status != 201 {
		t.Errorf("Status: got %d, want 201", gotResp.Status)
	}
	if gotResp.Headers["X-Request-Id"] != "abc-123" {
		t.Errorf("X-Request-Id: got %s, want abc-123", gotResp.Headers["X-Request-Id"])
	}
	if !bytes.Equal(gotBody, body) {
		t.Errorf("Body: got %s, want %s", gotBody, body)
	}
}

func TestStreamDataEncodeDecodeRoundtrip(t *testing.T) {
	header := &StreamHeader{
		ConnectionID: "conn-456",
		Event:        "message",
		Room:         "chat-room-1",
	}
	data := []byte("Hello WebSocket!")

	frame, err := EncodeStreamData(7, header, data)
	if err != nil {
		t.Fatalf("EncodeStreamData: %v", err)
	}

	var buf bytes.Buffer
	if err := WriteFrame(&buf, frame); err != nil {
		t.Fatalf("WriteFrame: %v", err)
	}

	readFrame, err := ReadFrame(&buf)
	if err != nil {
		t.Fatalf("ReadFrame: %v", err)
	}

	if readFrame.StreamID != 7 {
		t.Errorf("StreamID: got %d, want 7", readFrame.StreamID)
	}

	gotHeader, gotData, err := DecodeStreamData(readFrame)
	if err != nil {
		t.Fatalf("DecodeStreamData: %v", err)
	}

	if gotHeader.ConnectionID != "conn-456" {
		t.Errorf("ConnectionID: got %s, want conn-456", gotHeader.ConnectionID)
	}
	if gotHeader.Event != "message" {
		t.Errorf("Event: got %s, want message", gotHeader.Event)
	}
	if gotHeader.Room != "chat-room-1" {
		t.Errorf("Room: got %s, want chat-room-1", gotHeader.Room)
	}
	if !bytes.Equal(gotData, data) {
		t.Errorf("Data: got %s, want %s", gotData, data)
	}
}

func TestDecodeWrongFrameType(t *testing.T) {
	frame := &Frame{Type: TypePing}
	if _, _, err := DecodeRequest(frame); err == nil {
		t.Error("expected error decoding PING as REQUEST")
	}
	if _, _, err := DecodeResponse(frame); err == nil {
		t.Error("expected error decoding PING as RESPONSE")
	}
	if _, _, err := DecodeStreamData(frame); err == nil {
		t.Error("expected error decoding PING as STREAM_DATA")
	}
}
