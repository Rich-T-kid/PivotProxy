from random import random
import requests
import numpy as np
import yaml
import random

with open('../config/servers.yaml') as f:
    config = yaml.safe_load(f)


def generate_array():
    return np.random.randint(0, 100_000, 100)
def getURL() -> str:
    return "http://localhost:80/"

def gen_request(IP_address:str):
    l = requests.Request(
    method="POST",
    url=getURL(),
    headers={"IP": IP_address},
    data={"values": generate_array().tolist()},
)
    print(l.url)

def healthcheck():
    print(requests.get(getURL()+"health").status_code)

    
def generate_ip():
    return ".".join(str(random.randint(0, 255)) for _ in range(4))

if __name__ == "__main__":
    healthcheck()
    for i in range(5000):
        print(f"batch {i+1}")
        for j in range(500000):
            resp = requests.post(getURL()+"process", headers={"IP": generate_ip()},json={"values": generate_array().tolist()})
            print(resp.status_code)
    print("sent a total of 250000000 requests")