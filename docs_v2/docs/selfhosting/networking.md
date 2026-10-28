---
title: "Networking"
sidebar_position: 3
---


## Networking

The following ports need to be exposed to the public:

| Server (label)                   | Port            | 
|----------------------------------|-----------------|
| GoCast Server (tumlive, traefik) | 80 TCP, 443 TCP |
| Edge (edge)                      | 80 TCP, 443 TCP |

Runners need no public ports: the GoCast server and the edge servers reach them on 52735 TCP (HLS) and 8188 TCP (uploads), and they reach the GoCast server on 50056 TCP.
Between the individual servers, communication should not be firewalled. Auditorium hardware should also be in the same VLAN.
