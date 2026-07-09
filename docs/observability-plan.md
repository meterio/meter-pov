# Observability Improvement Plan

## Background

Meter nodes currently expose metrics via a standard Prometheus pull endpoint at
`:8670/metrics`. The Prometheus server scrapes this endpoint on a fixed interval
(typically 15–30s). This works, but it is poorly matched to how a blockchain node
actually behaves:

- The most important metrics (`pacemaker_round`, `blocks_commited_total`,
  `best_height`) change **every 2 seconds** — Prometheus misses 7–14 block
  commits between scrapes.
- Slow-changing metrics (`in_committee`, `current_epoch`) rarely change at all,
  so polling them constantly is waste.
- Nodes are often behind firewalls or NAT. Keeping an inbound scrape port open
  for every node is an operational burden.
- There is no mechanism to get an immediate alert when a node goes out of
  committee or falls behind — you only find out on the next scrape.

---

## Current Setup

### Metrics defined today

| Metric | Type | Package | Change cadence |
|---|---|---|---|
| `pacemaker_round` | Gauge | `consensus/prometheus.go` | Every ~2s (each round) |
| `pacemaker_running` | Gauge | `consensus/prometheus.go` | Rarely |
| `pacemaker_role` | Gauge | `consensus/prometheus.go` | Epoch boundary |
| `current_epoch` | Gauge | `consensus/prometheus.go` | Epoch boundary |
| `in_committee` | Gauge | `consensus/prometheus.go` | Epoch boundary |
| `last_kblock_height` | Gauge | `consensus/prometheus.go` | Epoch boundary |
| `blocks_commited_total` | Counter | `consensus/prometheus.go` | Every ~2s |
| `best_height` | Gauge | `chain/chain.go` | Every ~2s |
| `best_qc_height` | Gauge | `chain/chain.go` | Every ~2s |
| `peers_count` | Gauge | `comm/communicator.go` | On connect/disconnect |
| `pow_block_recved` | Gauge | `powpool/pow_pool.go` | Per PoW block |

### Endpoints

```
:8670/metrics          — Prometheus text format (pull)
:8670/probe            — Rich JSON (committee, pacemaker, chain, peers)
:8670/probe/peers      — Peer list
:8670/probe/version    — Node version
```

All metric definitions live in small, well-isolated files. The churn surface for
any migration is low.

---

## Recommended Approach

Two changes, independent of each other, ordered by impact.

---

### Change 1 — Event-driven hooks for critical state (quick win)

**Problem**: The five state transitions that matter most for operations happen
infrequently, but when they do, you need to know *immediately*, not at the next
scrape:

- Node leaves committee (`in_committee` 1 → 0)
- Node enters committee (`in_committee` 0 → 1)
- Epoch changes (`current_epoch` increments)
- Best block height stalls (node is stuck or partitioned)
- Peer count drops to zero

**Solution**: Add a small `notifier` package that fires a webhook POST whenever
one of these transitions happens. The existing call sites already call
`inCommitteeGauge.Set(...)` and `curEpochGauge.Set(...)` at exactly the right
moments — add a notifier call alongside each one.

#### Architecture

```
consensus/reactor.go
  inCommitteeGauge.Set(1)
  notifier.Notify(notifier.EventInCommittee, ...)   ← new

  inCommitteeGauge.Set(0)
  notifier.Notify(notifier.EventOutOfCommittee, ...) ← new

chain/chain.go
  bestHeightGauge.Set(...)
  notifier.CheckStall(height, timestamp)             ← new (detects no progress)
```

#### Notifier interface

```go
// notifier/notifier.go

type EventType string

const (
    EventInCommittee    EventType = "in_committee"
    EventOutOfCommittee EventType = "out_of_committee"
    EventEpochChange    EventType = "epoch_change"
    EventHeightStall    EventType = "height_stall"
    EventPeersLost      EventType = "peers_lost"
)

type Event struct {
    Type      EventType         `json:"type"`
    NodeID    string            `json:"node_id"`   // combo pubkey short form
    Timestamp time.Time         `json:"timestamp"`
    Data      map[string]any    `json:"data"`      // epoch, height, etc.
}

type Notifier struct {
    webhookURL string
    nodeID     string
    client     *http.Client
}

func (n *Notifier) Notify(ev Event) {
    // fire-and-forget POST; does not block the caller
    go n.send(ev)
}
```

#### Stall detection

```go
// Maintained in the notifier; checked whenever bestHeight is updated.
// If height has not advanced in > 30s, fire EventHeightStall.
type stallTracker struct {
    lastHeight    uint32
    lastAdvance   time.Time
    stallThreshold time.Duration  // default 30s
}
```

#### Configuration

```
--webhook-url  https://hooks.slack.com/...   (or any HTTP endpoint)
--node-id      <short pubkey>                 (auto-derived if not set)
```

If `--webhook-url` is not set, the notifier is a no-op. Existing nodes need zero
config changes to preserve current behaviour.

#### Files added / changed

```
notifier/
  notifier.go        — Notifier struct, Event types, send loop
  stall.go           — stallTracker for height-stall detection
  notifier_test.go   — unit tests with mock HTTP server

consensus/reactor.go  — call notifier at in_committee / epoch transitions
chain/chain.go        — call notifier.CheckStall on bestHeight update
comm/communicator.go  — call notifier on peers_count → 0
cmd/meter/flags.go    — add --webhook-url, --node-id flags
cmd/meter/must.go     — wire notifier into reactor, chain, communicator
```

#### Testing requirements

- Notifier fires `EventInCommittee` when `in_committee` gauge goes from 0 → 1
- Notifier fires `EventOutOfCommittee` when it goes from 1 → 0
- `stallTracker` fires after `stallThreshold` with no height update
- `stallTracker` does NOT fire if height advances before threshold
- Webhook POST fails silently (non-blocking); logged at debug level
- If `--webhook-url` is empty, no HTTP calls are made

---

### Change 2 — OpenTelemetry push (block-resolution metrics)

**Problem**: Block-level granularity (`pacemaker_round`, `blocks_commited_total`,
`best_height`) requires a 2s scrape interval to be useful. Running Prometheus at
2s on many nodes is expensive in storage and query load.  Prometheus also requires
inbound network access to each node, which complicates firewall rules.

**Solution**: Replace the Prometheus pull endpoint with an OpenTelemetry push
export. Nodes push metrics to a central OTel Collector every 2s. The Collector
re-exposes them as a Prometheus scrape endpoint or remote-writes to
Grafana/VictoriaMetrics. Grafana dashboards require no changes.

```
Before:
  Prometheus ──scrape every 15s──→ :8670/metrics (each node)

After:
  each node ──OTLP push every 2s──→ OTel Collector ──→ Prometheus / Grafana
```

#### Why OpenTelemetry, not Prometheus Pushgateway

Pushgateway has a well-known problem: if a node dies, the last pushed value stays
in the gateway forever, making it look like the node is still running. The OTel
Collector has native liveness tracking and drops stale series automatically.

#### Library

`go.opentelemetry.io/otel` + `go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc`

No CGo, pure Go, CNCF-graduated project. The same gauges and counters are
re-expressed using the OTel metrics API; no behavioural change.

#### Migration path (non-breaking)

The Prometheus endpoint at `:8670/metrics` is kept alongside the OTel push.
Operators can migrate to OTel at their own pace. The `/metrics` endpoint is
removed in a later cleanup PR once all operators have switched.

```
Phase 1 (this PR): OTel push added, Prometheus pull kept
Phase 2 (later):   Prometheus pull removed
```

#### Metric mapping

| Existing (prometheus) | OTel equivalent | Notes |
|---|---|---|
| `prometheus.NewGauge(...)` | `meter.Int64Gauge(...)` | Direct mapping |
| `prometheus.NewCounter(...)` | `meter.Int64Counter(...)` | Direct mapping |
| `gauge.Set(float64(v))` | `gauge.Record(ctx, int64(v))` | Type changes float64→int64 |
| `counter.Inc()` | `counter.Add(ctx, 1)` | Context required |

#### Push interval

The push interval is set to 2s to match `RoundInterval`. This means every round
is captured with zero missed events. Operators can override via flag.

#### OTel Collector config (reference)

```yaml
# collector.yaml
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317

exporters:
  prometheus:
    endpoint: "0.0.0.0:9090"
  # or:
  prometheusremotewrite:
    endpoint: "https://your-grafana-cloud/api/prom/push"

service:
  pipelines:
    metrics:
      receivers: [otlp]
      exporters: [prometheus]
```

#### Files added / changed

```
otelmetrics/
  metrics.go         — OTel meter provider setup, PeriodicReader at 2s
  gauges.go          — all gauge/counter definitions (mirrors prometheus.go)
  otelmetrics_test.go

consensus/prometheus.go  — keep existing defs; add parallel OTel updates
consensus/reactor.go     — call OTel gauges alongside Prometheus gauges
chain/chain.go           — same
comm/communicator.go     — same
cmd/meter/flags.go       — add --otel-endpoint, --otel-push-interval flags
cmd/meter/must.go        — initialise OTel provider if --otel-endpoint is set
```

#### Testing requirements

- With `--otel-endpoint` unset: no OTel calls are made; behaviour identical to today
- With `--otel-endpoint` set: metrics arrive at the collector within 1 push interval
- `pacemaker_round` at the collector matches the value on `/metrics` ± 1 round
- OTel export failure (collector unreachable) is logged but does not crash the node
- Benchmark: OTel `Record()` call overhead < 1µs (it must not slow down the hot
  block-commit path)

---

## Dependency graph

```
Change 1 (notifier webhooks)   — independent, no deps
Change 2 (OTel push)           — independent, no deps
```

Both changes are additive and non-breaking. Either can ship first or be skipped.

---

## Risks and Mitigations

| Risk | Likelihood | Mitigation |
|---|---|---|
| Webhook URL leaks auth token in logs | Low | Redact URL in log output; only log domain |
| Notifier blocks hot path on slow webhook | Low | `go n.send(ev)` is fire-and-forget; timeout is 3s |
| OTel collector is a new infra component to operate | Medium | Provide a reference docker-compose; Grafana Cloud Alloy is a drop-in with no self-hosting |
| OTel push interval (2s) floods collector with many nodes | Low | 2s × ~50 metrics × N nodes is still tiny; OTel Collector handles >100k samples/s easily |
| Duplicate metrics in Prometheus (pull + OTel push both active) | Medium | Name OTel metrics with `_otel` suffix during Phase 1 transition; remove suffix in Phase 2 |

---

## Appendix: metrics worth adding

The existing set is minimal. While doing this work, the following would be cheap
to add and immediately useful:

| Metric | Why |
|---|---|
| `consensus_round_duration_ms` (histogram) | Detect slow rounds caused by network issues |
| `consensus_vote_count` (gauge) | How many votes received per round; detects partial committee participation |
| `p2p_message_bytes_in/out` (counter) | Detect bandwidth spikes or gossip storms |
| `txpool_size` (gauge) | Detect mempool buildup |
| `block_processing_duration_ms` (histogram) | Detect slow state execution |
| `qc_verify_duration_ms` (histogram) | Relevant for the BLS12-381 migration (PR #35) |
