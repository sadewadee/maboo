<?php

/**
 * Example maboo WebSocket worker.
 *
 * Demonstrates native WebSocket support - FrankenPHP's biggest missing feature.
 */

require_once __DIR__ . '/../php-sdk/src/Protocol/Msgpack.php';
require_once __DIR__ . '/../php-sdk/src/Protocol/Frame.php';
require_once __DIR__ . '/../php-sdk/src/Protocol/Wire.php';
require_once __DIR__ . '/../php-sdk/src/WebSocket/Connection.php';
require_once __DIR__ . '/../php-sdk/src/WebSocket/Server.php';

use Maboo\WebSocket\Connection;
use Maboo\WebSocket\Server;

$ws = new Server();

$ws->onConnect(function (Connection $conn) {
    echo "Client connected: {$conn->id}\n";
    $conn->send(json_encode(['type' => 'welcome', 'message' => 'Connected to maboo WebSocket!']));
});

$ws->onMessage(function (Connection $conn, string $message) use ($ws) {
    $data = json_decode($message, true);

    if ($data && isset($data['type'])) {
        match ($data['type']) {
            'ping' => $conn->send(json_encode(['type' => 'pong', 'time' => date('c')])),
            'broadcast' => $ws->broadcast(json_encode([
                'type' => 'broadcast',
                'from' => $conn->id,
                'message' => $data['message'] ?? '',
            ]), $conn->id),
            default => $conn->send(json_encode(['type' => 'echo', 'data' => $data])),
        };
    } else {
        $conn->send(json_encode(['type' => 'echo', 'raw' => $message]));
    }
});

$ws->onClose(function (Connection $conn) {
    echo "Client disconnected: {$conn->id}\n";
});

$ws->onError(function (\Throwable $e) {
    echo "WebSocket error: {$e->getMessage()}\n";
});

$ws->run();
