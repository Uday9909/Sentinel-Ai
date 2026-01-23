import urllib.request
import json
import time
import uuid
import random

API_URL = "http://localhost:8080/ingest"
SERVICES = ["payment-gateway", "auth-service", "user-profile"]
HOSTS = ["prod-worker-1", "prod-worker-2", "prod-worker-3"]

def send_complex_log(i):
    service = random.choice(SERVICES)
    host = random.choice(HOSTS)
    trace_id = str(uuid.uuid4())
    
    data = {
        "service": service,
        "level": "info",
        "message": f"Phase 1 Verification Log {i}",
        "trace_id": trace_id,
        "host": host,
        "timestamp": int(time.time()),
        "labels": {"env": "prod", "region": "us-east-1"}
    }
    
    req = urllib.request.Request(API_URL)
    req.add_header('Content-Type', 'application/json')
    jsondata = json.dumps(data).encode('utf-8')
    
    try:
        urllib.request.urlopen(req, jsondata)
        print(f"✅ Sent log from {service} on {host} (Trace: {trace_id})")
    except Exception as e:
        print(f"❌ Failed to send log: {e}")

print("🚀 Starting Phase 1 Verification...")
print("Sending 10 rich metadata logs...")

for i in range(10):
    send_complex_log(i)
    time.sleep(0.5)

print("\n✨ Done. Check Processor output for TraceID interpretation.")
