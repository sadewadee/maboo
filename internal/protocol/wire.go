package protocol

import (
	"encoding/binary"
	"fmt"
	"io"
)

// Magic bytes identify maboo-wire protocol frames.
var Magic = [2]byte{0x4D, 0x42} // "MB"

// Version is the current protocol version.
const Version uint8 = 0x01

// FrameHeaderSize is the fixed size of a frame header in bytes.
const FrameHeaderSize = 14

// Message types define the purpose of each frame.
const (
	TypeRequest     uint8 = 0x01 // Go → PHP: new HTTP request
	TypeResponse    uint8 = 0x02 // PHP → Go: HTTP response
	TypeStreamData  uint8 = 0x03 // Bidirectional: WebSocket frame
	TypeStreamClose uint8 = 0x04 // Either: close WebSocket connection
	TypeWorkerReady uint8 = 0x05 // PHP → Go: worker is available
	TypeWorkerStop  uint8 = 0x06 // Go → PHP: graceful shutdown
	TypePing        uint8 = 0x07 // Health check (ping/pong)
	TypeError       uint8 = 0x08 // Error reporting
)

// Flags modify frame behavior.
const (
	FlagCompressed uint8 = 1 << 0 // Payload is compressed
	FlagChunked    uint8 = 1 << 1 // Chunked transfer
	FlagFinal      uint8 = 1 << 2 // Final chunk in sequence
)

// Frame represents a single maboo-wire protocol frame.
type Frame struct {
	Type     uint8
	Flags    uint8
	StreamID uint16
	Headers  []byte // msgpack encoded
	Payload  []byte // raw bytes
}

// WriteFrame encodes and writes a frame to the given writer.
func WriteFrame(w io.Writer, f *Frame) error {
	header := make([]byte, FrameHeaderSize)
	header[0] = Magic[0]
	header[1] = Magic[1]
	header[2] = Version
	header[3] = f.Type
	header[4] = f.Flags
	binary.BigEndian.PutUint16(header[5:7], f.StreamID)

	// Header size as 3 bytes (big-endian uint24)
	hdrSize := len(f.Headers)
	header[7] = byte(hdrSize >> 16)
	header[8] = byte(hdrSize >> 8)
	header[9] = byte(hdrSize)

	binary.BigEndian.PutUint32(header[10:14], uint32(len(f.Payload)))

	if _, err := w.Write(header); err != nil {
		return fmt.Errorf("writing frame header: %w", err)
	}
	if len(f.Headers) > 0 {
		if _, err := w.Write(f.Headers); err != nil {
			return fmt.Errorf("writing frame headers: %w", err)
		}
	}
	if len(f.Payload) > 0 {
		if _, err := w.Write(f.Payload); err != nil {
			return fmt.Errorf("writing frame payload: %w", err)
		}
	}
	return nil
}

// ReadFrame reads and decodes a frame from the given reader.
func ReadFrame(r io.Reader) (*Frame, error) {
	header := make([]byte, FrameHeaderSize)
	if _, err := io.ReadFull(r, header); err != nil {
		return nil, fmt.Errorf("reading frame header: %w", err)
	}

	if header[0] != Magic[0] || header[1] != Magic[1] {
		return nil, fmt.Errorf("invalid magic bytes: 0x%02x%02x", header[0], header[1])
	}
	if header[2] != Version {
		return nil, fmt.Errorf("unsupported protocol version: %d", header[2])
	}

	f := &Frame{
		Type:     header[3],
		Flags:    header[4],
		StreamID: binary.BigEndian.Uint16(header[5:7]),
	}

	hdrSize := int(header[7])<<16 | int(header[8])<<8 | int(header[9])
	payloadSize := binary.BigEndian.Uint32(header[10:14])

	if hdrSize > 0 {
		f.Headers = make([]byte, hdrSize)
		if _, err := io.ReadFull(r, f.Headers); err != nil {
			return nil, fmt.Errorf("reading frame headers (%d bytes): %w", hdrSize, err)
		}
	}
	if payloadSize > 0 {
		f.Payload = make([]byte, payloadSize)
		if _, err := io.ReadFull(r, f.Payload); err != nil {
			return nil, fmt.Errorf("reading frame payload (%d bytes): %w", payloadSize, err)
		}
	}

	return f, nil
}

// NewPingFrame creates a PING health check frame.
func NewPingFrame() *Frame {
	return &Frame{Type: TypePing, Payload: []byte("ping")}
}

// NewPongFrame creates a PONG response frame.
func NewPongFrame() *Frame {
	return &Frame{Type: TypePing, Payload: []byte("pong")}
}

// NewWorkerReadyFrame creates a WORKER_READY signal frame.
func NewWorkerReadyFrame() *Frame {
	return &Frame{Type: TypeWorkerReady}
}

// NewWorkerStopFrame creates a WORKER_STOP signal frame.
func NewWorkerStopFrame() *Frame {
	return &Frame{Type: TypeWorkerStop}
}

// NewErrorFrame creates an ERROR frame with a message.
func NewErrorFrame(msg string) *Frame {
	return &Frame{Type: TypeError, Payload: []byte(msg)}
}
