# Good First Issues

Welcome! These issues are specifically designed for new contributors to Sentinel. Each one is well-scoped, has clear acceptance criteria, and comes with a mentor available in the issue comments.

---

### 1. Add dark/light mode toggle to React dashboard
**Difficulty**: Beginner | **Area**: Frontend

**Description**: The dashboard currently only supports a light theme. Add a toggle switch that switches between light and dark mode, persisting the user's preference in `localStorage`.

**Files to touch**: `dashboard/src/App.jsx`, `dashboard/src/index.css` (or a new theme CSS file)

**Acceptance Criteria**:
- [ ] A toggle button is visible in the dashboard header/navbar
- [ ] Clicking the toggle switches between light and dark CSS themes
- [ ] The preference is saved to `localStorage` and restored on page reload
- [ ] All existing components render correctly in both modes

---

### 2. Add Docker health checks for Kafka, Elasticsearch, and Ollama
**Difficulty**: Beginner | **Area**: DevOps

**Description**: The `docker-compose.yml` currently lacks health checks. Add `healthcheck` blocks to the Kafka, Elasticsearch, and Prometheus services so Docker can report when they're truly ready.

**Files to touch**: `docker-compose.yml`

**Acceptance Criteria**:
- [ ] Each service has a `healthcheck` block with a reasonable test command and interval
- [ ] The `depends_on` clauses use `condition: service_healthy` where appropriate
- [ ] Running `docker ps` shows healthy status for all services once ready

---

### 3. Write unit tests for Go ingestion API handlers
**Difficulty**: Beginner | **Area**: Backend

**Description**: Improve test coverage for the ingestion service. Write table-driven tests covering the `/ingest` endpoint with valid payloads, missing fields, invalid JSON, and error paths.

**Files to touch**: `ingestion-service/main_test.go`

**Acceptance Criteria**:
- [ ] Tests cover at least: valid payload, missing fields, invalid JSON, missing timestamp auto-fill
- [ ] Tests use `httptest.NewServer` or `gin` test utilities
- [ ] All tests pass with `go test -v -race ./...`

---

### 4. Add input validation and error handling to the log ingestion endpoint
**Difficulty**: Beginner–Intermediate | **Area**: Backend

**Description**: The `/ingest` endpoint in `ingestion-service/main.go` accepts any JSON without field validation. Add validation that rejects logs with empty `service` or `level` fields with clear error messages.

**Files to touch**: `ingestion-service/main.go`

**Acceptance Criteria**:
- [ ] Reject requests where `service` is empty or missing with a 400 status
- [ ] Reject requests where `level` is empty or missing with a 400 status
- [ ] Error responses include a descriptive message (e.g., `"field 'service' is required"`)
- [ ] Existing valid requests continue to work

---

### 5. Create a Makefile target for one-command local development
**Difficulty**: Beginner | **Area**: DevOps

**Description**: The project has a Makefile but `make dev` only prints instructions. Create a `make run-all` target that orchestrates starting Docker infrastructure and each service (or at minimum, prints copy-pasteable multi-terminal commands).

**Files to touch**: `Makefile`

**Acceptance Criteria**:
- [ ] `make run-all` starts infrastructure and prints clear instructions for each service
- [ ] The target works on both macOS and Linux
- [ ] Error handling if Docker is not running

---

### 6. Add Prometheus metrics endpoint to the Go ingestion service
**Difficulty**: Beginner–Intermediate | **Area**: Backend, Observability

**Description**: The ingestion service already registers Prometheus metrics (`logs_ingested_total`, `ingestion_duration_seconds`) but could expose more detailed metrics. Add a counter for per-status-code response counts and a gauge for Kafka writer queue depth.

**Files to touch**: `ingestion-service/main.go`

**Acceptance Criteria**:
- [ ] New metrics are registered and exposed at the `/metrics` endpoint
- [ ] Metrics include proper label dimensions and help text
- [ ] Existing metrics are not broken

---

### 7. Improve README with a GIF demo instead of static screenshot
**Difficulty**: Beginner | **Area**: Documentation

**Description**: The README has a placeholder screenshot. Create a screen recording GIF showing the full pipeline — sending a log via curl, seeing it appear in the dashboard, and an anomaly being detected and analyzed.

**Files to touch**: `README.md`, a new `docs/demo.gif` or `docs/demo.mp4`

**Acceptance Criteria**:
- [ ] GIF (or short video) demonstrates the complete workflow
- [ ] GIF is hosted in the repo (under `docs/`) or linked from a reliable URL
- [ ] README is updated to reference the new GIF
- [ ] GIF is under 10MB and loads reasonably

---

### 8. Add pre-commit hooks for Go/Python/React linting
**Difficulty**: Beginner | **Area**: DevOps

**Description**: Add a `.pre-commit-config.yaml` that runs `gofmt`, `ruff`, and ESLint on staged files before commits. This ensures code quality without relying on CI to catch issues.

**Files to touch**: `.pre-commit-config.yaml` (new file)

**Acceptance Criteria**:
- [ ] `.pre-commit-config.yaml` is configured with hooks for Go (gofmt), Python (ruff), and JavaScript (eslint)
- [ ] Running `pre-commit run --all-files` succeeds on clean code
- [ ] Installation instructions are documented in CONTRIBUTING.md
