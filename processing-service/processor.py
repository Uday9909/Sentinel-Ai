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

msg_count = 0
for message in consumer:
    start_time = time.time()
    LOGS_PROCESSED.inc()
    
    log_text = message.value
    result = template_miner.add_log_message(log_text)
    msg_count += 1

    # --- ANOMALY DETECTION ---
    is_anomaly = False
    ai_summary = "Normal"
    
    # Simple training threshold for demo purposes
    if msg_count > 5:
        # Mock history for isolation forest (in prod, use real persisted history)
        history = np.array([[1], [2], [1], [3], [2], [msg_count]])
        model = IsolationForest(contamination=0.1).fit(history)
        
        # If the current message count pattern is an outlier
        if model.predict([[msg_count]])[0] == -1:
            is_anomaly = True
            ANOMALIES_DETECTED.inc()
            print("🚨 ANOMALY DETECTED! Consulting AI for a fix...")

            try:
                # --- THE GEN-AI BRAIN ---
                response = ollama.chat(model='llama3.2:1b', messages=[
                    {
                        'role': 'user',
                        'content': f"You are a Senior DevOps Engineer. We have an outage. The error log is: '{log_text}'. Explain what caused this in 1 sentence and give 2 bullet points on how to fix it.",
                    },
                ])
                ai_summary = response['message']['content']
                print(f"\n💡 AI ANALYSIS:\n{ai_summary}\n")
            except Exception as e:
                print(f"AI Analysis Failed: {e}")
                ai_summary = "AI Analysis Unavailable"

    # Save everything to the DB
    document = {
        "message": log_text,
        "is_anomaly": is_anomaly,
        "ai_explanation": ai_summary,
        "timestamp": time.time()
    }
    es.index(index="logs-index", document=document)
    
    PROCESSING_TIME.observe(time.time() - start_time)