package protocol

import "fmt"

// StreamHeader holds WebSocket stream metadata.
type StreamHeader struct {
	ConnectionID string `msgpack:"conn_id"`
	Event        string `msgpack:"event"` // "connect", "message", "close"
	Room         string `msgpack:"room"`
}

// EncodeStreamData creates a STREAM_DATA frame for WebSocket communication.
func EncodeStreamData(streamID uint16, header *StreamHeader, data []byte) (*Frame, error) {
	headers, err := MarshalMsgpack(header)
	if err != nil {
		return nil, fmt.Errorf("encoding stream headers: %w", err)
	}
	return &Frame{
		Type:     TypeStreamData,
		StreamID: streamID,
		Headers:  headers,
		Payload:  data,
	}, nil
}

// DecodeStreamData extracts stream header and data from a STREAM_DATA frame.
func DecodeStreamData(f *Frame) (*StreamHeader, []byte, error) {
	if f.Type != TypeStreamData {
		return nil, nil, fmt.Errorf("expected STREAM_DATA frame, got type 0x%02x", f.Type)
	}
	var header StreamHeader
	if err := UnmarshalMsgpack(f.Headers, &header); err != nil {
		return nil, nil, fmt.Errorf("decoding stream headers: %w", err)
	}
	return &header, f.Payload, nil
}

// EncodeStreamClose creates a STREAM_CLOSE frame.
func EncodeStreamClose(streamID uint16, connID string) (*Frame, error) {
	header := &StreamHeader{
		ConnectionID: connID,
		Event:        "close",
	}
	headers, err := MarshalMsgpack(header)
	if err != nil {
		return nil, fmt.Errorf("encoding stream close headers: %w", err)
	}
	return &Frame{
		Type:     TypeStreamClose,
		StreamID: streamID,
		Headers:  headers,
	}, nil
}
