from kafka import KafkaConsumer
from elasticsearch import Elasticsearch
from drain3 import TemplateMiner
from drain3.template_miner_config import TemplateMinerConfig
from sklearn.ensemble import IsolationForest
import ollama  # <--- The AI Library
import numpy as np
import json
import time

# 1. Setup Everything
template_miner = TemplateMiner(config=TemplateMinerConfig())
es = Elasticsearch(["http://localhost:9200"])
consumer = KafkaConsumer('raw-logs', bootstrap_servers=['localhost:9092'], value_deserializer=lambda m: m.decode('utf-8'))

print("🤖 AI-Powered SRE Agent is ONLINE.")

msg_count = 0
for message in consumer:
    log_text = message.value
    result = template_miner.add_log_message(log_text)
    msg_count += 1

    # --- ANOMALY DETECTION (Same as before) ---
    is_anomaly = False
    if msg_count > 5:
        history = np.array([[1], [2], [1], [3], [2], [msg_count]])
        model = IsolationForest(contamination=0.1).fit(history)
        if model.predict([[msg_count]])[0] == -1:
            is_anomaly = True
            print("🚨 ANOMALY DETECTED! Consulting AI for a fix...")

            # --- THE GEN-AI BRAIN ---
            # We ask the local Llama model to explain the error
            response = ollama.chat(model='llama3.2:1b', messages=[
                {
                    'role': 'user',
                    'content': f"You are a Senior DevOps Engineer. We have an outage. The error log is: '{log_text}'. Explain what caused this in 1 sentence and give 2 bullet points on how to fix it.",
                },
            ])
            ai_summary = response['message']['content']
            print(f"\n💡 AI ANALYSIS:\n{ai_summary}\n")
            # ------------------------

    # Save everything (including AI summary) to the DB
    document = {
        "message": log_text,
        "is_anomaly": is_anomaly,
        "ai_explanation": ai_summary if is_anomaly else "Normal",
        "timestamp": time.time()
    }
    es.index(index="logs-index", document=document)