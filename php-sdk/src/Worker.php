<?php

declare(strict_types=1);

namespace Maboo;

use Maboo\Protocol\Frame;
use Maboo\Protocol\Wire;

class Worker
{
    private ?\Closure $handler = null;
    private int $requestCount = 0;
    private int $maxRequests;
    private int $maxMemory;

    public function __construct(int $maxMemory = 128 * 1024 * 1024)
    {
        $this->maxMemory = $maxMemory;
        $this->maxRequests = (int)($_SERVER['MAX_REQUESTS'] ?? 0);
    }

    /**
     * Register the request handler.
     */
    public function onRequest(\Closure $handler): self
    {
        $this->handler = $handler;
        return $this;
    }

    /**
     * Start the worker loop.
     */
    public function run(): void
    {
        if ($this->handler === null) {
            throw new \RuntimeException('No request handler registered. Call onRequest() before run().');
        }

        // Signal to Go server that worker is ready
        $this->sendReady();

        while (true) {
            try {
                $frame = Wire::readFrame();
            } catch (\Throwable) {
                // stdin closed = server wants us to stop
                break;
            }

            if ($frame->type === Wire::TYPE_WORKER_STOP) {
                break;
            }

            if ($frame->type === Wire::TYPE_PING) {
                $this->sendPong();
                continue;
            }

            if ($frame->type === Wire::TYPE_REQUEST) {
                $this->handleRequest($frame);
                $this->requestCount++;
            }

            // Check limits
            if ($this->maxRequests > 0 && $this->requestCount >= $this->maxRequests) {
                break;
            }
            if (memory_get_usage(true) > $this->maxMemory) {
                break;
            }

            // Collect cycles between requests to prevent memory leaks
            gc_collect_cycles();

            // Signal ready for next request
            $this->sendReady();
        }
    }

    private function handleRequest(Frame $frame): void
    {
        try {
            $headerData = $frame->decodeHeaders();
            $request = Request::fromFrame($headerData, $frame->payload);

            // Populate PHP superglobals
            $_SERVER = $request->toServerVars();
            $_GET = $request->query();
            $_POST = [];
            $_REQUEST = [];
            $_COOKIE = [];
            $_FILES = [];

            // Parse POST data
            if (
                $request->method === 'POST' &&
                str_contains($request->header('content-type'), 'application/x-www-form-urlencoded')
            ) {
                parse_str($request->body, $_POST);
            }

            $_REQUEST = array_merge($_GET, $_POST);

            // Parse cookies
            $cookieHeader = $request->header('cookie');
            if ($cookieHeader !== '') {
                foreach (explode(';', $cookieHeader) as $cookie) {
                    $parts = explode('=', trim($cookie), 2);
                    if (count($parts) === 2) {
                        $_COOKIE[trim($parts[0])] = urldecode(trim($parts[1]));
                    }
                }
            }

            $response = new Response();
            ($this->handler)($request, $response);
            $response->send();
        } catch (\Throwable $e) {
            $response = new Response();
            $response->status(500)
                ->header('Content-Type', 'text/plain')
                ->body("Internal Server Error: " . $e->getMessage() . "\n" . $e->getTraceAsString())
                ->send();
        }
    }

    private function sendReady(): void
    {
        Wire::writeFrame(new Frame(
            type: Wire::TYPE_WORKER_READY,
            flags: 0,
            streamId: 0,
            headers: '',
            payload: '',
        ));
    }

    private function sendPong(): void
    {
        Wire::writeFrame(new Frame(
            type: Wire::TYPE_PING,
            flags: 0,
            streamId: 0,
            headers: '',
            payload: 'pong',
        ));
    }
}
