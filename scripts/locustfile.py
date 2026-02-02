"""
Locust load testing script for Highload Service
Usage: locust -f locustfile.py --host=http://localhost:8080

Target: 1000+ RPS with latency < 50ms
"""

import json
import random
import time
from locust import HttpUser, task, between, events
from datetime import datetime

class HighloadUser(HttpUser):
    """Simulates IoT device or API client sending metrics"""

    wait_time = between(0.001, 0.01)  # Very short wait for high RPS

    def on_start(self):
        """Initialize device ID on start"""
        self.device_id = f"device-{random.randint(1, 1000)}"
        self.metrics_count = 0

    @task(10)
    def send_metric(self):
        """Send single metric - main load generator"""
        # Generate realistic metric data
        cpu = random.gauss(50, 15)  # Mean 50%, stddev 15%
        cpu = max(0, min(100, cpu))  # Clamp to 0-100

        rps = random.gauss(500, 100)  # Mean 500 RPS, stddev 100
        rps = max(0, rps)

        # Occasionally inject anomalies (5% chance)
        if random.random() < 0.05:
            cpu = random.choice([95, 98, 99, 5, 2, 1])  # Extreme values
            rps = random.choice([1500, 2000, 50, 10])

        metric = {
            "timestamp": datetime.utcnow().isoformat() + "Z",
            "cpu": round(cpu, 2),
            "rps": round(rps, 2),
            "device_id": self.device_id
        }

        with self.client.post(
            "/metrics",
            json=metric,
            catch_response=True
        ) as response:
            if response.status_code == 200:
                self.metrics_count += 1
                response.success()
            else:
                response.failure(f"Status code: {response.status_code}")

    @task(2)
    def send_batch(self):
        """Send batch of metrics"""
        metrics = []
        for _ in range(10):
            cpu = random.gauss(50, 15)
            cpu = max(0, min(100, cpu))
            rps = random.gauss(500, 100)
            rps = max(0, rps)

            metrics.append({
                "timestamp": datetime.utcnow().isoformat() + "Z",
                "cpu": round(cpu, 2),
                "rps": round(rps, 2),
                "device_id": self.device_id
            })

        with self.client.post(
            "/metrics/batch",
            json={"metrics": metrics},
            catch_response=True
        ) as response:
            if response.status_code == 200:
                response.success()
            else:
                response.failure(f"Status code: {response.status_code}")

    @task(1)
    def get_analysis(self):
        """Get current analysis stats"""
        with self.client.get("/analyze", catch_response=True) as response:
            if response.status_code == 200:
                response.success()
            else:
                response.failure(f"Status code: {response.status_code}")

    @task(1)
    def health_check(self):
        """Check service health"""
        with self.client.get("/health", catch_response=True) as response:
            if response.status_code == 200:
                data = response.json()
                if data.get("status") == "healthy":
                    response.success()
                else:
                    response.failure("Service unhealthy")
            else:
                response.failure(f"Status code: {response.status_code}")


class HighRPSUser(HttpUser):
    """High-frequency user for stress testing - targets 1000+ RPS"""

    wait_time = between(0.0001, 0.001)  # Minimal wait

    @task
    def rapid_metrics(self):
        """Rapid fire metrics submission"""
        metric = {
            "timestamp": datetime.utcnow().isoformat() + "Z",
            "cpu": round(random.uniform(20, 80), 2),
            "rps": round(random.uniform(200, 800), 2)
        }

        self.client.post("/metrics", json=metric)


# Event handlers for reporting
@events.test_start.add_listener
def on_test_start(environment, **kwargs):
    print("=" * 60)
    print("Highload Service Load Test Started")
    print("Target: 1000+ RPS, Latency < 50ms")
    print("=" * 60)


@events.test_stop.add_listener
def on_test_stop(environment, **kwargs):
    print("=" * 60)
    print("Load Test Completed")
    print("=" * 60)


@events.request.add_listener
def on_request(request_type, name, response_time, response_length, exception, **kwargs):
    if response_time > 50:  # Log slow requests (> 50ms)
        pass  # Uncomment for debugging: print(f"Slow request: {name} - {response_time}ms")
