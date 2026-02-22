<?php

declare(strict_types=1);

namespace Maboo\WebSocket;

use Maboo\Protocol\Frame;
use Maboo\Protocol\Msgpack;
use Maboo\Protocol\Wire;

class Connection
{
    public function __construct(
        public readonly string $id,
        public readonly string $remoteAddr = '',
    ) {}

    /**
     * Send a message to this WebSocket connection via the Go server.
     */
    public function send(string $data, $stream = null): void
    {
        $headerData = Msgpack::encode([
            'conn_id' => $this->id,
            'event' => 'message',
            'room' => '',
        ]);

        Wire::writeFrame(new Frame(
            type: Wire::TYPE_STREAM_DATA,
            flags: 0,
            streamId: 0,
            headers: $headerData,
            payload: $data,
        ), $stream);
    }

    /**
     * Send a JSON message to this connection.
     */
    public function sendJson(mixed $data, $stream = null): void
    {
        $this->send(json_encode($data, JSON_THROW_ON_ERROR | JSON_UNESCAPED_UNICODE), $stream);
    }

    /**
     * Close this WebSocket connection.
     */
    public function close($stream = null): void
    {
        $headerData = Msgpack::encode([
            'conn_id' => $this->id,
            'event' => 'close',
            'room' => '',
        ]);

        Wire::writeFrame(new Frame(
            type: Wire::TYPE_STREAM_CLOSE,
            flags: 0,
            streamId: 0,
            headers: $headerData,
            payload: '',
        ), $stream);
    }
}
