import urllib.request
import json
import time
import random
import threading
from concurrent.futures import ThreadPoolExecutor
import sys

API_URL = "http://localhost:8080/ingest"

# Expanded services for more realistic enterprise scenario
services = [
    "auth-service", "payment-api", "inventory-service", "notification-worker",
    "user-profile-service", "recommendation-engine", "search-service", 
    "analytics-service", "file-upload-service", "email-service",
    "cart-service", "order-processing", "shipping-service", "review-service",
    "fraud-detection", "loyalty-service", "content-management", "cdn-service"
]

# More diverse log patterns
normal_patterns = [
    ("info", "User login successful"),
    ("info", "Health check passed"),
    ("success", "Transaction completed successfully"),
    ("info", "Cache refreshed"),
    ("info", "Email sent successfully"),
    ("info", "File uploaded successfully"),
    ("info", "Search query processed"),
    ("info", "Recommendation generated"),
    ("info", "User profile updated"),
    ("info", "Order placed successfully"),
    ("info", "Payment processed"),
    ("info", "Inventory updated"),
    ("info", "Notification sent"),
    ("info", "Session created"),
    ("info", "API request completed"),
    ("debug", "Database connection established"),
    ("debug", "Cache hit for user data"),
    ("debug", "Background job completed"),
]

warning_patterns = [
    ("warn", "High response time detected (>500ms)"),
    ("warn", "Memory usage above 80%"),
    ("warn", "Retry attempt for failed request"),
    ("warn", "Rate limit approaching for user"),
    ("warn", "Database connection pool 70% full"),
    ("warn", "Disk space below 20%"),
    ("warn", "High CPU usage detected"),
    ("warn", "Slow query detected (>2s)"),
    ("warn", "Cache miss rate above normal"),
    ("warn", "External API latency high"),
]

error_patterns = [
    ("error", "Database connection failed"),
    ("error", "Payment gateway timeout"),
    ("error", "Authentication failed"),
    ("error", "File upload failed - disk full"),
    ("error", "External API returned 500"),
    ("error", "Memory allocation failed"),
    ("error", "Network timeout occurred"),
    ("error", "Invalid user input received"),
    ("error", "Service unavailable"),
    ("error", "Configuration error detected"),
]

critical_patterns = [
    ("error", "CRITICAL: Database Connection Pool Exhausted"),
    ("error", "CRITICAL: Payment Service Unresponsive"),
    ("error", "CRITICAL: Authentication Service Down"),
    ("error", "CRITICAL: Memory Leak Detected - OOM Imminent"),
    ("error", "CRITICAL: Disk Space Critical - 95% Full"),
    ("error", "CRITICAL: Network Partition Detected"),
    ("error", "CRITICAL: Security Breach Attempt Detected"),
    ("error", "CRITICAL: Data Corruption in Primary Database"),
]

def send_log(service, level, message, thread_id=0):
    """Send a single log message"""
    data = {"service": service, "level": level, "message": message}
    try:
        req = urllib.request.Request(API_URL)
        req.add_header('Content-Type', 'application/json')
        jsondata = json.dumps(data).encode('utf-8')
        response = urllib.request.urlopen(req, jsondata, timeout=5)
        if thread_id == 0:  # Only print from main thread to avoid spam
            print(f"✓ [{level.upper()}] {service}: {message}")
        return True
    except Exception as e:
        if thread_id == 0:
            print(f"✗ Failed to send log: {e}")
        return False

def generate_normal_traffic(duration_seconds, logs_per_second=10, thread_id=0):
    """Generate normal traffic for specified duration"""
    total_logs = duration_seconds * logs_per_second
    interval = 1.0 / logs_per_second
    
    for i in range(total_logs):
        service = random.choice(services)
        level, message = random.choice(normal_patterns)
        send_log(service, level, message, thread_id)
        time.sleep(interval)

def generate_warning_burst(count=20, thread_id=0):
    """Generate a burst of warning messages"""
    for i in range(count):
        service = random.choice(services[:8])  # Focus on core services
        level, message = random.choice(warning_patterns)
        send_log(service, level, message, thread_id)
        time.sleep(random.uniform(0.1, 0.3))

def generate_error_burst(count=30, thread_id=0):
    """Generate a burst of error messages"""
    for i in range(count):
        service = random.choice(services[:5])  # Focus on critical services
        level, message = random.choice(error_patterns)
        send_log(service, level, message, thread_id)
        time.sleep(random.uniform(0.05, 0.2))

def generate_critical_incident(service_name, error_message, count=15, thread_id=0):
    """Generate a critical incident pattern"""
    for i in range(count):
        send_log(service_name, "error", f"CRITICAL: {error_message}", thread_id)
        time.sleep(random.uniform(0.1, 0.5))

def background_traffic_generator(stop_event):
    """Generate continuous background traffic"""
    thread_id = threading.current_thread().ident
    while not stop_event.is_set():
        service = random.choice(services)
        level, message = random.choice(normal_patterns)
        send_log(service, level, message, thread_id)
        time.sleep(random.uniform(0.5, 2.0))

def run_intensive_demo():
    """Run the intensive demo scenario"""
    print("🚀 INTENSIVE SENTINEL-AI DEMO STARTING...")
    print("=" * 60)
    
    # Start background traffic
    stop_background = threading.Event()
    background_threads = []
    
    print("🌊 Starting background traffic generators...")
    for i in range(3):  # 3 background threads
        thread = threading.Thread(target=background_traffic_generator, args=(stop_background,))
        thread.daemon = True
        thread.start()
        background_threads.append(thread)
    
    time.sleep(2)
    
    try:
        # Phase 1: Heavy Normal Traffic
        print("\n📈 PHASE 1: High-Volume Normal Operations (30 seconds)")
        print("Simulating peak business hours with 50+ logs/second...")
        
        with ThreadPoolExecutor(max_workers=5) as executor:
            futures = []
            for i in range(5):
                future = executor.submit(generate_normal_traffic, 30, 10, i+1)
                futures.append(future)
            
            # Wait for all threads to complete
            for future in futures:
                future.result()
        
        print("✅ Normal traffic phase completed")
        
        # Phase 2: Warning Signals
        print("\n⚠️  PHASE 2: System Stress Indicators (15 seconds)")
        print("Multiple services showing warning signs...")
        
        with ThreadPoolExecutor(max_workers=3) as executor:
            futures = [
                executor.submit(generate_warning_burst, 25, 1),
                executor.submit(generate_warning_burst, 20, 2),
                executor.submit(generate_normal_traffic, 15, 5, 3)
            ]
            for future in futures:
                future.result()
        
        print("⚠️  Warning phase completed")
        
        # Phase 3: Error Cascade
        print("\n🔥 PHASE 3: Error Cascade Beginning (10 seconds)")
        print("Multiple systems starting to fail...")
        
        with ThreadPoolExecutor(max_workers=4) as executor:
            futures = [
                executor.submit(generate_error_burst, 40, 1),
                executor.submit(generate_warning_burst, 15, 2),
                executor.submit(generate_normal_traffic, 10, 3, 3),
                executor.submit(generate_error_burst, 25, 4)
            ]
            for future in futures:
                future.result()
        
        print("🔥 Error cascade phase completed")
        
        # Phase 4: Critical Incident
        print("\n🚨 PHASE 4: CRITICAL INCIDENT - MULTIPLE SYSTEM FAILURE!")
        print("Simulating major outage scenario...")
        
        # Simulate database cluster failure
        print("💥 Database cluster failure...")
        with ThreadPoolExecutor(max_workers=3) as executor:
            futures = [
                executor.submit(generate_critical_incident, "payment-api", 
                              "Database Connection Pool Exhausted - All connections in use", 20, 1),
                executor.submit(generate_critical_incident, "auth-service", 
                              "Primary Database Unreachable - Failover Failed", 15, 2),
                executor.submit(generate_critical_incident, "order-processing", 
                              "Transaction Rollback Failed - Data Inconsistency", 12, 3)
            ]
            for future in futures:
                future.result()
        
        time.sleep(2)
        
        # Simulate cascading failures
        print("⛓️  Cascading failures across services...")
        with ThreadPoolExecutor(max_workers=4) as executor:
            futures = [
                executor.submit(generate_critical_incident, "payment-api", 
                              "Circuit Breaker OPEN - Service Degraded", 25, 1),
                executor.submit(generate_critical_incident, "inventory-service", 
                              "Redis Cluster Down - Cache Miss Storm", 20, 2),
                executor.submit(generate_critical_incident, "notification-worker", 
                              "Message Queue Full - Dropping Messages", 18, 3),
                executor.submit(generate_error_burst, 50, 4)
            ]
            for future in futures:
                future.result()
        
        print("\n🚨 CRITICAL INCIDENT SIMULATION COMPLETE!")
        print("=" * 60)
        print("📊 DEMO STATISTICS:")
        print("• Total Duration: ~90 seconds")
        print("• Estimated Logs Generated: 2000+ messages")
        print("• Peak Rate: 50+ logs/second")
        print("• Services Involved: 18 microservices")
        print("• Incident Types: 4 escalation phases")
        print("\n🎯 CHECK YOUR DASHBOARD NOW!")
        print("• Look for anomaly detection alerts")
        print("• Check AI root cause analysis")
        print("• Monitor real-time log feed")
        print("• Observe system performance metrics")
        
    except KeyboardInterrupt:
        print("\n⏹️  Demo interrupted by user")
    except Exception as e:
        print(f"\n❌ Demo error: {e}")
    finally:
        # Stop background traffic
        print("\n🛑 Stopping background traffic...")
        stop_background.set()
        time.sleep(2)
        print("✅ Demo completed successfully!")

def run_stress_test():
    """Run a stress test to see maximum throughput"""
    print("🔥 STRESS TEST MODE - MAXIMUM THROUGHPUT")
    print("=" * 50)
    
    duration = 60  # 1 minute stress test
    threads = 10
    logs_per_thread_per_second = 10
    
    print(f"⚡ Generating {threads * logs_per_thread_per_second} logs/second for {duration} seconds")
    print(f"📊 Total expected logs: {threads * logs_per_thread_per_second * duration}")
    
    def stress_worker(worker_id, stop_event):
        count = 0
        while not stop_event.is_set():
            service = random.choice(services)
            level, message = random.choice(normal_patterns + warning_patterns + error_patterns)
            if send_log(service, level, message, worker_id):
                count += 1
            time.sleep(0.1)  # 10 logs per second per thread
        print(f"Worker {worker_id} sent {count} logs")
    
    stop_event = threading.Event()
    threads_list = []
    
    # Start stress test
    start_time = time.time()
    for i in range(threads):
        thread = threading.Thread(target=stress_worker, args=(i, stop_event))
        thread.start()
        threads_list.append(thread)
    
    # Run for specified duration
    time.sleep(duration)
    stop_event.set()
    
    # Wait for all threads to complete
    for thread in threads_list:
        thread.join()
    
    end_time = time.time()
    actual_duration = end_time - start_time
    
    print(f"\n✅ Stress test completed in {actual_duration:.1f} seconds")
    print("🎯 Check system performance and anomaly detection!")

if __name__ == "__main__":
    if len(sys.argv) > 1 and sys.argv[1] == "stress":
        run_stress_test()
    else:
        run_intensive_demo()