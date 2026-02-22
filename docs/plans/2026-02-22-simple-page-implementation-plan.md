# Simple Maboo PHP Page Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Serve a PHP info page via Maboo to validate basic functionality.

**Architecture:** Maboo runs as Go binary serving HTTP; PHP workers execute a simple handler that renders server info. Configuration file ties Maboo to the worker script and exposes port 8080.

**Tech Stack:** Go binary (Maboo), Maboo PHP SDK, PHP 8 CLI, Bash scripts.

---

### Task 1: Create PHP info page

**Files:**
- Create: `public/info.php`

**Step 1: Write page content**
```php
<?php
$info = [
    'php_version' => phpversion(),
    'worker_pid' => getmypid(),
    'server' => [
        'request_method' => $_SERVER['REQUEST_METHOD'] ?? null,
        'request_uri' => $_SERVER['REQUEST_URI'] ?? null,
        'user_agent' => $_SERVER['HTTP_USER_AGENT'] ?? null,
    ],
];
header('Content-Type: application/json');
echo json_encode($info, JSON_PRETTY_PRINT);
```

**Step 2: Commit**
```bash
git add public/info.php
git commit -m "feat: add info page"
```

### Task 2: Worker bootstrap for Maboo

**Files:**
- Create: `public/worker.php`

**Step 1: Implement worker loop**
```php
<?php
require __DIR__.'/../vendor/autoload.php';
use Maboo\Worker;
$worker = new Worker(function ($request, $response) {
    $path = $request->getUri()->getPath();
    if ($path === '/' || $path === '/info.php') {
        ob_start();
        require __DIR__.'/info.php';
        $response->getBody()->write(ob_get_clean());
        return $response->withHeader('Content-Type', 'application/json');
    }
    $response->getBody()->write('Not Found');
    return $response->withStatus(404);
});
$worker->run();
```

**Step 2: Commit**
```bash
git add public/worker.php
git commit -m "feat: add Maboo worker"
```

### Task 3: Maboo configuration

**Files:**
- Create: `maboo.yaml`

**Step 1: Write config**
```yaml
server:
  address: "0.0.0.0:8080"
php:
  binary: "php"
  worker: "public/worker.php"
pool:
  min_workers: 4
  max_workers: 16
```

**Step 2: Commit**
```bash
git add maboo.yaml
git commit -m "chore: add Maboo config"
```

### Task 4: Run helper script

**Files:**
- Create: `scripts/run-simple.sh`

**Step 1: Script content**
```bash
#!/usr/bin/env bash
set -euo pipefail
./maboo -config maboo.yaml > maboo.log 2>&1 &
PID=$!
echo "Maboo running on :8080 (PID $PID)"
trap 'kill $PID' INT TERM
wait $PID
```

**Step 2: Mark executable & commit**
```bash
chmod +x scripts/run-simple.sh
git add scripts/run-simple.sh
git commit -m "chore: add run script"
```

### Task 5: Smoke test

**Files:**
- n/a

**Step 1: Start Maboo**
```bash
./scripts/run-simple.sh
```

**Step 2: In another terminal, curl endpoint**
```bash
curl http://localhost:8080/
```
Expected JSON with `php_version` key.

**Step 3: Capture result in README snippet**
```bash
tee docs/examples/simple-info-response.json <<'EOF'
{ "php_version": "8.3.x", ... }
EOF
```

**Step 4: Commit**
```bash
git add docs/examples/simple-info-response.json
git commit -m "docs: add sample info response"
```

---

Plan complete and saved to `docs/plans/2026-02-22-simple-page-implementation-plan.md`. Two execution options:

1. **Subagent-Driven (this session)** — I dispatch a fresh subagent per task and review between steps.
2. **Parallel Session (separate)** — Start a new session and implement the plan using superpowers:executing-plans.

Which execution mode do you prefer?