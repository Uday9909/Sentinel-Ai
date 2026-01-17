import urllib.request
import json
import time
import random

API_URL = "http://localhost:8080/ingest"

services = ["auth-service", "payment-api", "inventory-service", "notification-worker"]
actions = [
    ("info", "User login successful"),
    ("info", "Health check passed"),
    ("success", "Transaction completed"),
    ("info", "Cache refreshed"),
    ("info", "Email sent"),
]

def send_log(service, level, message):
    data = {"service": service, "level": level, "message": message}
    try:
        req = urllib.request.Request(API_URL)
        req.add_header('Content-Type', 'application/json')
        jsondata = json.dumps(data).encode('utf-8')
        urllib.request.urlopen(req, jsondata)
        print(f"Sent: [{level.upper()}] {message}")
    except Exception as e:
        print(f"Failed to send: {e}")

print("🎬 Starting LinkedIn Demo Scenario...")
print("Step 1: Normal Traffic (10s)...")
for _ in range(10):
    svc = random.choice(services)
    lvl, msg = random.choice(actions)
    send_log(svc, lvl, msg)
    time.sleep(random.uniform(0.5, 1.5))

print("\n⚠️ Step 2: Signs of Trouble (Warnings)...")
send_log("payment-api", "warn", "High latency detected on DB connection (400ms)")
time.sleep(2)
send_log("payment-api", "warn", "Retrying transaction ID #99281")
time.sleep(2)

print("\n🔥 Step 3: CRITICAL ANOMALY ALERT! (Burst)...")
# Send rapid fire errors to trigger the detector
for i in range(8):
    send_log("payment-api", "error", f"CRITICAL: Database Connection Refused - Connection Pool Empty")
    time.sleep(0.3)

print("\n✅ Scenario Complete. Check Dashboard!")
