# Contributing to Sentinel

First off, thanks for taking the time to contribute! 🎉

Sentinel is an open-source real-time anomaly detection and AI-powered root cause analysis platform. Every contribution — code, docs, bug reports, or design feedback — makes the project better.

---

## Ways to Contribute

- **Code** — Pick a [good first issue](https://github.com/Uday9909/Sentinel-Ai/issues?q=is%3Aissue+is%3Aopen+label%3A%22good+first+issue%22) or an unassigned bug/feature and submit a PR.
- **Documentation** — Improve README, fix typos, add examples, or write tutorials.
- **Bug Reports** — Open a GitHub issue with a clear reproduction. Include logs, environment details, and expected vs. actual behavior.
- **Feature Requests** — Open an issue describing the problem you're solving and the proposed solution.
- **Design Feedback** — Comment on UI/UX issues with mockups or suggestions.
- **Questions / Discussions** — Use GitHub Discussions to ask questions or share ideas.

---

## Development Environment Setup

### Prerequisites

| Tool | Minimum Version | How to Verify |
|------|----------------|---------------|
| Docker | 24+ | `docker --version` |
| Docker Compose | 2.20+ | `docker compose version` |
| Go | 1.21 | `go version` |
| Python | 3.9 | `python3 --version` |
| Node.js | 18 | `node --version` |
| Ollama | latest | `ollama --version` |

You also need the `llama3.2:1b` model pulled locally:

```bash
ollama pull llama3.2:1b
```

### Step-by-Step Setup

1. **Clone the repository**

   ```bash
   git clone https://github.com/Uday9909/Sentinel-Ai.git
   cd Sentinel-Ai
   ```

2. **Start infrastructure**

   ```bash
   docker-compose up -d
   ```

   This starts Kafka (with Zookeeper), Elasticsearch, Prometheus, and Grafana. Give it ~30 seconds for Kafka to become ready.

3. **Start the Go ingestion service**

   ```bash
   cd ingestion-service
   go run main.go
   ```

   The API will be available at `http://localhost:8080`.

4. **Start the Python AI processor** (in a new terminal)

   ```bash
   cd processing-service
   python3 -m venv venv
   source venv/bin/activate
   pip install -r requirements.txt
   python processor.py
   ```

5. **Start the React dashboard** (in another terminal)

   ```bash
   cd dashboard
   npm install
   npm run dev
   ```

   Visit `http://localhost:5173` to see the dashboard.

6. **Copy environment variables** (optional)

   ```bash
   cp .env.example .env
   # Edit .env as needed for your local setup
   ```

7. **Run the demo script** (optional)

   ```bash
   python3 scripts/demo_script.py
   ```

### Alternative: One-Command Setup

If you have `make` installed:

```bash
make setup     # Start infrastructure
make dev       # Print instructions for starting services
```

### Troubleshooting

| Symptom | Likely Cause | Fix |
|---------|-------------|-----|
| `kafka: client has run out of available brokers` | Kafka not ready yet | Wait 30s after `docker-compose up -d`, then restart the ingestion service |
| `Connection refused: localhost:11434` | Ollama not running | Start Ollama with `ollama serve` and pull the model |
| `Error: listen tcp :5173: bind: address already in use` | Port conflict | Kill the existing process on 5173 or change the Vite port in `vite.config.js` |
| `Error: listen tcp :9200: bind: address already in use` | Elasticsearch already running | Stop the existing ES instance or change the mapped port in `docker-compose.yml` |
| `elasticsearch.exceptions.ConnectionError` | Elasticsearch not healthy | Run `curl http://localhost:9200` — if no response, restart the ES container |
| `ModuleNotFoundError: No module named 'sklearn'` | Python deps not installed | Activate your venv and run `pip install -r requirements.txt` |

---

## Project Structure

```
Sentinel-Ai/
├── ingestion-service/       # Go (Gin) HTTP API — log ingestion gateway
│   ├── main.go              # HTTP handlers, Kafka producer, Prometheus metrics
│   ├── main_test.go         # Tests
│   ├── go.mod / go.sum
│   └── ...
├── processing-service/      # Python — ML anomaly detection + LLM RCA
│   ├── processor.py         # Kafka consumer, Isolation Forest, Ollama client
│   ├── requirements.txt
│   ├── isolation_forest.joblib  # Pre-trained model (regenerated at startup)
│   └── tests/
│       └── ...
├── dashboard/               # React + Vite — mission control UI
│   ├── src/
│   │   ├── App.jsx          # Main app component
│   │   ├── components/      # UI components
│   │   └── ...
│   ├── package.json
│   └── vite.config.js
├── scripts/                 # Demo and helper scripts
│   └── demo_script.py
├── docker-compose.yml       # Infrastructure (Kafka, ES, Prometheus, Grafana)
├── prometheus.yml           # Prometheus scrape config
├── grafana/                 # Grafana provisioning
└── docs/
    └── adr/                 # Architecture Decision Records
```

---

## Coding Standards

### Go

- Format with `gofmt` (run `gofmt -w .`)
- Static analysis with `go vet` (run `go vet ./...`)
- All exported symbols must have doc comments
- Follow [Effective Go](https://go.dev/doc/effective_go) conventions

### Python

- Format with [Black](https://github.com/psf/black) (`black .`)
- Lint with [Ruff](https://github.com/astral-sh/ruff) (`ruff check .`)
- Type hints encouraged for public functions

### React / JavaScript

- Format with [Prettier](https://prettier.io/) (`npx prettier --write .`)
- Lint with ESLint (`cd dashboard && npm run lint`)
- Use functional components and hooks

---

## Commit Message Format

We follow [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>: <short description>

[optional body]
```

Types: `feat`, `fix`, `docs`, `refactor`, `test`, `chore`, `perf`, `ci`

Examples:
```
feat: add health check endpoint to ingestion service
fix: handle null timestamp in log parser
docs: update API examples in README
```

---

## Pull Request Process

1. **Branch from `main`**: `git checkout -b feat/my-feature`
2. **Make your changes** — keep commits atomic and well-described
3. **Run checks locally**:
   ```bash
   make lint     # or individual lint commands
   make test     # or individual test commands
   ```
4. **Open a PR** against `main` with a clear title and description
5. **Link the issue** your PR addresses (e.g., `Closes #12`)
6. **Request review** from at least one maintainer
7. **CI must pass** before merge
8. **Squash merge** is preferred to keep history clean

### PR Checklist

- [ ] Tests added/updated for new code
- [ ] Documentation updated (README, inline docs, or ADRs as needed)
- [ ] CHANGELOG.md updated under `[Unreleased]`
- [ ] PR linked to a related issue
- [ ] Screenshot attached if UI changed

---

## AI-Assisted Contributions

We welcome AI-assisted development, but require disclosure. If you use AI tools (GitHub Copilot, Claude Code, Cursor, etc.) to generate non-trivial code:

1. **Disclose it** in your PR description: "AI-assisted: Used Copilot to scaffold the React component"
2. **Review it yourself** — AI code can have subtle bugs, security issues, or hallucinated APIs
3. **Test it** — AI-generated tests might pass for the wrong reasons
4. **You are responsible** — By submitting, you certify the code is correct and properly licensed

Trivial changes (auto-formatting, variable renames, comment generation) do not need disclosure.

---

## Good First Issues

Check out the [`good first issue`](https://github.com/Uday9909/Sentinel-Ai/issues?q=is%3Aissue+is%3Aopen+label%3A%22good+first+issue%22) label on GitHub for starter-friendly tasks. See [GOOD_FIRST_ISSUES.md](./GOOD_FIRST_ISSUES.md) for detailed descriptions.

---

## Getting Help

- Open a [GitHub Discussion](https://github.com/Uday9909/Sentinel-Ai/discussions) for questions
- File a [bug report](https://github.com/Uday9909/Sentinel-Ai/issues/new?labels=bug&template=bug_report.yml) for issues
- Reach out via email: `writetoudaybir@gmail.com`

---

> **Note**: By contributing, you agree that your contributions will be licensed under the MIT License.
