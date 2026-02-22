# Maboo Benchmark Design

**Date:** 2026-02-22
**Status:** Draft

## Objective
Benchmark Maboo's request handling performance against FrankenPHP and PHP-FPM using synthetic workloads focused on concurrency scaling. Target outcome: Maboo matches or exceeds FrankenPHP throughput/latency while retaining stability under high parallel load.

## Scope
- Synthetic microbenchmark with a lightweight PHP script shared across all servers.
- Compare three server setups: Maboo, FrankenPHP, PHP-FPM (FastCGI behind minimal HTTP server).
- Metrics: throughput (req/s), latency distribution, error rate, CPU/RAM usage.

## Environment
- Hardware: identical machines or VMs; one dedicated load generator, one SUT host.
- Software:
  - PHP version parity across all servers (same extensions enabled).
  - Containers for isolation (Docker Compose recommended).
  - Load generator tool: `wrk2`, `vegeta`, or `k6`.
- Networking: load generator connects over LAN; avoid localhost bottlenecks.

## Test Application
- Single PHP entry script returning “Hello World” plus trivial logic (e.g., date formatting) to prevent compiler optimizations.
- Script stored identically for all environments.
- Worker bootstrap for Maboo/FrankenPHP, and simple FPM pool + Nginx/Apache for PHP-FPM.

## Scenarios
| Scenario | Concurrency Levels | Duration | Notes |
|----------|--------------------|----------|-------|
| Baseline throughput | 50, 200 reqs in-flight | 60s warm-up + 120s sample | Measures steady-state performance |
| Stress test | 500, 1000 reqs in-flight | 120s warm-up + 180s sample | Observes saturation, error rate |
| Resource profile | 200 reqs steady | 300s | Capture CPU/RAM over time |

## Metrics Collection
- Load generator outputs throughput, latency percentiles, error counts.
- System metrics from `docker stats`, `pidstat`, or Prometheus exporters.
- Log configuration snapshots: worker counts, memory limits, PHP config.

## Procedure
1. Build container images for each server with identical PHP binaries/extensions.
2. Deploy baseline configuration files (Maboo `maboo.yaml`, FrankenPHP config, PHP-FPM pool/Nginx).
3. Validate functionality with curl smoke tests.
4. Run each scenario per server:
   - Reset environment (restart containers, clear caches) between runs.
   - Execute workload 3x per scenario; record averages and standard deviations.
5. Aggregate results into tables/graphs comparing throughput, latency, and resource use.
6. Document observations: where Maboo meets/exceeds FrankenPHP, note regressions or tuning needs.

## Success Criteria
- Maboo throughput and latency are ≥ FrankenPHP at all tested concurrency levels.
- Error rates remain within tolerance (<0.1%) under stress.
- Resource usage comparable or better than FrankenPHP, with stable worker recycling.

## Risks & Mitigations
- **Unequal PHP configs**: automate container build to share php.ini/extensions.
- **Network bottlenecks**: dedicate load generator host, verify with iperf.
- **Measurement noise**: multiple runs and warm-ups reduce variance.
- **Tool limitations**: cross-validate `wrk2` with secondary tool (e.g., `vegeta`).

## Next Steps
- Finalize container definitions and scripts for each server type.
- Prepare automation scripts to orchestrate benchmark runs and data capture.
- After design approval, create implementation plan via writing-plans skill.
