# source venv/bin/activate
# to activate the virtual environment
import yaml
import requests
import time

with open('../config/servers.yaml') as f:
    config = yaml.safe_load(f)

port = int(config.get("Proxy_port", config.get("Proxy_port", 79)))
host = config.get("Proxy_ip", "127.0.0.1")

target_url = f"http://{host}:{port}/"
print(f"Target URL: {target_url}")
data = {"title": "foo", "body": "bar", "userId": 1}
headers = {"Content-Type": "application/json"}

for i in range(10000):
    try:
        resp = requests.post(target_url, json=data, headers=headers, timeout=5)
        print(f"Response {i}: {resp.status_code} - {resp.text[:200]}")
    except requests.RequestException as e:
        print(f"Request {i} failed: {e}")
