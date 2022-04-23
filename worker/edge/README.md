# TUM-Live/worker/edge

The edge module is designed as a simple edge proxy and cache node for TUM-Live/worker.
It can be used when network traffic to worker nodes exceeds the available bandwidth, the architecture might look like this:
```
                                                ┌───────┐ proxy /stream1.m3u8
┌─────────────────┐                         ┌─► │Edge 1 ├──────────────────────────┐  ┌───────────┐
│                 ├─────────────────────────┘   └───────┘                          └─►│           │
│  Load Balancer  │GET /worker-n/stream1.m3u8                                         │ worker-n  │
│(DNS-RR/HTTP 302)│GET /worker-n/media123.ts                                          │           │
│                 ├─────────────────────────┐   ┌───────┐                          ┌─►└───────────┘
└─────────────────┘                         └─► │Edge 2 ├──────────────────────────┘
                                                └───────┘ proxy & cache /media123.ts
```

## Configuration

The following configuration options are available via environment variables:

- `PORT`: The port on which the edge node should listen for incoming connections (default: 8080).
- `ORIGIN_PORT`: The port on which the workers hls files are available (default: 8085). 
- `ORIGIN_PROTO`: The protocol of the origin server (default: http).
