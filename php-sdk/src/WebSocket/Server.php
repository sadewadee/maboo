<?php

declare(strict_types=1);

namespace Maboo\WebSocket;

use Maboo\Protocol\Frame;
use Maboo\Protocol\Wire;

class Server
{
    private ?\Closure $onConnect = null;
    private ?\Closure $onMessage = null;
    private ?\Closure $onClose = null;
    private ?\Closure $onError = null;

    /** @var array<string, Connection> */
    private array $connections = [];

    public function onConnect(\Closure $handler): self
    {
        $this->onConnect = $handler;
        return $this;
    }

    public function onMessage(\Closure $handler): self
    {
        $this->onMessage = $handler;
        return $this;
    }

    public function onClose(\Closure $handler): self
    {
        $this->onClose = $handler;
        return $this;
    }

    public function onError(\Closure $handler): self
    {
        $this->onError = $handler;
        return $this;
    }

    /**
     * Get all active connections.
     *
     * @return array<string, Connection>
     */
    public function connections(): array
    {
        return $this->connections;
    }

    /**
     * Broadcast a message to all connected clients.
     */
    public function broadcast(string $data, ?string $excludeId = null): void
    {
        foreach ($this->connections as $conn) {
            if ($excludeId !== null && $conn->id === $excludeId) {
                continue;
            }
            $conn->send($data);
        }
    }

    /**
     * Start the WebSocket worker loop.
     */
    public function run(): void
    {
        // Signal ready
        Wire::writeFrame(new Frame(
            type: Wire::TYPE_WORKER_READY,
            flags: 0,
            streamId: 0,
            headers: '',
            payload: '',
        ));

        while (true) {
            try {
                $frame = Wire::readFrame();
            } catch (\Throwable) {
                break;
            }

            if ($frame->type === Wire::TYPE_WORKER_STOP) {
                break;
            }

            if ($frame->type === Wire::TYPE_PING) {
                Wire::writeFrame(new Frame(
                    type: Wire::TYPE_PING,
                    flags: 0,
                    streamId: 0,
                    headers: '',
                    payload: 'pong',
                ));
                continue;
            }

            if ($frame->type === Wire::TYPE_STREAM_DATA || $frame->type === Wire::TYPE_STREAM_CLOSE) {
                $this->handleStream($frame);
            }
        }
    }

    private function handleStream(Frame $frame): void
    {
        try {
            $header = $frame->decodeHeaders();
            $connId = $header['conn_id'] ?? '';
            $event = $header['event'] ?? '';

            switch ($event) {
                case 'connect':
                    $conn = new Connection($connId);
                    $this->connections[$connId] = $conn;
                    if ($this->onConnect) {
                        ($this->onConnect)($conn);
                    }
                    break;

                case 'message':
                    $conn = $this->connections[$connId] ?? new Connection($connId);
                    if ($this->onMessage) {
                        ($this->onMessage)($conn, $frame->payload);
                    }
                    break;

                case 'close':
                    $conn = $this->connections[$connId] ?? new Connection($connId);
                    unset($this->connections[$connId]);
                    if ($this->onClose) {
                        ($this->onClose)($conn);
                    }
                    break;
            }
        } catch (\Throwable $e) {
            if ($this->onError) {
                ($this->onError)($e);
            }
        }
    }
}
