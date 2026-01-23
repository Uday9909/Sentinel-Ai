import urllib.request
import json

ES_URL = "http://localhost:9200/logs-index/_search?size=1&sort=timestamp_processed:desc"

try:
    req = urllib.request.Request(ES_URL)
    with urllib.request.urlopen(req) as response:
        if response.status == 200:
            data = json.loads(response.read().decode())
            hits = data.get("hits", {}).get("hits", [])
            if hits:
                doc = hits[0]["_source"]
                print("✅ Latest Log Found:")
                print(json.dumps(doc, indent=2))
                
                if "trace_id" in doc and "host" in doc:
                    print("\n🎉 SUCCESS: TraceID and Host fields are present!")
                else:
                    print("\n❌ FAILURE: Missing metadata fields.")
            else:
                print("⚠️ No logs found in index.")
        else:
            print(f"❌ ES Error: {response.status}")
except Exception as e:
    print(f"❌ Failed to query ES: {e}")
