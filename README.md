# Maboo

<p align="center">
  <img src="assets/logo.svg" alt="Maboo Logo" width="180">
</p>

<p align="center">
  <strong>High-Performance PHP Application Server</strong>
</p>

<p align="center">
  <a href="#performance">Benchmarks</a> •
  <a href="#quick-start">Quick Start</a> •
  <a href="#configuration">Configuration</a> •
  <a href="#php-worker">PHP Worker</a>
</p>

---

A high-performance PHP application server written in Go. Maboo keeps PHP workers alive as long-running processes and communicates via a custom binary protocol, eliminating per-request bootstrap overhead.

## Why Maboo?

Traditional PHP-FPM spawns a new PHP process for each request, loading the entire framework on every request. Maboo solves this by:

- **Persistent workers** — PHP workers boot once and handle thousands of requests
- **Binary protocol** — Efficient msgpack-based communication between Go and PHP
- **Low overhead** — Pooled resources, zero-allocation hot paths
- **Graceful operations** — Zero-downtime reload, graceful shutdown

## Features

- **Worker Pool Management** — Auto-scaling min/max workers, idle timeout, memory limits
- **Binary Wire Protocol** — msgpack-encoded headers, zero-copy payload transfer
- **Gzip Compression** — Pooled writers with lazy buffering (~1 GB/s throughput)
- **HTTP/103 Early Hints** — `rel=preload` / `rel=preconnect` support
- **WebSocket** — Full duplex via dedicated PHP worker
- **Prometheus Metrics** — Request histograms, worker gauges, memory stats
- **Auto-TLS** — Self-signed cert for development or bring your own
- **File Watcher** — Auto-reload workers on PHP changes (dev mode)
- **Zero-downtime Reload** — `SIGUSR1` for graceful worker rotation
- **Static File Serving** — With configurable `Cache-Control`
- **Health Checks** — `/health`, `/healthz`, `/ready`, `/readyz`
- **Framework Bridges** — Laravel, Symfony, WordPress, PSR-7

## Performance

Benchmarks run on Apple M1 (arm64), Go 1.25, tested with `-race` detector.

### HTTP Middleware

| Benchmark | Throughput | Latency | Memory | Allocs |
|-----------|------------|---------|--------|--------|
| Small response (gzip) | 8.4M req/s | 447 ns | 1 KB | 10 |
| Large response (gzip) | 243K req/s | 16.1 µs | 17.7 KB | 14 |
| No compression | 3.3M req/s | 1.1 µs | 3 KB | 9 |
| Full middleware stack | 403K req/s | 9.1 µs | 12.3 KB | 31 |
| Health endpoint | 9.6M req/s | 374 ns | 976 B | 9 |

### Gzip Compression

| Benchmark | Throughput | Memory/op | Allocs/op |
|-----------|------------|-----------|-----------|
| **Pooled (BestSpeed)** | **1,064 MB/s** | 7 B | 0 |
| No Pool (Default) | 237 MB/s | 813,857 B | 17 |
| **Improvement** | **4.5x faster** | **99.999% less** | **-17 allocs** |

### Wire Protocol

| Benchmark | Throughput | Latency | Memory | Allocs |
|-----------|------------|---------|--------|--------|
| WriteFrame | 176M ops/s | 22 ns | 16 B | 1 |
| ReadFrame | 6.9M ops/s | 538 ns | 4.3 KB | 5 |
| Ping/Pong roundtrip | 43M ops/s | 85 ns | 104 B | 5 |
| Write/Read roundtrip | 30M ops/s | 123 ns | 336 B | 5 |
| msgpack decode | 26M ops/s | 139 ns | 144 B | 4 |

### Optimizations Applied

1. **sync.Pool for gzip.Writer** — Reuses writers, eliminates 813 KB/op allocation
2. **Pooled response writers** — Single wrapper for all middleware layers
3. **Stack-allocated slog attrs** — Fixed-size array instead of variadic spread
4. **Pooled wire buffers** — Header + payload coalesced into single write
5. **Lazy compression buffering** — Only allocate when compression threshold met
6. **BestSpeed compression level** — 4.5x throughput with ~5% size trade-off

### Stability

All components tested with Go race detector:

```bash
go test -race -count=1 ./...
# ok  github.com/sadewadee/maboo/internal/config
# ok  github.com/sadewadee/maboo/internal/pool
# ok  github.com/sadewadee/maboo/internal/protocol
# ok  github.com/sadewadee/maboo/internal/server
```

## Quick Start

### 1. Build

```bash
make build
# or
go build -o maboo ./cmd/maboo
```

### 2. Configure

```bash
cp maboo.yaml.example maboo.yaml
```

### 3. Run

```bash
./maboo serve
```

The server starts on `http://0.0.0.0:8080` by default.

## Project Structure

```
maboo/
├── cmd/maboo/              # CLI entry point
├── internal/
│   ├── config/             # YAML config loader
│   ├── pool/               # Worker pool manager
│   ├── protocol/           # Binary wire protocol
│   ├── server/             # HTTP server, middleware, health, metrics
│   └── websocket/          # WebSocket handler
├── php-sdk/
│   └── src/
│       ├── Protocol/       # Frame, Wire, Msgpack
│       ├── Bridge/         # Laravel, Symfony, WordPress, PSR-7
│       ├── WebSocket/      # Connection handler
│       ├── Worker.php      # Main worker loop
│       ├── Request.php     # HTTP request object
│       └── Response.php    # HTTP response builder
├── examples/
│   ├── worker.php          # Basic HTTP worker
│   └── ws-worker.php       # WebSocket worker
├── maboo.yaml.example      # Example configuration
├── Dockerfile
├── Makefile
└── README.md
```

## Configuration

| Section | Key | Default | Description |
|---------|-----|---------|-------------|
| `server.address` | `0.0.0.0:8080` | Listen address |
| `server.tls.auto` | `false` | Auto-generate self-signed cert |
| `server.tls.cert` | `""` | Path to TLS certificate |
| `server.tls.key` | `""` | Path to TLS private key |
| `php.binary` | `php` | PHP CLI binary path |
| `php.worker` | `examples/worker.php` | Worker script path |
| `pool.min_workers` | `4` | Minimum workers |
| `pool.max_workers` | `32` | Maximum workers |
| `pool.max_jobs` | `10000` | Requests per worker before restart |
| `pool.max_memory` | `128M` | Memory limit per worker |
| `pool.idle_timeout` | `60s` | Kill idle workers after |
| `pool.request_timeout` | `30s` | Max request handling time |
| `websocket.enabled` | `false` | Enable WebSocket |
| `static.root` | `public` | Static files directory |
| `logging.level` | `info` | Log level (debug/info/warn/error) |
| `logging.format` | `json` | Log format (json/text) |
| `metrics.enabled` | `true` | Enable Prometheus metrics |
| `watch.enabled` | `false` | Enable file watcher |

## PHP Worker

### Basic Example

```php
<?php

require_once __DIR__ . '/../php-sdk/src/Worker.php';
require_once __DIR__ . '/../php-sdk/src/Request.php';
require_once __DIR__ . '/../php-sdk/src/Response.php';

use Maboo\Worker;
use Maboo\Request;
use Maboo\Response;

$worker = new Worker();

$worker->onRequest(function (Request $request, Response $response) {
    match ($request->uri) {
        '/' => $response->html('<h1>Hello from Maboo!</h1>'),

        '/api/info' => $response->json([
            'php_version' => PHP_VERSION,
            'pid' => getmypid(),
            'memory' => memory_get_usage(true),
        ]),

        '/api/echo' => $response->json([
            'method' => $request->method,
            'uri' => $request->uri,
            'query' => $request->query(),
            'headers' => $request->headers,
            'body' => $request->body,
        ]),

        default => $response->status(404)->json(['error' => 'Not Found']),
    };
});

$worker->run();
```

### Request Object

```php
$request->method;      // HTTP method (GET, POST, etc.)
$request->uri;         // Request URI path
$request->query();     // Query parameters as array
$request->headers;     // Request headers
$request->body;        // Raw request body
$request->remote_addr; // Client IP address
```

### Response Object

```php
$response->status(200);                    // Set status code
$response->header('X-Custom', 'value');    // Set header
$response->html('<h1>Hello</h1>');         // HTML response
$response->json(['key' => 'value']);       // JSON response
$response->redirect('/new-path');          // Redirect
```

## Framework Bridges

### Laravel

```php
<?php

require_once __DIR__ . '/../php-sdk/src/Bridge/Laravel.php';
require_once __DIR__ . '/../php-sdk/src/Worker.php';

use Maboo\Bridge\Laravel;
use Maboo\Worker;

$worker = new Worker();
$bridge = new Laravel(__DIR__ . '/../');

$worker->onRequest(function ($request, $response) use ($bridge) {
    $bridge->handle($request, $response);
});

$worker->run();
```

### Symfony

```php
<?php

require_once __DIR__ . '/../php-sdk/src/Bridge/Symfony.php';
require_once __DIR__ . '/../php-sdk/src/Worker.php';

use Maboo\Bridge\Symfony;
use Maboo\Worker;

$worker = new Worker();
$bridge = new Symfony(__DIR__ . '/../');

$worker->onRequest(function ($request, $response) use ($bridge) {
    $bridge->handle($request, $response);
});

$worker->run();
```

### WordPress

```php
<?php

require_once __DIR__ . '/../php-sdk/src/Bridge/WordPress.php';
require_once __DIR__ . '/../php-sdk/src/Worker.php';

use Maboo\Bridge\WordPress;
use Maboo\Worker;

$worker = new Worker();
$bridge = new WordPress(__DIR__ . '/../');

$worker->onRequest(function ($request, $response) use ($bridge) {
    $bridge->handle($request, $response);
});

$worker->run();
```

## Wire Protocol

Maboo uses a custom binary protocol for Go ↔ PHP communication:

```
┌───────────────────────────────────────────────────────────────┐
│ Magic (2B) │ Version (1B) │ Type (1B) │ Flags (1B) │ StreamID (2B) │
├───────────────────────────────────────────────────────────────┤
│ HeaderLen (3B) │ PayloadLen (4B) │ Headers (msgpack) │ Payload │
└───────────────────────────────────────────────────────────────┘
```

- **14-byte fixed header** — Minimal overhead per frame
- **msgpack headers** — ~40% smaller than JSON
- **Frame types**: Request, Response, StreamData, StreamClose, WorkerReady, WorkerStop, Ping, Error

## Signals

| Signal | Action |
|--------|--------|
| `SIGINT` | Graceful shutdown |
| `SIGTERM` | Graceful shutdown |
| `SIGUSR1` | Zero-downtime worker reload |

## Endpoints

| Path | Description |
|------|-------------|
| `/health` | Health check (always 200) |
| `/healthz` | Liveness probe |
| `/ready` | Readiness probe (checks worker pool) |
| `/readyz` | Readiness probe |
| `/metrics` | Prometheus metrics (if enabled) |

## Docker

```bash
# Build
docker build -t maboo .

# Run
docker run -p 8080:8080 -v $(pwd):/app maboo serve

# With custom config
docker run -p 8080:8080 -v $(pwd):/app maboo serve maboo.yaml
```

## Development

```bash
# Build
make build

# Build with version
VERSION=1.0.0 make build

# Build optimized
go build -trimpath -ldflags "-s -w" -o maboo ./cmd/maboo

# Format
make fmt

# Lint
make lint
```

## Dependencies

- `gorilla/websocket` — WebSocket protocol
- `vmihailenco/msgpack` — Binary serialization
- `gopkg.in/yaml.v3` — Config parsing

## License

MIT
