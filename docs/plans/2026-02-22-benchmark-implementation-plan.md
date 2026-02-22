# Maboo Benchmark Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build and run synthetic concurrency benchmarks comparing Maboo vs FrankenPHP vs PHP-FPM.

**Architecture:** Use Docker containers for each PHP server variant; load generator machine runs wrk2/vegeta to drive HTTP traffic. Shared PHP script ensures identical workload. Metrics gathered from load tool and system monitors.

**Tech Stack:** Go (Maboo binary), PHP CLI/FPM, FrankenPHP, Docker/Compose, wrk2 or vegeta, Bash scripting, Prometheus/node-exporter (optional).

---

### Task 1: Containerize benchmark targets

**Files:**
- Create: `bench/docker/maboo/Dockerfile`
- Create: `bench/docker/frankenphp/Dockerfile`
- Create: `bench/docker/phpfpm/Dockerfile`
- Create: `bench/php/app/worker.php`
- Create: `bench/php/app/maboo-worker.php`
- Create: `bench/php/app/frankenphp-worker.php`
- Create: `bench/php/app/index.php`

**Step 1: Write Maboo Dockerfile**
```Dockerfile
FROM golang:1.22 AS build
WORKDIR /src
COPY go.* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /maboo-bench ./cmd/maboo

FROM php:8.3-cli
COPY --from=build /maboo-bench /usr/local/bin/maboo
COPY bench/php/app /app
WORKDIR /app
CMD ["maboo", "-config", "maboo.yaml"]
```

**Step 2: Write FrankenPHP Dockerfile** (use official image, copy app)
```Dockerfile
FROM dunglas/frankenphp:1.1
COPY bench/php/app /app
WORKDIR /app
CMD ["frankenphp", "php-server", "--config=frankenphp.yaml"]
```

**Step 3: Write PHP-FPM Dockerfile**
```Dockerfile
FROM php:8.3-fpm
RUN apt-get update && apt-get install -y nginx && rm -rf /var/lib/apt/lists/*
COPY bench/php/app /app
COPY bench/nginx/default.conf /etc/nginx/conf.d/default.conf
CMD ["/bin/sh", "-c", "service nginx start && php-fpm"]
```

**Step 4: Add shared PHP script**
```php
<?php
function busywork() {
    $sum = 0;
    for ($i = 0; $i < 1000; $i++) {
        $sum += sin($i) * cos($i);
    }
    return $sum;
}

echo json_encode([
    'time' => microtime(true),
    'busy' => busywork(),
]);
```

**Step 5: Commit**
```bash
git add bench/docker bench/php
git commit -m "chore: scaffold benchmark containers"
```

### Task 2: Configuration files & compose setup

**Files:**
- Create: `bench/maboo.yaml`
- Create: `bench/frankenphp.yaml`
- Create: `bench/php-fpm.conf`
- Create: `bench/nginx/default.conf`
- Create: `bench/docker-compose.yml`

**Step 1: Maboo config**
```yaml
server:
  address: "0.0.0.0:8080"
php:
  binary: "php"
  worker: "maboo-worker.php"
pool:
  min_workers: 4
  max_workers: 32
```

**Step 2: FrankenPHP config**
```yaml
worker:
  script: frankenphp-worker.php
  workers: 16
```

**Step 3: PHP-FPM pool**
```
[www]
listen = 9000
pm = dynamic
pm.max_children = 32
```

**Step 4: Nginx config**
```
server {
    listen 8080;
    root /app;
    location / {
        fastcgi_pass 127.0.0.1:9000;
        include fastcgi_params;
        fastcgi_param SCRIPT_FILENAME /app/index.php;
    }
}
```

**Step 5: docker-compose definition**
```yaml
services:
  maboo:
    build: ./docker/maboo
    ports: ["18080:8080"]
  frankenphp:
    build: ./docker/frankenphp
    ports: ["28080:8080"]
  phpfpm:
    build: ./docker/phpfpm
    ports: ["38080:8080"]
```

**Step 6: Commit**
```bash
git add bench/maboo.yaml bench/frankenphp.yaml bench/php-fpm.conf bench/nginx bench/docker-compose.yml
git commit -m "chore: add benchmark configs"
```

### Task 3: Load generator tooling

**Files:**
- Create: `bench/load/README.md`
- Create: `bench/load/run-wrk2.sh`
- Create: `bench/load/run-vegeta.sh`

**Step 1: Document prerequisites**
```markdown
# Load Generator
Install wrk2 and vegeta on separate host. Example commands below assume target host `sut` accessible via SSH.
```

**Step 2: wrk2 script**
```bash
#!/usr/bin/env bash
TARGET=$1
CONCURRENCY=$2
RATE=$3
DURATION=$4
wrk2 -t4 -c"$CONCURRENCY" -d"$DURATION" -R"$RATE" "http://$TARGET/"
```

**Step 3: vegeta script**
```bash
#!/usr/bin/env bash
TARGET=$1
RATE=$2
DURATION=$3
echo "GET http://$TARGET/" | vegeta attack -rate="$RATE" -duration="$DURATION" | tee results.bin | vegeta report
```

**Step 4: Mark executable & commit**
```bash
chmod +x bench/load/*.sh
git add bench/load
git commit -m "chore: add load generator helpers"
```

### Task 4: Benchmark automation script

**Files:**
- Create: `bench/scripts/run-benchmarks.sh`
- Create: `bench/scripts/collect-metrics.sh`
- Modify: `bench/load/README.md`

**Step 1: run-benchmarks**
```bash
#!/usr/bin/env bash
set -euo pipefail
SERVICES=(maboo frankenphp phpfpm)
CONCURRENCY=(50 200 500 1000)
DURATION=120
for svc in "${SERVICES[@]}"; do
  docker compose up -d "$svc"
  sleep 30
  for c in "${CONCURRENCY[@]}"; do
    ./bench/load/run-wrk2.sh localhost:$(port "$svc") "$c" "$((c*20))" "${DURATION}s" | tee "results/${svc}-${c}.log"
  done
  docker compose down "$svc"
done
```
(Implement helper `port()` mapping service to exposed port.)

**Step 2: collect-metrics**
```bash
#!/usr/bin/env bash
SERVICE=$1
DURATION=$2
pidstat 1 "$DURATION" -C "$SERVICE" > "results/${SERVICE}-pidstat.log"
```

**Step 3: Update README with automation instructions.**

**Step 4: Commit**
```bash
git add bench/scripts bench/load/README.md
git commit -m "feat: add benchmark automation scripts"
```

### Task 5: Run benchmarks & capture data

**Files:**
- Results stored in `bench/results/`
- Create: `bench/results/.gitkeep`

**Step 1: Prepare**
```bash
mkdir -p bench/results
./bench/scripts/run-benchmarks.sh
```

**Step 2: Collect resource metrics**
```bash
./bench/scripts/collect-metrics.sh maboo 300
./bench/scripts/collect-metrics.sh frankenphp 300
./bench/scripts/collect-metrics.sh phpfpm 300
```

**Step 3: Archive results**
```bash
tar czf bench/results-$(date +%Y%m%d).tar.gz bench/results
```

**Step 4: Commit raw data (optional)**
```bash
git add bench/results/.gitkeep
git commit -m "chore: prep results dir"
```

### Task 6: Analyze & report

**Files:**
- Create: `docs/reports/2026-02-22-benchmark-summary.md`
- Create: `bench/results/process_results.py`

**Step 1: Write parser script**
```python
import json
# load wrk2 logs, compute averages, output CSV/Markdown tables
```

**Step 2: Generate plots/tables**
```bash
python bench/results/process_results.py bench/results > docs/reports/2026-02-22-benchmark-summary.md
```

**Step 3: Summarize key findings**
Include throughput, latency, resource comparison; note if Maboo >= FrankenPHP.

**Step 4: Commit**
```bash
git add bench/results/process_results.py docs/reports/2026-02-22-benchmark-summary.md
git commit -m "feat: add benchmark analysis"
```

---

Plan complete and saved to `docs/plans/2026-02-22-benchmark-implementation-plan.md`. Two execution options:

1. **Subagent-Driven (this session)** — I dispatch fresh subagent per task with reviews between steps.
2. **Parallel Session (separate)** — Open new session focused on execution using superpowers:executing-plans.

Which approach do you prefer?```}