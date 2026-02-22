# Maboo Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a PHP application server in Go that solves FrankenPHP's core problems through process-based architecture, custom binary protocol, and native WebSocket support.

**Architecture:** Go HTTP server → Worker Pool Manager → PHP worker processes communicating via custom binary protocol (maboo-wire v1) over stdin/stdout pipes. No CGo, no ZTS requirement.

**Tech Stack:** Go 1.22+, net/http, msgpack, gorilla/websocket, cobra CLI, YAML config, PHP 8.1+ SDK with PSR-7/PSR-15 bridges

---

## Task Dependency Graph

```
Task 1: Project Scaffold
    ├── Task 2: Config Package
    ├── Task 3: Wire Protocol
    └── Task 7: PHP SDK Core
         └── Task 8: Framework Bridges

Task 2 + Task 3 → Task 4: Worker Pool
Task 2 + Task 4 → Task 5: HTTP Server
Task 3 + 4 + 5 → Task 6: WebSocket Handler
Task 4 + 5 + 6 → Task 9: CLI Entry Point
Task 7 + 8 + 9 → Task 10: Integration Tests + Docker
```

## Parallelization Strategy

- **Wave 1**: Task 1 (scaffold) - sequential, must be first
- **Wave 2**: Tasks 2, 3, 7 in parallel (config, protocol, PHP SDK)
- **Wave 3**: Task 4 (pool) + Task 8 (bridges) in parallel
- **Wave 4**: Task 5 (HTTP server)
- **Wave 5**: Task 6 (WebSocket)
- **Wave 6**: Task 9 (CLI)
- **Wave 7**: Task 10 (integration tests)

## Tasks

### Task 1: Initialize Go Module and Project Scaffold
**Owner:** architect
**Files:** go.mod, cmd/maboo/main.go, Makefile, Dockerfile, maboo.yaml.example, .gitignore, php-sdk/composer.json

### Task 2: Config Package (internal/config)
**Owner:** backend
**Files:** internal/config/config.go, internal/config/defaults.go, internal/config/config_test.go

### Task 3: Wire Protocol (internal/protocol)
**Owner:** backend
**Files:** internal/protocol/wire.go, internal/protocol/request.go, internal/protocol/response.go, internal/protocol/stream.go, internal/protocol/*_test.go

### Task 4: Worker Pool Manager (internal/pool)
**Owner:** backend
**Files:** internal/pool/pool.go, internal/pool/worker.go, internal/pool/scaler.go, internal/pool/watchdog.go, internal/pool/*_test.go

### Task 5: HTTP Server (internal/server)
**Owner:** backend
**Files:** internal/server/server.go, internal/server/router.go, internal/server/middleware.go, internal/server/static.go, internal/server/tls.go

### Task 6: WebSocket Handler (internal/websocket)
**Owner:** backend
**Files:** internal/websocket/handler.go, internal/websocket/manager.go, internal/websocket/pubsub.go

### Task 7: PHP SDK Core
**Owner:** backend (PHP)
**Files:** php-sdk/src/Worker.php, php-sdk/src/Request.php, php-sdk/src/Response.php, php-sdk/src/Protocol/Wire.php, php-sdk/src/WebSocket/Server.php, php-sdk/src/WebSocket/Connection.php

### Task 8: PHP Framework Bridges
**Owner:** backend (PHP)
**Files:** php-sdk/src/Bridge/Laravel.php, php-sdk/src/Bridge/Symfony.php, php-sdk/src/Bridge/WordPress.php, php-sdk/src/Bridge/PSR7.php, + secondary bridges

### Task 9: CLI and Main Entry Point
**Owner:** backend
**Files:** cmd/maboo/main.go (full implementation), pkg/maboo/maboo.go

### Task 10: Integration Testing and Dockerfile
**Owner:** reviewer + devops
**Files:** tests/integration/*, examples/*, Dockerfile (finalized), docker-compose.yml
