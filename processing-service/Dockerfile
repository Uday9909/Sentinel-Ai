# syntax=docker/dockerfile:1
FROM python:3.11-slim

# build-essential needed for scikit-learn native extensions
RUN apt-get update && \
    apt-get install -y --no-install-recommends build-essential && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY processor.py .

# The IsolationForest model is persisted here at runtime (optional).
VOLUME /app/data

# 8001 = Prometheus metrics, 8002 = health check
EXPOSE 8001 8002
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
  CMD python3 -c "import urllib.request; urllib.request.urlopen('http://localhost:8002/healthz')" || exit 1
CMD ["python", "processor.py"]
