<?php

/**
 * Example maboo PHP worker.
 *
 * This is a simple HTTP request handler that demonstrates how to use
 * the maboo PHP SDK to handle requests from the Go server.
 *
 * Usage: Referenced in maboo.yaml as the worker script.
 */

require_once __DIR__ . '/../php-sdk/src/Protocol/Msgpack.php';
require_once __DIR__ . '/../php-sdk/src/Protocol/Frame.php';
require_once __DIR__ . '/../php-sdk/src/Protocol/Wire.php';
require_once __DIR__ . '/../php-sdk/src/Request.php';
require_once __DIR__ . '/../php-sdk/src/Response.php';
require_once __DIR__ . '/../php-sdk/src/Worker.php';

use Maboo\Request;
use Maboo\Response;
use Maboo\Worker;

$worker = new Worker();

$worker->onRequest(function (Request $request, Response $response) {
    $path = $request->uri;

    match (true) {
        $path === '/' => $response->html('<h1>Welcome to Maboo!</h1><p>PHP Application Server powered by Go.</p>'),
        $path === '/api/info' => $response->json([
            'server' => 'maboo',
            'php_version' => PHP_VERSION,
            'time' => date('c'),
            'memory' => memory_get_usage(true),
            'pid' => getmypid(),
        ]),
        $path === '/api/echo' => $response->json([
            'method' => $request->method,
            'uri' => $request->uri,
            'query' => $request->query(),
            'headers' => $request->headers,
            'body' => $request->body,
        ]),
        default => $response->status(404)->json(['error' => 'Not Found', 'path' => $path]),
    };
});

$worker->run();
