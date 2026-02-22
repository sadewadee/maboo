<?php

declare(strict_types=1);

namespace Maboo\Bridge;

use Maboo\Request;
use Maboo\Response;

/**
 * Generic PSR-7/PSR-15 bridge.
 *
 * Works with any framework that implements PSR-15 RequestHandlerInterface:
 * Slim, Mezzio (Laminas), Yii, CodeIgniter 4, CakePHP, etc.
 *
 * Usage:
 *   $handler = new Slim\App(); // or any PSR-15 handler
 *   $bridge = new PSR7($handler);
 */
class PSR7 implements BridgeInterface
{
    private object $handler; // Psr\Http\Server\RequestHandlerInterface

    public function __construct(object $handler)
    {
        $this->handler = $handler;
    }

    public function bootstrap(): void
    {
        // PSR-15 handlers are typically already bootstrapped
    }

    public function handle(Request $request, Response $response): void
    {
        // Convert to PSR-7 ServerRequest
        $psrRequest = $this->createServerRequest($request);

        // Handle through PSR-15 handler
        $psrResponse = $this->handler->handle($psrRequest);

        // Convert back to Maboo response
        $response->status($psrResponse->getStatusCode());

        foreach ($psrResponse->getHeaders() as $name => $values) {
            $response->header($name, implode(', ', $values));
        }

        $body = $psrResponse->getBody();
        $body->rewind();
        $response->body($body->getContents());
        $response->send();
    }

    public function cleanup(): void
    {
        // Reset superglobals
        $_GET = [];
        $_POST = [];
        $_REQUEST = [];
        $_COOKIE = [];
        $_FILES = [];
        $_SERVER = [];
    }

    public function terminate(): void
    {
        // Nothing to clean up for generic PSR-15
    }

    private function createServerRequest(Request $request): object
    {
        // Build a PSR-7 ServerRequest using the standard interface
        // This uses whatever PSR-7 implementation is available (nyholm, guzzle, etc.)
        $serverParams = $request->toServerVars();
        $uri = $request->uri . ($request->queryString ? '?' . $request->queryString : '');

        // Try Nyholm (most lightweight)
        if (class_exists(\Nyholm\Psr7\ServerRequest::class)) {
            return new \Nyholm\Psr7\ServerRequest(
                $request->method,
                $uri,
                $request->headers,
                $request->body,
                $request->protocol,
                $serverParams,
            );
        }

        // Try GuzzleHttp
        if (class_exists(\GuzzleHttp\Psr7\ServerRequest::class)) {
            return new \GuzzleHttp\Psr7\ServerRequest(
                $request->method,
                $uri,
                $request->headers,
                $request->body,
                $request->protocol,
                $serverParams,
            );
        }

        throw new \RuntimeException(
            'No PSR-7 implementation found. Install nyholm/psr7 or guzzlehttp/psr7.'
        );
    }
}
