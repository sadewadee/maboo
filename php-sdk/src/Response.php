<?php

declare(strict_types=1);

namespace Maboo;

use Maboo\Protocol\Frame;
use Maboo\Protocol\Msgpack;
use Maboo\Protocol\Wire;

class Response
{
    private int $status = 200;
    private array $headers = ['Content-Type' => 'text/html; charset=UTF-8'];
    private string $body = '';

    public function status(int $code): self
    {
        $this->status = $code;
        return $this;
    }

    public function header(string $name, string $value): self
    {
        $this->headers[$name] = $value;
        return $this;
    }

    public function body(string $content): self
    {
        $this->body = $content;
        return $this;
    }

    public function json(mixed $data, int $status = 200): self
    {
        $this->status = $status;
        $this->headers['Content-Type'] = 'application/json';
        $this->body = json_encode($data, JSON_THROW_ON_ERROR | JSON_UNESCAPED_UNICODE);
        return $this;
    }

    public function html(string $content, int $status = 200): self
    {
        $this->status = $status;
        $this->headers['Content-Type'] = 'text/html; charset=UTF-8';
        $this->body = $content;
        return $this;
    }

    public function redirect(string $url, int $status = 302): self
    {
        $this->status = $status;
        $this->headers['Location'] = $url;
        return $this;
    }

    /**
     * Send the response back to the Go server via protocol.
     */
    public function send($stream = null): void
    {
        $headerData = Msgpack::encode([
            'status' => $this->status,
            'headers' => $this->headers,
        ]);

        $frame = new Frame(
            type: Wire::TYPE_RESPONSE,
            flags: 0,
            streamId: 0,
            headers: $headerData,
            payload: $this->body,
        );

        Wire::writeFrame($frame, $stream);
    }
}
