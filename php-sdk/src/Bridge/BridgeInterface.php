<?php

declare(strict_types=1);

namespace Maboo\Bridge;

use Maboo\Request;
use Maboo\Response;

/**
 * BridgeInterface defines the contract for PHP framework bridges.
 *
 * Each bridge bootstraps a framework once, then handles multiple requests
 * with proper state cleanup between them - solving FrankenPHP's state pollution.
 */
interface BridgeInterface
{
    /**
     * Bootstrap the framework (called once on worker start).
     */
    public function bootstrap(): void;

    /**
     * Handle an HTTP request through the framework.
     */
    public function handle(Request $request, Response $response): void;

    /**
     * Clean up state between requests (CRITICAL for preventing state pollution).
     */
    public function cleanup(): void;

    /**
     * Gracefully terminate the framework (called on worker shutdown).
     */
    public function terminate(): void;
}
