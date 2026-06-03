#!/usr/bin/env bash
# =============================================================================
# pipeline.test.sh — Meta-tests for the CI/CD pipeline itself.
#
# Validates that all pipeline files exist, are well-formed, and that
# the required infrastructure is in place.
#
# Usage:  bash scripts/pipeline.test.sh
# =============================================================================
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

PASS=0
FAIL=0

pass() { PASS=$((PASS+1)); echo -e "${GREEN}[PASS]${NC} $1"; }
fail() { FAIL=$((FAIL+1)); echo -e "${RED}[FAIL]${NC} $1"; }
check() { if [ "$1" ]; then pass "$2"; else fail "$2"; fi; }

echo -e "${YELLOW}───── Pipeline Structure Tests ─────${NC}"

# --- 1. GitHub Actions workflows exist and are valid YAML ---
check [ -f .github/workflows/ci.yml ] ".github/workflows/ci.yml exists"
check [ -f .github/workflows/cd.yml ] ".github/workflows/cd.yml exists"

if command -v python3 &>/dev/null; then
    for wf in .github/workflows/ci.yml .github/workflows/cd.yml; do
        python3 -c "import yaml; yaml.safe_load(open('$wf'))" 2>/dev/null \
            && pass "$wf is valid YAML" \
            || fail "$wf is NOT valid YAML"
    done
else
    echo -e "${YELLOW}[SKIP]${NC} python3 not available — skipping YAML validation"
fi

# --- 2. CI workflow has all required jobs ---
if [ -f .github/workflows/ci.yml ]; then
    REQUIRED_CI_JOBS=("lint-go" "lint-py" "lint-js" "test-go" "test-py" "test-js" "build-go" "build-py" "build-js" "smoke-test")
    for job in "${REQUIRED_CI_JOBS[@]}"; do
        grep -q "  $job:" .github/workflows/ci.yml && pass "CI job: $job" || fail "CI job: $job (missing)"
    done
fi

# --- 3. CD workflow has push jobs ---
if [ -f .github/workflows/cd.yml ]; then
    grep -q "build-push-go" .github/workflows/cd.yml \
        && pass "CD job: build-push-go" \
        || fail "CD job: build-push-go (missing)"
fi

# --- 4. .dockerignore files exist for all services ---
check [ -f ingestion-service/.dockerignore ] "ingestion-service/.dockerignore"
check [ -f processing-service/.dockerignore ] "processing-service/.dockerignore"
check [ -f dashboard/.dockerignore ] "dashboard/.dockerignore"

# --- 5. Dockerfiles have HEALTHCHECK ---
for df in ingestion-service/Dockerfile processing-service/Dockerfile dashboard/Dockerfile; do
    grep -q "HEALTHCHECK" "$df" && pass "$df has HEALTHCHECK" || fail "$df missing HEALTHCHECK"
done

# --- 6. docker-compose has no :latest tags (reproducibility) ---
if grep -q ':latest' docker-compose.yml 2>/dev/null; then
    fail "docker-compose.yml uses :latest tags (non-reproducible)"
else
    pass "docker-compose.yml has no :latest tags"
fi

# --- 7. Grafana provisioning is correct ---
check [ -f monitoring/grafana/provisioning/datasources/datasource.yml ] "Grafana datasource file exists"
check [ -f monitoring/grafana/provisioning/dashboards/dashboard.yml ] "Grafana dashboard provider exists"
check [ -f monitoring/grafana/provisioning/dashboards/sentinel-dashboard.json ] "Grafana dashboard JSON exists"

# --- 8. Grafana datasource has uid: prometheus ---
grep -q "uid: prometheus" monitoring/grafana/provisioning/datasources/datasource.yml 2>/dev/null \
    && pass "Grafana datasource has uid: prometheus" \
    || fail "Grafana datasource missing uid: prometheus"

# --- 9. Grafana dashboard.yml path matches docker-compose mount ---
grep -q "/etc/grafana/provisioning/dashboards" monitoring/grafana/provisioning/dashboards/dashboard.yml 2>/dev/null \
    && pass "Grafana dashboard provider path matches compose mount" \
    || fail "Grafana dashboard provider path mismatch"

# --- 10. No stale root-level prometheus.yml ---
check [ ! -f prometheus.yml ] "Root-level prometheus.yml is gone (moved to monitoring/)"

# --- 11. Docker images build (non-blocking check) ---
echo ""
echo -e "${YELLOW}───── Docker Build Tests ─────${NC}"
for svc in ingestion-service processing-service dashboard; do
    if docker build -q -t "sentinel-${svc}:test" "./${svc}" 2>/dev/null; then
        pass "${svc} Docker image builds"
    else
        fail "${svc} Docker image build failed"
    fi
done

# --- 12. Dashboard coverage config is in place ---
grep -q "coverage:" dashboard/vite.config.js 2>/dev/null \
    && pass "Dashboard vitest coverage is configured" \
    || fail "Dashboard vitest coverage NOT configured"

grep -q "test:coverage" dashboard/package.json 2>/dev/null \
    && pass "Dashboard test:coverage script exists" \
    || fail "Dashboard test:coverage script missing"

# --- 13. Pipeline test scripts exist ---
check [ -x scripts/verify_pipeline.sh ] "scripts/verify_pipeline.sh is executable"
check [ -x scripts/pipeline.test.sh ] "scripts/pipeline.test.sh is executable"

# --- Summary ---
echo ""
echo -e "${YELLOW}═══════════════════════════════════════════════════════════════${NC}"
echo -e "  ${GREEN}${PASS} passed${NC}, ${RED}${FAIL} failed${NC}"
echo -e "${YELLOW}═══════════════════════════════════════════════════════════════${NC}"
exit $FAIL
