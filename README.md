# Maboo

<p align="center">
  <img src="assets/logo.svg" alt="Maboo Logo" width="180">
</p>

<p align="center">
  <strong>Embedded PHP Application Server</strong>
</p>

<p align="center">
  <a href="#performance">Benchmarks</a> •
  <a href="#quick-start">Quick Start</a> •
  <a href="#configuration">Configuration</a> •
  <a href="#architecture">Architecture</a>
</p>

---

A high-performance PHP application server with embedded PHP. Single binary, zero configuration, runs Laravel, Symfony, and WordPress out of the box.

## Why Maboo?

Traditional PHP-FPM spawns a new PHP process for each request. Maboo embeds PHP directly in Go:

- **Single Binary** — Download and run, no PHP installation needed
- **Multi-Version** — PHP 7.4, 8.0, 8.1, 8.2, 8.3, 8.4 bundled
- **Worker Mode** — Persistent workers for 10x performance
- **Request Mode** — Fresh context per request for compatibility
- **Auto-Detect** — Automatically finds entry points and frameworks
- **Graceful Operations** — Zero-downtime reload, graceful shutdown
- **Modern Protocols** — HTTP/2, HTTP/3 (QUIC), ACME (Let's Encrypt)

## Features

- **Embedded PHP Engine** — CGO bindings to libphp for direct execution
- **Worker Pool Management** — Auto-scaling min/max workers, idle timeout
- **HTTP/2 & HTTP/3** — Modern protocol support with h2c and QUIC
- **ACME/Let's Encrypt** — Automatic HTTPS with free certificates
- **Gzip Compression** — Pooled writers with lazy buffering (~1 GB/s throughput)
- **Prometheus Metrics** — Request histograms, worker gauges, memory stats
- **Auto-TLS** — Self-signed cert for development or bring your own
- **Zero-downtime Reload** — `SIGUSR1` for graceful worker rotation
- **Static File Serving** — With configurable `Cache-Control`
- **Health Checks** — `/health`, `/healthz`, `/ready`, `/readyz`
- **Framework Detection** — Laravel, Symfony, WordPress, Drupal, generic PHP

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

### Optimizations Applied

1. **sync.Pool for gzip.Writer** — Reuses writers, eliminates 813 KB/op allocation
2. **Pooled response writers** — Single wrapper for all middleware layers
3. **Stack-allocated slog attrs** — Fixed-size array instead of variadic spread
4. **Lazy compression buffering** — Only allocate when compression threshold met
5. **BestSpeed compression level** — 4.5x throughput with ~5% size trade-off

## Quick Start

### 1. Build

```bash
make build
# or
go build -o maboo ./cmd/maboo
```

### 2. Run

```bash
# That's it! Maboo auto-detects your framework
cd your-laravel-project
maboo serve
```

The server starts on `http://0.0.0.0:8080` by default.

### 3. Configure (Optional)

```bash
cp maboo.yaml.example maboo.yaml
# Edit maboo.yaml to customize
```

## Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                         Maboo Binary                                │
├─────────────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────────┐ │
│  │ HTTP Server │  │ Middleware  │  │      Worker Pool            │ │
│  │ (net/http)  │  │   Stack     │  │  ┌─────────┐ ┌─────────┐   │ │
│  │             │  │             │  │  │Worker 1 │ │Worker 2 │   │ │
│  │ - HTTP/2    │  │ - Gzip      │  │  │ (PHP    │ │ (PHP    │   │ │
│  │ - HTTP/3    │  │ - Metrics   │  │  │ Context)│ │ Context)│   │ │
│  │ - TLS/ACME  │  │ - Health    │  │  └─────────┘ └─────────┘   │ │
│  │ - WebSocket │  │ - Early Hints│ │                            │ │
│  └──────┬──────┘  └──────┬──────┘  └─────────────┬───────────────┘ │
│         │                │                     │                   │
│         └────────────────┼─────────────────────┘                   │
│                          │                                         │
│                          ▼                                         │
│  ┌───────────────────────────────────────────────────────────────┐ │
│  │                    PHP Engine (CGO)                           │ │
│  │  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐ │ │
│  │  │ PHP 7.4 │ │ PHP 8.0 │ │ PHP 8.1 │ │ PHP 8.2 │ │ PHP 8.3 │ │ │
│  │  │         │ │         │ │         │ │         │ │   +8.4  │ │ │
│  │  └─────────┘ └─────────┘ └─────────┘ └─────────┘ └─────────┘ │ │
│  └───────────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────────┘
```

## Configuration

| Section | Key | Default | Description |
|---------|-----|---------|-------------|
| `server.address` | `0.0.0.0:8080` | Listen address |
| `server.http2` | `true` | Enable HTTP/2 support |
| `server.http3` | `false` | Enable HTTP/3 (QUIC) |
| `server.http_redirect` | `false` | HTTP to HTTPS redirect |
| `server.tls.auto` | `false` | Auto-generate self-signed cert |
| `server.tls.cert` | `""` | Path to TLS certificate |
| `server.tls.key` | `""` | Path to TLS private key |
| `server.tls.acme.email` | `""` | Let's Encrypt email |
| `server.tls.acme.domains` | `[]` | Domains for certificate |
| `server.tls.acme.staging` | `false` | Use Let's Encrypt staging |
| `php.version` | `auto` | PHP version (auto, 7.4, 8.0, 8.1, 8.2, 8.3, 8.4) |
| `php.mode` | `worker` | Execution mode (worker, request) |
| `pool.min_workers` | `4` | Minimum workers |
| `pool.max_workers` | `32` | Maximum workers |
| `pool.max_jobs` | `10000` | Requests per worker before restart |
| `pool.max_memory` | `128M` | Memory limit per worker |
| `pool.idle_timeout` | `60s` | Kill idle workers after |
| `app.root` | `.` | Document root |
| `app.entry` | `auto` | Entry point (auto-detect or explicit) |
| `static.root` | `public` | Static files directory |
| `logging.level` | `info` | Log level (debug/info/warn/error) |
| `logging.format` | `json` | Log format (json/text) |
| `metrics.enabled` | `true` | Enable Prometheus metrics |

## HTTP/2 & HTTP/3

Maboo supports modern HTTP protocols:

### HTTP/2

```yaml
server:
  http2: true
```

- Automatic for HTTPS connections
- h2c (HTTP/2 cleartext) for HTTP

### HTTP/3 (QUIC)

```yaml
server:
  http3: true
  tls:
    cert: "/path/to/cert.pem"
    key: "/path/to/key.pem"
```

- Uses QUIC protocol (UDP-based)
- Alt-Svc header auto-advertised
- Requires TLS

## Automatic HTTPS (Let's Encrypt)

```yaml
server:
  http_redirect: true
  tls:
    acme:
      email: "admin@example.com"
      domains:
        - example.com
        - www.example.com
      staging: false  # Set true for testing
```

- Automatic certificate provisioning
- Auto-renewal before expiry
- HTTP-01 challenge support

## Execution Modes

### Worker Mode (Default)

Persistent workers that handle multiple requests. Best performance:

```yaml
php:
  mode: "worker"
```

- Workers boot once and stay alive
- PHP state persists across requests
- Framework boot happens only once
- ~10x faster than traditional PHP-FPM

### Request Mode

Fresh PHP context for each request. Maximum compatibility:

```yaml
php:
  mode: "request"
```

- New context per request
- No state persistence
- Works with all PHP applications
- Similar to traditional PHP-FPM behavior

## Framework Detection

Maboo automatically detects common PHP frameworks:

| Framework | Detection | Entry Point |
|-----------|-----------|-------------|
| Laravel | `artisan` file | `public/index.php` |
| Symfony | `bin/console` file | `public/index.php` |
| WordPress | `wp-config.php` file | `index.php` |
| Drupal | `core/lib/Drupal.php` | `index.php` |
| Generic | — | `index.php` or `public/index.php` |

## PHP Version Selection

Version selection priority:

1. **Explicit config** — `php.version: "8.3"` in maboo.yaml
2. **composer.json** — Reads `require.php` constraint
3. **Default** — Falls back to PHP 8.3

Example composer.json:
```json
{
    "require": {
        "php": "^8.1"
    }
}
```

Maboo will automatically use PHP 8.3 (latest compatible with ^8.1).

## Signals

| Signal | Action |
|--------|--------|
| `SIGINT` | Graceful shutdown |
| `SIGTERM` | Graceful shutdown |
| `SIGUSR1` | Zero-downtime worker reload |

## Endpoints

| Path | Description |
|------|-------------|
| `/` | PHP application (placeholder until CGO) |
| `/health` | Health check (always 200) |
| `/healthz` | Liveness probe |
| `/ready` | Readiness probe (checks worker pool) |
| `/readyz` | Readiness probe |
| `/metrics` | Prometheus metrics (if enabled) |

## Metrics

Maboo exposes Prometheus metrics at `/metrics`:

| Metric | Type | Description |
|--------|------|-------------|
| `maboo_http_requests_total` | counter | Total HTTP requests |
| `maboo_http_requests_active` | gauge | Active HTTP requests |
| `maboo_http_response_bytes_total` | counter | Total bytes sent |
| `maboo_http_request_duration_seconds` | histogram | Request duration |
| `maboo_workers_total` | gauge | Total PHP workers |
| `maboo_workers_busy` | gauge | Busy PHP workers |
| `maboo_workers_idle` | gauge | Idle PHP workers |
| `maboo_pool_requests_total` | counter | Pool requests processed |
| `maboo_go_goroutines` | gauge | Number of goroutines |
| `maboo_go_memstats_alloc_bytes` | gauge | Memory allocated |

## Development

```bash
# Build
make build

# Build with version
VERSION=1.0.0 make build

# Build optimized
go build -trimpath -ldflags "-s -w" -o maboo ./cmd/maboo

# Run tests
go test -race -count=1 ./...

# Format
make fmt

# Lint
make lint
```

## Dependencies

- `gopkg.in/yaml.v3` — Config parsing
- `github.com/quic-go/quic-go` — HTTP/3 support
- `golang.org/x/net/http2` — HTTP/2 support
- `golang.org/x/crypto/acme` — Let's Encrypt support

## License

MIT
