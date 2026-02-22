<?php

declare(strict_types=1);

namespace Maboo;

class Request
{
    public function __construct(
        public readonly string $method,
        public readonly string $uri,
        public readonly string $queryString,
        public readonly array $headers,
        public readonly string $body,
        public readonly string $remoteAddr,
        public readonly string $serverName,
        public readonly string $serverPort,
        public readonly string $protocol,
    ) {}

    /**
     * Create a Request from a protocol frame's decoded headers + payload.
     */
    public static function fromFrame(array $headerData, string $payload): self
    {
        return new self(
            method: $headerData['method'] ?? 'GET',
            uri: $headerData['uri'] ?? '/',
            queryString: $headerData['query_string'] ?? '',
            headers: $headerData['headers'] ?? [],
            body: $payload,
            remoteAddr: $headerData['remote_addr'] ?? '',
            serverName: $headerData['server_name'] ?? '',
            serverPort: $headerData['server_port'] ?? '8080',
            protocol: $headerData['protocol'] ?? 'HTTP/1.1',
        );
    }

    /**
     * Get a specific header value (case-insensitive).
     */
    public function header(string $name, string $default = ''): string
    {
        $name = strtolower($name);
        foreach ($this->headers as $key => $value) {
            if (strtolower($key) === $name) {
                return $value;
            }
        }
        return $default;
    }

    /**
     * Get parsed query parameters.
     */
    public function query(): array
    {
        parse_str($this->queryString, $params);
        return $params;
    }

    /**
     * Build $_SERVER superglobal equivalent.
     */
    public function toServerVars(): array
    {
        $server = [
            'REQUEST_METHOD' => $this->method,
            'REQUEST_URI' => $this->uri . ($this->queryString ? '?' . $this->queryString : ''),
            'QUERY_STRING' => $this->queryString,
            'SERVER_NAME' => $this->serverName,
            'SERVER_PORT' => $this->serverPort,
            'SERVER_PROTOCOL' => $this->protocol,
            'REMOTE_ADDR' => $this->remoteAddr,
            'SCRIPT_NAME' => $this->uri,
            'DOCUMENT_ROOT' => getcwd(),
        ];
        foreach ($this->headers as $key => $value) {
            $server['HTTP_' . strtoupper(str_replace('-', '_', $key))] = $value;
        }
        if ($ct = $this->header('content-type')) {
            $server['CONTENT_TYPE'] = $ct;
        }
        if ($cl = $this->header('content-length')) {
            $server['CONTENT_LENGTH'] = $cl;
        }
        return $server;
    }
}
