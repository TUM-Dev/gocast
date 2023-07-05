---
title: "Prerequisites"
draft: false
weight: 10
---

## Prerequisites

We deploy GoCast on our own hardware in VMs. Any cloud hosting provider works just as well.
You will need the following hardware configuration:

- 1 VM for the GoCast server and database. This can be a small VM if you are not expecting a lot of users.
- At least 1 VM as an edge server. This server serves the videos to the users. Network throughput is important here. If you serve lots of users, you can spin up more of these.
- At least 1 Worker VM. This server produces the stream, transcodes the vod and much more. CPU performance is important here. As you start streaming more, you can spin up more of these.
- Optional: At least 1 NVIDIA CUDA equipped Server that transcribes streams using the Whisper LLM. 
- Optional: 1 VM for monitoring (grafana, prometheus, influx...). This can be a small VM as well.

## Storage

GoCast produces lots of large files. They'll need to be accessed by all workers and edge servers. 
Thus, you'll need a shared storage solution. We use [Ceph](https://www.ceph.com/en/). 
The reliability and performance of the storage solution is critical for the performance of GoCast, setting it up and running it is not trivial.
Operating a network storage solution is out of scope for this documentation.

For this documentation, we assume that you have some sort of high performance shared filesystem mounted to the same directory on all your servers.
