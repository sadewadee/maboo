package protocol

import "fmt"

// ResponseHeader holds HTTP response metadata from PHP workers.
type ResponseHeader struct {
	Status  int               `msgpack:"status"`
	Headers map[string]string `msgpack:"headers"`
}

// EncodeResponse creates a RESPONSE frame from response data.
func EncodeResponse(resp *ResponseHeader, body []byte) (*Frame, error) {
	headers, err := MarshalMsgpack(resp)
	if err != nil {
		return nil, fmt.Errorf("encoding response headers: %w", err)
	}
	return &Frame{
		Type:    TypeResponse,
		Headers: headers,
		Payload: body,
	}, nil
}

// DecodeResponse extracts response header and body from a RESPONSE frame.
func DecodeResponse(f *Frame) (*ResponseHeader, []byte, error) {
	if f.Type != TypeResponse {
		return nil, nil, fmt.Errorf("expected RESPONSE frame, got type 0x%02x", f.Type)
	}
	var resp ResponseHeader
	if err := UnmarshalMsgpack(f.Headers, &resp); err != nil {
		return nil, nil, fmt.Errorf("decoding response headers: %w", err)
	}
	return &resp, f.Payload, nil
}
