package protocol

import "fmt"

// RequestHeader holds HTTP request metadata sent to PHP workers.
type RequestHeader struct {
	Method      string            `msgpack:"method"`
	URI         string            `msgpack:"uri"`
	QueryString string            `msgpack:"query_string"`
	Headers     map[string]string `msgpack:"headers"`
	RemoteAddr  string            `msgpack:"remote_addr"`
	ServerName  string            `msgpack:"server_name"`
	ServerPort  string            `msgpack:"server_port"`
	Protocol    string            `msgpack:"protocol"`
}

// EncodeRequest creates a REQUEST frame from HTTP request data.
func EncodeRequest(req *RequestHeader, body []byte) (*Frame, error) {
	headers, err := MarshalMsgpack(req)
	if err != nil {
		return nil, fmt.Errorf("encoding request headers: %w", err)
	}
	return &Frame{
		Type:    TypeRequest,
		Headers: headers,
		Payload: body,
	}, nil
}

// DecodeRequest extracts request header and body from a REQUEST frame.
func DecodeRequest(f *Frame) (*RequestHeader, []byte, error) {
	if f.Type != TypeRequest {
		return nil, nil, fmt.Errorf("expected REQUEST frame, got type 0x%02x", f.Type)
	}
	var req RequestHeader
	if err := UnmarshalMsgpack(f.Headers, &req); err != nil {
		return nil, nil, fmt.Errorf("decoding request headers: %w", err)
	}
	return &req, f.Payload, nil
}
