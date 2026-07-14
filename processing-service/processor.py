import json
import logging
import os
import signal
import threading
import time
from collections import deque
from http.server import BaseHTTPRequestHandler, HTTPServer

import ollama
from drain3 import TemplateMiner
from drain3.template_miner_config import TemplateMinerConfig
from elasticsearch import Elasticsearch
from joblib import dump, load
from kafka import KafkaConsumer
from prometheus_client import Counter, Histogram, start_http_server
from sklearn.ensemble import IsolationForest

logger = logging.getLogger(__name__)

# --- CONFIG ---
_model_dir = os.getenv("MODEL_DIR", "")
if _model_dir:
    os.makedirs(_model_dir, exist_ok=True)
    MODEL_PATH = os.path.join(_model_dir, "isolation_forest.joblib")
else:
    MODEL_PATH = os.getenv("MODEL_PATH", "isolation_forest.joblib")

TRAIN_INTERVAL = 60          # seconds between model retrains
ANOMALY_RATE_FLOOR = 20.0    # minimum logs/sec to consider rate-based anomaly
OLLAMA_COOLDOWN_SEC = 30     # skip LLM if it failed within this many seconds
KAFKA_RECONNECT_INITIAL_DELAY_SEC = 1
KAFKA_RECONNECT_MAX_DELAY_SEC = 60
KAFKA_RETRY_BACKOFF_MS = 1000
KAFKA_RECONNECT_BACKOFF_MS = 1000
KAFKA_RECONNECT_BACKOFF_MAX_MS = 60000

# Ollama LLM client — host is configurable for K8s / Docker networking
OLLAMA_HOST = os.getenv("OLLAMA_HOST", "http://localhost:11434")
_ollama_client = ollama.Client(host=OLLAMA_HOST)


# ---------------------------------------------------------------------------
# Health check server (port 8002) for Kubernetes liveness / readiness probes
# ---------------------------------------------------------------------------

class _HealthHandler(BaseHTTPRequestHandler):
    """Minimal handler that responds to GET /healthz with 200 OK."""

    def do_GET(self):
        if self.path == "/healthz":
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.end_headers()
            self.wfile.write(b'{"status": "ok"}')
        else:
            self.send_response(404)
            self.end_headers()

    def log_message(self, format, *args):  # noqa: A002
        """Suppress default stderr logging for health checks."""
        pass


def start_health_server(port: int = 8002) -> None:
    """Start a background HTTP server for K8s health probes."""
    server = HTTPServer(("0.0.0.0", port), _HealthHandler)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    logger.info("Health check server running on port %s", port)

# --- PROMETHEUS METRICS ---
LOGS_PROCESSED = Counter('logs_processed_total', 'Total logs consumed from Kafka')
ANOMALIES_DETECTED = Counter('anomalies_detected_total', 'Total anomalies identified by AI')
PROCESSING_TIME = Histogram('processing_duration_seconds', 'Time taken to analyze a log batch')
ES_WRITE_ERRORS = Counter('es_write_errors_total', 'Total Elasticsearch write failures')


# ---------------------------------------------------------------------------
# Detection helpers (extracted for testability)
# ---------------------------------------------------------------------------

def has_keyword_indicator(log_text: str) -> bool:
    """Return True if the log text contains known severity keywords."""
    return any(kw in log_text.lower() for kw in ("error", "critical", "fail"))


def train_if_needed(model, rate_history, last_train_time):
    """Retrain the IsolationForest model if it is missing or stale."""
    now = time.time()
    if model is not None and (now - last_train_time < TRAIN_INTERVAL):
        return model, last_train_time
    if len(rate_history) < 10:
        return model, last_train_time
    try:
        new_model = IsolationForest(contamination=0.05, random_state=42)
        new_model.fit(list(rate_history))
        dump(new_model, MODEL_PATH)
        logger.info("Model retrained and persisted.")
        return new_model, now
    except Exception as e:
        logger.exception("Model training failed: %s", e)
        return model, last_train_time


def check_rate_anomaly(logs_per_sec: float, rate_history, model) -> bool:
    """Use IsolationForest to detect anomalous log rates."""
    if model is None or len(rate_history) < 10:
        return False
    current_rate_vector = [[logs_per_sec]]
    try:
        if model.predict(current_rate_vector)[0] == -1 and logs_per_sec > ANOMALY_RATE_FLOOR:
            return True
    except Exception:
        pass
    return False


def run_ai_analysis(log_text, log_data, last_failure_time):
    """Call Ollama for RCA.  Returns (is_anomaly, ai_summary, new_failure_time)."""
    now = time.time()
    if now - last_failure_time < OLLAMA_COOLDOWN_SEC:
        return True, "AI Analysis Unavailable (Ollama cooldown)", last_failure_time

    try:
        response = _ollama_client.chat(model='llama3.2:1b', messages=[
            {
                'role': 'system',
                'content': (
                    "You are a Senior DevOps Engineer. Analyze the log entry. "
                    "If it indicates a system failure, error, or critical issue, provide "
                    "a 1-sentence explanation and 2 short fixes. If the log is just "
                    "informational, debug, or success, standard system activity, reply "
                    "ONLY with the word 'Normal'."
                )
            },
            {
                'role': 'user',
                'content': (
                    f"Log Entry: '{log_text}'\n"
                    f"Service: {log_data.get('service')}\n"
                    f"TraceID: {log_data.get('trace_id', 'N/A')}\n"
                    f"Host: {log_data.get('host', 'N/A')}"
                )
            },
        ])
        ai_content = response['message']['content'].strip()
        if "Normal" in ai_content and len(ai_content) < 15:
            return False, "Normal", last_failure_time
        else:
            logger.info("AI ANALYSIS:\n%s", ai_content)
            return True, ai_content, last_failure_time
    except Exception as e:
        logger.exception("AI Analysis Failed: %s", e)
        return True, "AI Analysis Unavailable", now


def build_kafka_consumer(kafka_broker):
    """Build a Kafka consumer with reconnect-friendly client backoff settings."""
    return KafkaConsumer(
        'raw-logs',
        bootstrap_servers=[kafka_broker],
        value_deserializer=lambda m: m.decode('utf-8'),
        group_id='sentinel-processor',
        auto_offset_reset='earliest',
        retry_backoff_ms=KAFKA_RETRY_BACKOFF_MS,
        reconnect_backoff_ms=KAFKA_RECONNECT_BACKOFF_MS,
        reconnect_backoff_max_ms=KAFKA_RECONNECT_BACKOFF_MAX_MS,
    )


def get_next_reconnect_delay(previous_delay):
    """Return the next reconnect delay using exponential backoff with a hard cap."""
    if previous_delay <= 0:
        return KAFKA_RECONNECT_INITIAL_DELAY_SEC
    return min(previous_delay * 2, KAFKA_RECONNECT_MAX_DELAY_SEC)


def process_kafka_stream(
    kafka_broker,
    template_miner,
    es,
    log_timestamps,
    rate_history,
    model,
    last_train_time,
    last_ollama_failure,
    stop_event,
    wait_fn,
):
    """Consume Kafka logs, reconnecting with backoff if the consumer fails."""
    reconnect_delay = KAFKA_RECONNECT_INITIAL_DELAY_SEC

    while not stop_event.is_set():
        consumer = None
        try:
            consumer = build_kafka_consumer(kafka_broker)
            reconnect_delay = KAFKA_RECONNECT_INITIAL_DELAY_SEC
            logger.info("Connected to Kafka broker %s", kafka_broker)

            for message in consumer:
                if stop_event.is_set():
                    break

                start_time = time.time()
                LOGS_PROCESSED.inc()

                # --- Parse ---
                try:
                    log_data = json.loads(message.value)
                    if isinstance(log_data, str):
                        log_text = log_data
                        log_data = {"message": log_text, "service": "unknown", "level": "unknown"}
                    else:
                        log_text = log_data.get("message", "")
                except json.JSONDecodeError:
                    log_text = message.value
                    log_data = {"message": log_text, "service": "unknown", "level": "unknown"}

                # --- Template mining (Drain3) ---
                template_result = template_miner.add_log_message(log_text)
                cluster_id = template_result.get("cluster_id", 0)

                # --- Log rate (sliding window) ---
                current_time = time.time()
                log_timestamps.append(current_time)

                logs_per_sec = 0.0
                if len(log_timestamps) > 1:
                    time_span = log_timestamps[-1] - log_timestamps[0]
                    if time_span > 0:
                        logs_per_sec = len(log_timestamps) / time_span
                        rate_history.append([logs_per_sec])

                # --- ANOMALY DETECTION ---
                # Each detector runs independently — no single check short-circuits the others.
                keyword_anomaly = has_keyword_indicator(log_text)

                model, last_train_time = train_if_needed(model, rate_history, last_train_time)
                rate_anomaly = check_rate_anomaly(logs_per_sec, rate_history, model)

                is_anomaly = keyword_anomaly or rate_anomaly

                # --- AI ANALYSIS (only for anomalies) ---
                ai_summary = "Normal"
                if is_anomaly:
                    ANOMALIES_DETECTED.inc()
                    logger.info("ANOMALY DETECTED! (Rate: %.1f/s) Consulting AI...", logs_per_sec)
                    is_anomaly, ai_summary, last_ollama_failure = run_ai_analysis(
                        log_text, log_data, last_ollama_failure
                    )

                # --- Persist to Elasticsearch ---
                document = {
                    "message": log_text,
                    "service": log_data.get("service", "unknown"),
                    "level": log_data.get("level", "unknown"),
                    "trace_id": log_data.get("trace_id", ""),
                    "host": log_data.get("host", ""),
                    "timestamp_log": log_data.get("timestamp", 0),
                    "is_anomaly": is_anomaly,
                    "ai_explanation": ai_summary,
                    "cluster_id": cluster_id,
                    "timestamp_processed": time.time(),
                }
                try:
                    es.index(index="logs-index", document=document)
                except Exception as e:
                    ES_WRITE_ERRORS.inc()
                    logger.exception("ES write failed: %s", e)

                PROCESSING_TIME.observe(time.time() - start_time)
        except KeyboardInterrupt:
            break
        except Exception as e:
            if stop_event.is_set():
                break

            logger.exception("Kafka consumer error: %s", e)
            logger.info("Reconnecting to Kafka in %s seconds", reconnect_delay)
            try:
                if wait_fn(reconnect_delay):
                    break
            except KeyboardInterrupt:
                break
            reconnect_delay = get_next_reconnect_delay(reconnect_delay)
        finally:
            if consumer is not None:
                consumer.close()
                logger.info("Kafka consumer closed.")

    return model, last_train_time, last_ollama_failure


# ---------------------------------------------------------------------------
# Main loop
# ---------------------------------------------------------------------------

def main():
    logging.basicConfig(
        level=getattr(logging, os.getenv("LOG_LEVEL", "INFO").upper(), logging.INFO),
        format="%(asctime)s %(levelname)s %(name)s: %(message)s",
    )
    template_miner = TemplateMiner(config=TemplateMinerConfig())
    es_url = os.getenv("ES_URL", "http://localhost:9200")
    kafka_broker = os.getenv("KAFKA_BROKER", "localhost:9092")
    es = Elasticsearch([es_url])

    logger.info("AI-Powered SRE Agent is ONLINE.")

    start_http_server(8001)
    logger.info("Metrics Server running on port 8001")

    start_health_server(8002)

    log_timestamps = deque(maxlen=100)
    rate_history = deque(maxlen=50)

    model = None
    if os.path.exists(MODEL_PATH):
        try:
            model = load(MODEL_PATH)
            logger.info("Loaded persisted anomaly model.")
        except Exception as e:
            logger.exception("Failed to load model: %s", e)

    last_train_time = time.time()
    last_ollama_failure = 0.0
    stop_event = threading.Event()

    # --- Graceful shutdown ---
    def handle_sigterm(signum, frame):
        logger.info("Shutdown requested...")
        stop_event.set()
        raise KeyboardInterrupt()

    signal.signal(signal.SIGTERM, handle_sigterm)

    try:
        model, last_train_time, last_ollama_failure = process_kafka_stream(
            kafka_broker=kafka_broker,
            template_miner=template_miner,
            es=es,
            log_timestamps=log_timestamps,
            rate_history=rate_history,
            model=model,
            last_train_time=last_train_time,
            last_ollama_failure=last_ollama_failure,
            stop_event=stop_event,
            wait_fn=stop_event.wait,
        )
    except KeyboardInterrupt:
        logger.info("Shutting down gracefully...")
    finally:
        logger.info("Kafka consumer loop stopped.")


if __name__ == "__main__":
    main()
