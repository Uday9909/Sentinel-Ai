# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Fixed

- Verify Kafka connectivity in ingestion service `/healthz` endpoint (#9)

### Added

- Async Kafka writes with bounded queue, exponential backoff retry, and DLQ routing (#29)
- Exponential backoff for dashboard polling on connection errors (#8)
- Initial open-source release
- Real-time log ingestion via Go (Gin) HTTP API on port 8080
- Kafka-based event streaming with service-level partitioning
- Isolation Forest anomaly detection with periodic model persistence
- Ollama LLM-powered Root Cause Analysis (RCA) using llama3.2:1b
- React + Vite "Mission Control" dashboard with real-time updates
- Elasticsearch log storage and search
- Prometheus metrics (logs ingested, anomalies detected, processing duration)
- Grafana dashboards with pre-configured provisioning
- Docker Compose setup for all infrastructure dependencies
- Story mode demo script generating realistic incident scenarios
- MIT license
- Contributor guidelines, code of conduct, and issue/PR templates
- CI/CD pipeline with Go, Python, and React checks
- Architecture Decision Records (ADR)
