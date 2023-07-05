---
title: "Networking"
draft: false
weight: 30
---


## Networking

The following ports need to be exposed to the public:

| Server (label)                   | Port            | 
|----------------------------------|-----------------|
| GoCast Server (tumlive, traefik) | 80 TCP, 443 TCP |
| Worker (worker)                  | 1935 TCP        |
| Edge (edge)                      | 80 TCP, 443 TCP |

Between the individual servers, communication should not be firewalled. Auditorium hardware should also be in the same vlan.
