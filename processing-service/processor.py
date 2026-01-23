from kafka import KafkaConsumer
from elasticsearch import Elasticsearch
from drain3 import TemplateMiner
from drain3.template_miner_config import TemplateMinerConfig
from sklearn.ensemble import IsolationForest
from prometheus_client import start_http_server, Counter, Histogram
import ollama
import numpy as np
import json
import time
import os
from collections import deque
from joblib import dump, load

# --- PROMETHEUS METRICS ---
LOGS_PROCESSED = Counter('logs_processed_total', 'Total logs consumed from Kafka')
ANOMALIES_DETECTED = Counter('anomalies_detected_total', 'Total anomalies identified by AI')
PROCESSING_TIME = Histogram('processing_duration_seconds', 'Time taken to analyze a log batch')

# 1. Setup Everything
template_miner = TemplateMiner(config=TemplateMinerConfig())
es = Elasticsearch(["http://localhost:9200"])
consumer = KafkaConsumer('raw-logs', bootstrap_servers=['localhost:9092'], value_deserializer=lambda m: m.decode('utf-8'))

print("🤖 AI-Powered SRE Agent is ONLINE.")

# Start Prometheus Metrics Server on Port 8001
start_http_server(8001)
print("📊 Metrics Server running on port 8001")

# Use a sliding window for anomaly detection (History of log rates)
log_timestamps = deque(maxlen=100)
rate_history = deque(maxlen=50) 
NORMAL_RATE_THRESHOLD = 50.0 # logs/sec guess
MODEL_PATH = "isolation_forest.joblib"
LAST_TRAIN_TIME = time.time()
TRAIN_INTERVAL = 60 # seconds

# Load existing model if available
model = None
if os.path.exists(MODEL_PATH):
    try:
        model = load(MODEL_PATH)
        print("✅ Loaded persisted anomaly model.")
    except Exception as e:
        print(f"⚠️ Failed to load model: {e}")

for message in consumer:
    start_time = time.time()
    LOGS_PROCESSED.inc()
    
    # Parse JSON Log
    try:
        log_data = json.loads(message.value)
        # Handle case where message might still be raw text (backward compatibility/safety)
        if isinstance(log_data, str):
             log_text = log_data
             log_data = {"message": log_text, "service": "unknown", "level": "unknown"}
        else:
             log_text = log_data.get("message", "")
    except json.JSONDecodeError:
        log_text = message.value
        log_data = {"message": log_text, "service": "unknown", "level": "unknown"}
        
    result = template_miner.add_log_message(log_text)
    
    # Calculate Log Rate (Sliding Window)
    current_time = time.time()
    log_timestamps.append(current_time)
    
    logs_per_sec = 0
    if len(log_timestamps) > 1:
        time_span = log_timestamps[-1] - log_timestamps[0]
        if time_span > 0:
            logs_per_sec = len(log_timestamps) / time_span
            rate_history.append([logs_per_sec])

    # --- ANOMALY DETECTION ---
    is_anomaly = False
    ai_summary = "Normal"
    
    # 1. Keyword Heuristic (Immediate Flag)
    if "error" in log_text.lower() or "critical" in log_text.lower() or "fail" in log_text.lower():
        is_anomaly = True
    
    # 2. Statistical Anomaly (Spike Detection)
    elif len(rate_history) > 10:
        # Train/Update Model Periodically
        if model is None or (current_time - LAST_TRAIN_TIME > TRAIN_INTERVAL):
             try:
                # Re-train on recent history (sliding window)
                # In a real system, you'd want a larger, persisted training set
                new_model = IsolationForest(contamination=0.05, random_state=42)
                new_model.fit(list(rate_history))
                model = new_model
                dump(model, MODEL_PATH)
                LAST_TRAIN_TIME = current_time
                print("🔄 Model retrained and persisted.")
             except Exception as e:
                 print(f"⚠️ Model training failed: {e}")

        # Predict using current model
        if model:
             current_rate_vector = [[logs_per_sec]]
             if model.predict(current_rate_vector)[0] == -1:
                 # Only flag if rate is also significantly high (avoid low-rate anomalies)
                 if logs_per_sec > 20: 
                     is_anomaly = True

    if is_anomaly:
        ANOMALIES_DETECTED.inc()
        print(f"🚨 ANOMALY DETECTED! (Rate: {logs_per_sec:.1f}/s) Consulting AI...")

        try:
            # --- THE GEN-AI BRAIN (Improved Prompt) ---
            response = ollama.chat(model='llama3.2:1b', messages=[
                {
                    'role': 'system', 
                    'content': "You are a Senior DevOps Engineer. Analyze the log entry. If it indicates a system failure, error, or critical issue, provide a 1-sentence explanation and 2 short fixes. If the log is just informational, debug, or success, standard system activity, reply ONLY with the word 'Normal'."
                },
                {
                    'role': 'user',
                    'content': f"Log Entry: '{log_text}'\nService: {log_data.get('service')}\nTraceID: {log_data.get('trace_id', 'N/A')}\nHost: {log_data.get('host', 'N/A')}"
                },
            ])
            
            ai_content = response['message']['content'].strip()
            
            if "Normal" in ai_content and len(ai_content) < 15:
                is_anomaly = False # False positive corrected by AI
                ai_summary = "Normal"
            else:
                ai_summary = ai_content
                print(f"\n💡 AI ANALYSIS:\n{ai_summary}\n")
                
        except Exception as e:
            print(f"AI Analysis Failed: {e}")
            ai_summary = "AI Analysis Unavailable"

    # Save everything to the DB
    document = {
        "message": log_text,
        "service": log_data.get("service", "unknown"),
        "level": log_data.get("level", "unknown"),
        "trace_id": log_data.get("trace_id", ""),
        "host": log_data.get("host", ""),
        "timestamp_log": log_data.get("timestamp", 0),
        "is_anomaly": is_anomaly,
        "ai_explanation": ai_summary,
        "timestamp_processed": time.time()
    }
    es.index(index="logs-index", document=document)
    
    PROCESSING_TIME.observe(time.time() - start_time)
