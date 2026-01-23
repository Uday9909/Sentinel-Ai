# 🚀 Sentinel Enhancement Roadmap

## 1️⃣ Ingestion Layer (Go) — Scale, Safety, Signal
**Goal**: Robust data entry with rich context.

-   [ ] **a) Preserve full log context**
    -   Update `LogEntry` struct to include `TraceID`, `Host`, `Timestamp`, `Labels`.
    -   Enables: Service maps, Trace correlation, Cross-host failure detection.

-   [ ] **b) Backpressure & fail-safe Kafka writes**
    -   Use async writer.
    -   Add retry + Dead Letter Queue (DLQ) topic (`raw-logs-dlq`).

-   [ ] **c) Rate limiting & abuse protection**
    -   Per-service rate limits.
    -   Return 429 responses.

## 2️⃣ Kafka & Streaming — Resilience & Control
**Goal**: Ordered, replayable, manageable streams.

-   [ ] **a) Partition by service**
    -   Key: `[]byte(entry.Service)`
    -   Benefits: Ordered logs per service, Scalable consumers.

-   [ ] **b) Replay & forensic mode**
    -   Ability to replay last N hours for incident reconstruction.

## 3️⃣ Intelligence Layer — Smarter, Cheaper, Faster
**Goal**: Efficient high-fidelity detection.

-   [ ] **a) Persistent anomaly models**
    -   Train Isolation Forest every 30–60s and persist state.
    -   Reuse model to save CPU.

-   [ ] **b) Multi-dimensional anomaly detection**
    -   Add features: Error % vs Info %, Template entropy, Seasonality.

-   [ ] **c) Template-level anomaly detection**
    -   Detect "New unseen log pattern" or "Rare template suddenly dominating".

## 4️⃣ LLM Layer — From Analysis → Action
**Goal**: Actionable, learning AI SRE.

-   [ ] **a) Incident memory**
    -   Store past anomalies and fixes.
    -   Prompt: "Have we seen this before? What resolved it?"

-   [ ] **b) Controlled prompting**
    -   Require confidence scores and preconditions.

-   [ ] **c) Auto-silencing**
    -   Silence repeated "Normal" alerts.

## 5️⃣ Storage & Data — Search, Retention, Cost
**Goal**: Sustainable storage.

-   [ ] **a) Time-based indices + ILM**
    -   `logs-YYYY.MM.DD`
    -   Hot/Warm/Delete phases.

-   [ ] **b) Incident index**
    -   Separate indices for Raw Logs vs Incidents.

## 6️⃣ Frontend (Mission Control) — SRE UX
**Goal**: Visual clarity during chaos.

-   [ ] **a) Incident timeline view**
    -   Spike graph, logs before/after.

-   [ ] **b) Blast radius visualization**
    -   Affected services graph.

-   [ ] **c) Feedback loop**
    -   Buttons for "Useful", "False positive", "Fix worked".

## 7️⃣ Alerting & Automation — Closing the Loop
**Goal**: Automated response.

-   [ ] **a) Alert routing**
    -   Slack/PagerDuty integration.

-   [ ] **b) Safe auto-remediation**
    -   Restart pods, clear cache.

## 8️⃣ Security & Compliance — Enterprise Ready
**Goal**: Secure and auditable.

-   [ ] **a) PII detection & redaction**
    -   Mask emails, tokens, credit cards.

-   [ ] **b) RBAC + audit logs**
    -   Track who viewed/resolved incidents.

## 9️⃣ Reliability of Sentinel itself
**Goal**: Watch the watchers.

-   [ ] **a) SRE for SRE**
    -   Circuit breakers for LLM.
    -   Self-anomaly detection.
    -   Chaos testing.

---

## 🧠 Prioritization Strategy

### Phase 1 (High ROI, low risk)
-   Kafka keying
-   Metadata preservation
-   ILM
-   Model reuse
-   Consumer groups
