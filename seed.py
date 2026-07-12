import requests
import json
import random
import time
from datetime import datetime, timedelta

API_URL = "http://localhost:8080/api/v1"
EMAIL = "admin@example.com"
PASSWORD = "password123"

# 1. Login
res = requests.post(f"{API_URL}/auth/login", json={"email": EMAIL, "password": PASSWORD})
if res.status_code != 200:
    print("Login failed", res.text)
    exit(1)
token = res.json()["data"]["access_token"]
headers = {"Authorization": f"Bearer {token}"}

# 2. Create Organization
res = requests.post(f"{API_URL}/orgs", headers=headers, json={
    "name": "Acme Corp",
    "slug": "acme-corp",
    "description": "Mock organization for testing"
})
org_id = res.json()["data"]["id"]
print(f"Created Org: {org_id}")

# 3. Create Project
res = requests.post(f"{API_URL}/orgs/{org_id}/projects", headers=headers, json={
    "name": "Data Processing pipeline",
    "slug": "data-pipeline",
    "description": "Main pipeline for data processing"
})
project_id = res.json()["data"]["id"]
print(f"Created Project: {project_id}")

# 4. Create Queues
res = requests.post(f"{API_URL}/projects/{project_id}/queues", headers=headers, json={
    "name": "high-priority",
    "description": "High priority tasks",
    "priority": 1,
    "concurrency": 10,
    "max_retries": 3,
    "retry_strategy": "exponential",
    "retry_delay_sec": 5,
    "visibility_timeout_sec": 60,
    "job_timeout_sec": 300,
})
queue_high = res.json()["data"]["id"]
print(f"Created Queue high-priority: {queue_high}")

res = requests.post(f"{API_URL}/projects/{project_id}/queues", headers=headers, json={
    "name": "default",
    "description": "Default tasks",
    "priority": 5,
    "concurrency": 20,
    "max_retries": 1,
    "retry_strategy": "linear",
    "retry_delay_sec": 10,
    "visibility_timeout_sec": 30,
    "job_timeout_sec": 120,
})
queue_default = res.json()["data"]["id"]
print(f"Created Queue default: {queue_default}")

# 5. Enqueue Jobs
print("Enqueueing jobs...")
for i in range(10):
    run_at = None
    if i % 3 == 0:
        run_at = (datetime.utcnow() + timedelta(minutes=5)).isoformat() + "Z"
    
    requests.post(f"{API_URL}/projects/{project_id}/queues/{queue_high}/jobs", headers=headers, json={
        "type": "send_email",
        "payload": {"user_id": i, "template": "welcome"},
        "run_at": run_at,
        "tags": ["email", "welcome"]
    })

for i in range(25):
    requests.post(f"{API_URL}/projects/{project_id}/queues/{queue_default}/jobs", headers=headers, json={
        "type": "process_image",
        "payload": {"image_id": 100+i, "filter": "grayscale"},
        "tags": ["image_processing"]
    })

print("Database successfully seeded with mock data!")
