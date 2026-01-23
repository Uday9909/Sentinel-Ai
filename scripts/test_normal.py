import urllib.request
import json
import time

API_URL = "http://localhost:8080/ingest"

def send_log(message):
    data = {"service": "test-service", "level": "info", "message": message}
    req = urllib.request.Request(API_URL)
    req.add_header('Content-Type', 'application/json')
    jsondata = json.dumps(data).encode('utf-8')
    urllib.request.urlopen(req, jsondata)
    print(f"Sent: {message}")

print("Sending 20 normal logs...")
for i in range(20):
    send_log(f"VerifyNormalUnique-{time.time()}-{i}")
    time.sleep(0.1)

print("Done. Check processor output for silence.")
