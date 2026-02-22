<?php

declare(strict_types=1);

namespace Maboo\Protocol;

class Wire
{
    private const MAGIC = "\x4D\x42"; // "MB"
    private const VERSION = 0x01;
    private const HEADER_SIZE = 14;

    // Message types (must match Go constants)
    public const TYPE_REQUEST = 0x01;
    public const TYPE_RESPONSE = 0x02;
    public const TYPE_STREAM_DATA = 0x03;
    public const TYPE_STREAM_CLOSE = 0x04;
    public const TYPE_WORKER_READY = 0x05;
    public const TYPE_WORKER_STOP = 0x06;
    public const TYPE_PING = 0x07;
    public const TYPE_ERROR = 0x08;

    // Flags
    public const FLAG_COMPRESSED = 0x01;
    public const FLAG_CHUNKED = 0x02;
    public const FLAG_FINAL = 0x04;

    /**
     * Read a frame from the given stream (default: STDIN).
     */
    public static function readFrame($stream = null): Frame
    {
        $stream = $stream ?? STDIN;

        $header = self::readExact($stream, self::HEADER_SIZE);

        // Validate magic
        if ($header[0] !== self::MAGIC[0] || $header[1] !== self::MAGIC[1]) {
            throw new \RuntimeException(sprintf(
                'Invalid magic bytes: 0x%02x%02x',
                ord($header[0]),
                ord($header[1])
            ));
        }

        // Validate version
        if (ord($header[2]) !== self::VERSION) {
            throw new \RuntimeException('Unsupported protocol version: ' . ord($header[2]));
        }

        $type = ord($header[3]);
        $flags = ord($header[4]);
        $streamId = unpack('n', substr($header, 5, 2))[1]; // big-endian uint16

        // Header size as 3 bytes (big-endian uint24)
        $hdrSize = (ord($header[7]) << 16) | (ord($header[8]) << 8) | ord($header[9]);

        // Payload size as 4 bytes (big-endian uint32)
        $payloadSize = unpack('N', substr($header, 10, 4))[1];

        $headers = '';
        if ($hdrSize > 0) {
            $headers = self::readExact($stream, $hdrSize);
        }

        $payload = '';
        if ($payloadSize > 0) {
            $payload = self::readExact($stream, $payloadSize);
        }

        return new Frame($type, $flags, $streamId, $headers, $payload);
    }

    /**
     * Write a frame to the given stream (default: STDOUT).
     */
    public static function writeFrame(Frame $frame, $stream = null): void
    {
        $stream = $stream ?? STDOUT;

        $hdrSize = strlen($frame->headers);
        $payloadSize = strlen($frame->payload);

        $header = self::MAGIC;
        $header .= chr(self::VERSION);
        $header .= chr($frame->type);
        $header .= chr($frame->flags);
        $header .= pack('n', $frame->streamId); // big-endian uint16

        // Header size as 3 bytes (big-endian uint24)
        $header .= chr(($hdrSize >> 16) & 0xFF);
        $header .= chr(($hdrSize >> 8) & 0xFF);
        $header .= chr($hdrSize & 0xFF);

        $header .= pack('N', $payloadSize); // big-endian uint32

        fwrite($stream, $header);
        if ($hdrSize > 0) {
            fwrite($stream, $frame->headers);
        }
        if ($payloadSize > 0) {
            fwrite($stream, $frame->payload);
        }
        fflush($stream);
    }

    /**
     * Read exact number of bytes from stream.
     */
    private static function readExact($stream, int $length): string
    {
        $data = '';
        $remaining = $length;
        while ($remaining > 0) {
            $chunk = fread($stream, $remaining);
            if ($chunk === false || $chunk === '') {
                throw new \RuntimeException("Stream closed: expected $length bytes, got " . strlen($data));
            }
            $data .= $chunk;
            $remaining -= strlen($chunk);
        }
        return $data;
    }
}
