# Maboo - PHP Application Server Written in Go

**Date:** 2026-02-22
**Status:** Approved

## Overview

Maboo is a PHP application server written in Go that solves the fundamental problems
of FrankenPHP by using a process-based architecture instead of CGo embedding. It provides
native WebSocket support, seamless integration with all major PHP frameworks, and automatic
worker lifecycle management.

## Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Go-PHP interaction | Process-based (no CGo) | Full extension compat, no ZTS, process isolation |
| Communication | Custom binary protocol | Lower overhead than FastCGI, stream multiplexing |
| Transport | stdin/stdout pipes (default) | Zero-copy, lowest latency for same-machine |
| Framework support | All major frameworks via adapters | PSR-7/PSR-15 core + framework-specific bridges |
| Key differentiator | Native WebSocket + Real-time | FrankenPHP's biggest missing feature |

## Architecture

```
[Client] → [Go HTTP Server (HTTP/1.1, HTTP/2, HTTP/3)]
                    ↓
            [Router + Middleware]
                    ↓
        ┌───────────┼───────────┐
        ↓           ↓           ↓
   [Static]    [PHP Handler] [WS Handler]
                    ↓           ↓
            [Worker Pool Manager]
                    ↓
    [PHP Worker 1] ... [PHP Worker N] [WS Worker]
    (Binary Protocol over stdin/stdout pipes)
```

## Wire Protocol (maboo-wire v1)

```
Frame Format (14 bytes header):
┌─────────────────────────────────┐
│ Magic (2B): 0x4D42 ("MB")      │
│ Version (1B): 0x01              │
│ Type (1B)                       │
│ Flags (1B)                      │
│ Stream ID (2B)                  │
│ Header Size (3B): up to 16MB   │
│ Payload Size (4B): up to 4GB   │
├─────────────────────────────────┤
│ Headers (msgpack encoded)       │
├─────────────────────────────────┤
│ Payload (raw bytes)             │
└─────────────────────────────────┘

Message Types:
  0x01 = REQUEST       (Go → PHP)
  0x02 = RESPONSE      (PHP → Go)
  0x03 = STREAM_DATA   (bidirectional WebSocket)
  0x04 = STREAM_CLOSE
  0x05 = WORKER_READY
  0x06 = WORKER_STOP
  0x07 = PING/PONG
  0x08 = ERROR

Transport: pipes (default), Unix socket, TCP
```

## Worker Pool Manager

```
Lifecycle: IDLE → BUSY → IDLE → RECYCLE (at max_jobs/max_memory)
                   ↓
              RESTART (on crash/timeout)

Scaling:
  min_workers: 4
  max_workers: 32
  scale_up_threshold: 80%
  scale_down_threshold: 20%
  worker_max_jobs: 10000
  worker_max_memory: 128M
```

## WebSocket Architecture

Go manages connections, rooms, and broadcasting. PHP handles business logic.
Dedicated WebSocket workers run persistently with event-driven handler pattern.

```php
$ws = new Maboo\WebSocket\Server();
$ws->onConnect(function($conn) { ... });
$ws->onMessage(function($conn, $msg) { ... });
$ws->onClose(function($conn) { ... });
$ws->run();
```

## Framework Integration

PHP SDK (`maboo/php-sdk`) via Composer with:
- Core: Worker, Request, Response, Protocol, WebSocket
- PSR-7/PSR-15 bridge (any PSR-compliant framework works)
- Framework-specific adapters:
  - Laravel, Symfony, WordPress, Yii, Slim, CodeIgniter, CakePHP, Laminas

Each bridge handles:
1. One-time framework bootstrap
2. Request conversion to framework format
3. State cleanup between requests
4. Graceful shutdown

## Configuration (maboo.yaml)

```yaml
server:
  address: "0.0.0.0:8080"
  tls:
    auto: true
  http3: true

php:
  binary: "php"
  worker: "public/worker.php"

pool:
  min_workers: 4
  max_workers: 32
  max_jobs: 10000
  max_memory: "128M"

websocket:
  enabled: true
  path: "/ws"
  worker: "ws-worker.php"

static:
  root: "public"

logging:
  level: "info"
  format: "json"

metrics:
  enabled: true
  path: "/metrics"
```

## Project Structure

```
maboo/
├── cmd/maboo/main.go
├── internal/
│   ├── server/     (HTTP server, router, middleware, TLS)
│   ├── pool/       (worker pool, scaler, watchdog)
│   ├── protocol/   (wire protocol, request/response serialization)
│   ├── websocket/  (WS handler, rooms, pub/sub)
│   └── config/     (YAML config, defaults)
├── pkg/maboo/      (public API for embedding)
├── php-sdk/        (Composer package with framework bridges)
├── docs/
├── go.mod
├── Makefile
├── Dockerfile
└── maboo.yaml.example
```

## FrankenPHP Problems Solved

| FrankenPHP Problem | Maboo Solution |
|---|---|
| Memory leaks (#1797) | Worker recycling + memory watchdog |
| zend_mm_heap corrupted (#1530) | Process isolation per worker |
| CGo overhead | No CGo - binary protocol |
| ZTS required | Standard PHP, all extensions work |
| No WebSocket (#1888) | Native WebSocket with dedicated workers |
| dd() causes 500 | Worker isolation - crashes don't affect server |
| No Windows (#880) | Process-based works everywhere |
| DB "gone away" | Per-worker connections, clean recycling |
| Laravel Octane BETA | First-class Laravel bridge |
| Worker state pollution | Bridges manage explicit state cleanup |
