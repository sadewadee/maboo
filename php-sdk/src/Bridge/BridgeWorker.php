<?php

declare(strict_types=1);

namespace Maboo\Bridge;

use Maboo\Request;
use Maboo\Response;
use Maboo\Worker;

/**
 * BridgeWorker wraps a Worker + Bridge together for easy framework integration.
 *
 * Usage:
 *   $worker = new BridgeWorker(new LaravelBridge('/path/to/laravel'));
 *   $worker->run();
 */
class BridgeWorker
{
    private Worker $worker;

    public function __construct(
        private readonly BridgeInterface $bridge,
        int $maxMemory = 128 * 1024 * 1024,
    ) {
        $this->worker = new Worker($maxMemory);
    }

    public function run(): void
    {
        // Bootstrap framework once
        $this->bridge->bootstrap();

        $this->worker->onRequest(function (Request $request, Response $response) {
            try {
                $this->bridge->handle($request, $response);
            } finally {
                // ALWAYS clean up state between requests
                $this->bridge->cleanup();
            }
        });

        try {
            $this->worker->run();
        } finally {
            $this->bridge->terminate();
        }
    }
}
