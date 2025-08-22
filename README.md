# PivotProxy

A lightweight custom reverse proxy with built-in load balancing.  

## Overview
PivotProxy forwards client requests to backend application servers while hiding server IPs, improving scalability, and mitigating DDoS risks. It supports multiple load balancing algorithms, including:
- Round Robin  
- Least Connections  
- Weighted Balancing  

## Features
- Single public entrypoint for client traffic  
- Load balancing across 1-N application servers  
- Server health checks & metrics (connections, latency, capacity)  
- Minimal overhead (<15ms added latency)  
- DDoS mitigation by distributing load across servers  
- Fast server state tracking via Redis  

## Limitations
- Reverse proxy itself may become a bottleneck under heavy attack  
- Requires scaling with multiple proxies to avoid overload  

## Getting Started
Earliest start date: **08/25/25**  
Requests are sent to a single public IP and routed to backend servers based on load balancing rules. Metrics can be exposed via an API for monitoring. 

## Related design document
https://docs.google.com/document/d/1dyVmPib7QwCITQSdrs2h5IQmpqHJdFSTw78ACIIFWjE/edit?usp=sharing


