#!/usr/bin/env bash
# =============================================================================
# verify_pipeline.sh — Local CI/CD mirror
# Runs the same stages as the GitHub Actions CI pipeline:
#   1. Lint (all 3 services)
#   2. Test (all 3 services with coverage)
#   3. Build Docker images
#   4. Smoke test (docker-compose up → health checks → teardown)
#
# Usage:  bash scripts/verify_pipeline.sh
# =============================================================================
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

pass() { echo -e "${GREEN}[PASS]${NC} $1"; }
fail() { echo -e "${RED}[FAIL]${NC} $1"; exit 1; }
stage() { echo -e "\n${YELLOW}━━━ $1 ━━━${NC}"; }

# ---------------------------------------------------------------------------
# STAGE 1: Lint
# ---------------------------------------------------------------------------
stage "Lint: Go (ingestion-service)"
cd "$ROOT/ingestion-service"
go vet ./... && pass "go vet" || fail "go vet"
test -z "$(gofmt -s -d .)" && pass "gofmt -s (no diffs)" || {
    echo "gofmt would make these changes:"
    gofmt -s -d .
    fail "gofmt check (run: gofmt -s -w .)"
}

stage "Lint: Python (processing-service)"
cd "$ROOT/processing-service"
pip install -q ruff
ruff check . && pass "ruff" || fail "ruff"

stage "Lint: Dashboard"
cd "$ROOT/dashboard"
npm ci --silent
npm run lint && pass "eslint" || fail "eslint"

# ---------------------------------------------------------------------------
# STAGE 2: Test with coverage
# ---------------------------------------------------------------------------
stage "Test: Go"
cd "$ROOT/ingestion-service"
go test -v -coverprofile=coverage.out -covermode=atomic ./... 2>&1 | tail -5
echo ""
go tool cover -func=coverage.out | tail -1
pass "go test"

stage "Test: Python"
cd "$ROOT/processing-service"
pip install -q pytest pytest-cov
pytest -v --cov=processor --cov-report=term --junitxml=junit.xml test_processor.py 2>&1 | tail -10
pass "pytest"

stage "Test: Dashboard"
cd "$ROOT/dashboard"
npm test 2>&1 | tail -10
pass "vitest"

# ---------------------------------------------------------------------------
# STAGE 3: Build Docker images
# ---------------------------------------------------------------------------
stage "Build: Docker images"
cd "$ROOT"
docker build -t sentinel-ingestion:test ./ingestion-service && pass "ingestion-service image" || fail "ingestion-service image"
docker build -t sentinel-processing:test ./processing-service && pass "processing-service image" || fail "processing-service image"
docker build -t sentinel-dashboard:test ./dashboard && pass "dashboard image" || fail "dashboard image"

# ---------------------------------------------------------------------------
# STAGE 4: Smoke test — start containers and check health endpoints
# ---------------------------------------------------------------------------
stage "Smoke: docker compose up & health check"
cd "$ROOT"

# Bring up infra + app services
docker compose up -d 2>/dev/null || true
echo "Waiting for services to start..."
sleep 10

HEALTH_FAIL=0

echo ""
echo "--- Health Checks ---"
if curl -sf http://localhost:8080/healthz > /dev/null 2>&1; then
    pass "ingestion-service  :8080/healthz"
else
    echo -e "${RED}[FAIL]${NC} ingestion-service health endpoint"
    HEALTH_FAIL=1
fi

if curl -sf http://localhost:8002/healthz > /dev/null 2>&1; then
    pass "processing-service  :8002/healthz"
else
    echo -e "${RED}[FAIL]${NC} processing-service health endpoint"
    HEALTH_FAIL=1
fi

if curl -sf http://localhost:3001/api/health > /dev/null 2>&1; then
    pass "dashboard  :3001/api/health"
else
    echo -e "${RED}[FAIL]${NC} dashboard health endpoint"
    HEALTH_FAIL=1
fi

stage "Teardown"
docker compose down -v 2>/dev/null || true
pass "containers stopped and cleaned up"

if [ "$HEALTH_FAIL" -ne 0 ]; then
    fail "One or more health checks failed"
fi

# ---------------------------------------------------------------------------
# Done
# ---------------------------------------------------------------------------
echo ""
echo -e "${GREEN}═══════════════════════════════════════════════════════════════${NC}"
echo -e "${GREEN}  All pipeline stages passed!${NC}"
echo -e "${GREEN}═══════════════════════════════════════════════════════════════${NC}"
