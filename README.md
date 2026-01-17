# 🛰️ Sentinel Platform

**Real-time Incident Command & Log Intelligence Platform**

Sentinel is an end-to-end streaming observability platform that ingests logs, detects anomalies in real-time using unsupervised machine learning, and leverages Large Language Models (LLMs) to provide instant Root Cause Analysis (RCA).

![Dashboard Preview](https://via.placeholder.com/1200x600?text=Sentinel+Dashboard+Preview)

---

## 🏗 Architecture

```mermaid
graph LR
    A[Producer Services] -->|HTTP POST| B[Ingestion Service (Go)]
    B -->|Log Events| C[Apache Kafka]
    C -->|Consume| D[Processor Service (Python)]
    
    subgraph "AI Core"
    D -->|Buffer & Train| E[Isolation Forest Model]
    D -->|Query| F[Ollama (Llama 3.2)]
    end
    
    D -->|Index| G[Elasticsearch]
    G <-->|Query| H[Dashboard (React + Vite)]
```

## 🚀 Technology Stack

| Component | Tech | Responsibility |
|-----------|------|----------------|
| **Ingestion** | **Go (Gin)** | High-throughput API gateway handling thousands of reqs/sec. |
| **Messaging** | **Kafka** | Durable event streaming and backpressure management. |
| **Processing** | **Python 3** | Anomaly detection (`scikit-learn`) and AI orchestration. |
| **Intelligence** | **Ollama** | Local LLM inference for Root Cause Analysis. |
| **Storage** | **Elasticsearch** | Indexed log storage for fast search and aggregation. |
| **UI** | **React + Vite** | Real-time "Mission Control" dashboard. |

---

## 🛠 Quick Start

### Prerequisites
- Docker & Docker Compose
- Node.js v18+
- Python 3.9+
- Go 1.21+
- [Ollama](https://ollama.com/) running locally with `llama3.2:1b` model.

### 1. Start Infrastructure
```bash
# Start Kafka, Zookeeper, Elasticsearch
docker-compose up -d
```

### 2. Start Services
Open 3 separate terminals:

**Terminal 1: Ingestion API**
```bash
cd ingestion-service
go run main.go
```

**Terminal 2: AI Processor**
```bash
cd processing-service
python3 -m venv venv
source venv/bin/activate
pip install -r requirements.txt
python processor.py
```

**Terminal 3: Mission Control Dashboard**
```bash
cd dashboard
npm install
npm run dev
```

Visit the dashboard at [http://localhost:5173](http://localhost:5173).

---

## 🎬 Demo "Story Mode"

To demonstrate the platform's capabilities (e.g., for a LinkedIn demo), run the included scenario script. This generates a sequence of:
1.  **Normal Traffic** (Healthy logs)
2.  **Warning Signals** (Latency spikes, retries)
3.  **CRITICAL ANOMALY** (Burst of database failures triggering the AI)

```bash
python3 scripts/demo_script.py
```

---

## 🔮 Roadmap
See [roadmap.md](./roadmap.md) for full engineering plans, including:
- [ ] Phase 1: Observability (Prometheus/Grafana)
- [ ] Phase 2: Chaos Engineering & Reliability
- [ ] Phase 3: Distributed Tracing with Jaeger
- [ ] Phase 4: Kubernetes Migration
