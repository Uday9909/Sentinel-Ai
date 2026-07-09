.PHONY: setup dev test lint clean help

help:
	@echo "Sentinel Development Targets"
	@echo "──────────────────────────────────────"
	@echo "make setup   — Start Docker infrastructure and print instructions"
	@echo "make dev     — Print instructions for starting all services"
	@echo "make test    — Run Go tests, Python tests, and React build"
	@echo "make lint    — Run Go fmt/vet, Python ruff, React lint"
	@echo "make clean   — Stop Docker containers and remove build artifacts"
	@echo ""

setup:
	@echo "Starting Docker infrastructure (Kafka, ES, Prometheus, Grafana)..."
	docker-compose up -d
	@echo ""
	@echo "Infrastructure is starting up. Run 'make dev' for next steps."
	@echo "Give Kafka ~30 seconds to become ready before starting services."

dev:
	@echo "Start each service in a separate terminal:"
	@echo ""
	@echo "  Terminal 1 — Ingestion API:"
	@echo "    cd ingestion-service && go run main.go"
	@echo ""
	@echo "  Terminal 2 — AI Processor:"
	@echo "    cd processing-service && python3 -m venv venv && source venv/bin/activate && pip install -r requirements.txt && python processor.py"
	@echo ""
	@echo "  Terminal 3 — Dashboard:"
	@echo "    cd dashboard && npm install && npm run dev"
	@echo ""
	@echo "  Dashboard: http://localhost:5173"
	@echo "  Ingestion API: http://localhost:8080"
	@echo "  Metrics: http://localhost:8080/metrics"
	@echo "  Grafana: http://localhost:3000"

test:
	@echo "Running Go tests..."
	cd ingestion-service && go test -v -race -cover ./...
	@echo ""
	@echo "Running Python tests..."
	cd processing-service && python3 -m pytest tests/ -v
	@echo ""
	@echo "Building React app..."
	cd dashboard && npm ci && npm run build

lint:
	@echo "Running Go fmt/vet..."
	cd ingestion-service && gofmt -l . && go vet ./...
	@echo ""
	@echo "Running Python Ruff..."
	cd processing-service && ruff check .
	@echo ""
	@echo "Running React ESLint..."
	cd dashboard && npm run lint

clean:
	@echo "Stopping Docker containers and removing volumes..."
	docker-compose down -v
	@echo ""
	@echo "Removing build artifacts..."
	rm -rf ingestion-service/tmp/
	@echo ""
	@echo "Clean complete."
