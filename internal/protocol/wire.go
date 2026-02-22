package protocol

import (
	"encoding/binary"
	"fmt"
	"io"
	"sync"
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

// writeBufPool pools scratch buffers for WriteFrame to avoid per-call allocation.
// For small frames (ping/pong, worker signals) this eliminates the header escape.
var writeBufPool = sync.Pool{
	New: func() interface{} {
		b := make([]byte, 0, 256) // enough for most control frames
		return &b
	},
}

// WriteFrame encodes and writes a frame to the given writer.
// Coalesces header + headers + payload into a single Write call to reduce
// syscalls and avoid per-call heap allocations for small frames.
func WriteFrame(w io.Writer, f *Frame) error {
	totalSize := FrameHeaderSize + len(f.Headers) + len(f.Payload)

	// Get a pooled buffer, grow if needed
	bp := writeBufPool.Get().(*[]byte)
	buf := (*bp)[:0]
	if cap(buf) < totalSize {
		buf = make([]byte, 0, totalSize)
	}
	buf = buf[:FrameHeaderSize]

	buf[0] = Magic[0]
	buf[1] = Magic[1]
	buf[2] = Version
	buf[3] = f.Type
	buf[4] = f.Flags
	binary.BigEndian.PutUint16(buf[5:7], f.StreamID)

	hdrSize := len(f.Headers)
	buf[7] = byte(hdrSize >> 16)
	buf[8] = byte(hdrSize >> 8)
	buf[9] = byte(hdrSize)

	binary.BigEndian.PutUint32(buf[10:14], uint32(len(f.Payload)))

	buf = append(buf, f.Headers...)
	buf = append(buf, f.Payload...)

	_, err := w.Write(buf)

	// Return buffer to pool
	*bp = buf
	writeBufPool.Put(bp)

	if err != nil {
		return fmt.Errorf("writing frame: %w", err)
	}
	return nil
}

// readHdrPool pools the 14-byte header buffer for ReadFrame.
var readHdrPool = sync.Pool{
	New: func() interface{} {
		b := make([]byte, FrameHeaderSize)
		return &b
	},
}

// ReadFrame reads and decodes a frame from the given reader.
// Uses pooled header buffer and coalesced data allocation.
func ReadFrame(r io.Reader) (*Frame, error) {
	bp := readHdrPool.Get().(*[]byte)
	header := *bp

	if _, err := io.ReadFull(r, header); err != nil {
		readHdrPool.Put(bp)
		return nil, fmt.Errorf("reading frame header: %w", err)
	}

	if header[0] != Magic[0] || header[1] != Magic[1] {
		readHdrPool.Put(bp)
		return nil, fmt.Errorf("invalid magic bytes: 0x%02x%02x", header[0], header[1])
	}
	if header[2] != Version {
		readHdrPool.Put(bp)
		return nil, fmt.Errorf("unsupported protocol version: %d", header[2])
	}

	f := &Frame{
		Type:     header[3],
		Flags:    header[4],
		StreamID: binary.BigEndian.Uint16(header[5:7]),
	}

	hdrSize := int(header[7])<<16 | int(header[8])<<8 | int(header[9])
	payloadSize := int(binary.BigEndian.Uint32(header[10:14]))

	readHdrPool.Put(bp)

	// Single allocation for both headers + payload data
	totalData := hdrSize + payloadSize
	if totalData > 0 {
		data := make([]byte, totalData)
		if _, err := io.ReadFull(r, data); err != nil {
			return nil, fmt.Errorf("reading frame data (%d bytes): %w", totalData, err)
		}
		if hdrSize > 0 {
			f.Headers = data[:hdrSize]
		}
		if payloadSize > 0 {
			f.Payload = data[hdrSize:]
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
