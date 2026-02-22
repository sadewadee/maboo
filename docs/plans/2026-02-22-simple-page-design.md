# Simple Maboo PHP Page Design

**Date:** 2026-02-22
**Status:** Draft

## Objective
Serve a simple PHP page via Maboo that displays server information (PHP version, environment) to validate a minimal Maboo deployment.

## Requirements
- Content: display PHP version, Maboo worker PID, HTTP headers.
- Deployment: single Maboo binary serving the page.
- Verification: curl/browser request returns expected info.

## Architecture
- Use Maboo's process-based worker to run a PHP script located in `public/info.php`.
- Maboo configuration `maboo.yaml` points to the PHP binary and worker bootstrap `public/worker.php` that routes requests to `info.php`.
- HTTP request flow: `maboo (Go HTTP server)` → `worker pool` → `info.php` script.

## Components
1. **PHP script (`public/info.php`)**
   - Outputs JSON or HTML containing `phpversion()`, `getmypid()`, `$_SERVER` subset.
2. **Worker entry (`public/worker.php`)**
   - Uses Maboo PHP SDK to handle requests and include `info.php` when `/` path requested.
3. **Maboo config (`maboo.yaml`)**
   - Minimal config referencing PHP binary, worker script, and server address.
4. **Run script (`scripts/run-simple.sh`)**
   - Starts Maboo with the config and tails logs for verification.

## Data Flow
1. HTTP GET `/` hits Maboo on port 8080.
2. Maboo enqueues request to worker pool via binary protocol.
3. Worker executes `worker.php`, instantiates Maboo SDK worker loop.
4. Request routed to handler that includes `info.php`.
5. `info.php` renders server info as HTML.
6. Response returned to client.

## Error Handling
- If `info.php` throws, respond with 500 and log error using Maboo SDK logger.
- Provide fallback text if `$_SERVER` keys missing.

## Testing
- Smoke test: `curl http://localhost:8080/` returns HTML containing `PHP Version:`.
- Validate headers such as `X-Maboo-Worker` if added.

## Next Steps
1. Write implementation plan via writing-plans.
2. Implement files, run Maboo, confirm output.
