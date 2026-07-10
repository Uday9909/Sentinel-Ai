# Good First Issues

> **Last reviewed:** July 9, 2026
>
> For the most up-to-date list, see issues labeled [`good first issue`](https://github.com/Uday9909/Sentinel-Ai/issues?q=is%3Aissue+is%3Aopen+label%3A%22good+first+issue%22) on GitHub.

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

### 2. Create a Makefile target for one-command local development
**Difficulty**: Beginner | **Area**: DevOps

**Description**: The project has a Makefile but `make dev` only prints instructions. Create a `make run-all` target that orchestrates starting Docker infrastructure and each service (or at minimum, prints copy-pasteable multi-terminal commands).

**Files to touch**: `Makefile`

**Acceptance Criteria**:
- [ ] `make run-all` starts infrastructure and prints clear instructions for each service
- [ ] The target works on both macOS and Linux
- [ ] Error handling if Docker is not running

---

### 3. Improve README with a GIF demo instead of static screenshot
**Difficulty**: Beginner | **Area**: Documentation

**Description**: The README has a placeholder screenshot. Create a screen recording GIF showing the full pipeline — sending a log via curl, seeing it appear in the dashboard, and an anomaly being detected and analyzed.

**Files to touch**: `README.md`, a new `docs/demo.gif` or `docs/demo.mp4`

**Acceptance Criteria**:
- [ ] GIF (or short video) demonstrates the complete workflow
- [ ] GIF is hosted in the repo (under `docs/`) or linked from a reliable URL
- [ ] README is updated to reference the new GIF
- [ ] GIF is under 10MB and loads reasonably

---

### 4. Add pre-commit hooks for Go/Python/React linting
**Difficulty**: Beginner | **Area**: DevOps

**Description**: Add a `.pre-commit-config.yaml` that runs `gofmt`, `ruff`, and ESLint on staged files before commits. This ensures code quality without relying on CI to catch issues.

**Files to touch**: `.pre-commit-config.yaml` (new file)

**Acceptance Criteria**:
- [ ] `.pre-commit-config.yaml` is configured with hooks for Go (gofmt), Python (ruff), and JavaScript (eslint)
- [ ] Running `pre-commit run --all-files` succeeds on clean code
- [ ] Installation instructions are documented in CONTRIBUTING.md
