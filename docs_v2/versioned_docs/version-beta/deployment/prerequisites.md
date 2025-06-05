---
title: "Prerequisites"
sidebar_position: 2
description: "What you need before you can start deploying GoCast."
---

## Hardware

Make sure to check the following system requirements before deploying GoCast:

We deploy GoCast on our own hardware in VMs. Any cloud hosting provider works just as well.
You will need the following hardware configuration:

1. Hardware:

   - **At least 1 VM as an Edge server**. This server serves the videos to the users. Network throughput is important here. If you serve lots of users, you can spin up more of these.
   - **At least 1 Worker or Runner VM**. This server produces the stream, transcodes the VoD and much more. CPU performance is important here. As you start streaming more, you can spin up more of these.
   - _Optional_: At least 1 NVIDIA CUDA equipped Server that transcribes streams using the Whisper LLM.

2. **Verify that you're authorized to create new resources** for an organization by going to the ["organizations"-tab](https://live.rbg.tum.de/admins/organizations) in the admin dashboard and checking that you're a maintainer of an organization. If not, contact your organization's IT team or the RBG.

3. **Fetch a new organization token** by clicking on the key-icon of the relevant organization. The token expires after 7 hours.
   :::danger
   DO NOT PUBLISH THIS TOKEN AS IT CAN BE USED TO ADD AND REMOVE RESOURCES UNTIL EXPIRATION!!!
   :::

## Networking

The following ports need to be exposed to the public:

| Service       | Node Label     | Port                          | Required |
| ------------- | -------------- | ----------------------------- | -------- |
| Runner        | `runner`       | 50057 TCP, 8187 TCP           | ✅       |
| Worker        | `worker`       | 1935 TCP, 8060 TCP, 50051 TCP | ✅       |
| VoD Service   | `worker`       | 8089 TCP                      | ✅       |
| Edge          | `edge`         | 80 TCP, 443 TCP               | ✅       |
| Voice-Service | `voiceservice` | 50055 TCP                     | ❌       |

Between the individual servers, communication should not be firewalled. Auditorium hardware should also be in the same VLAN.

## Storage

GoCast produces lots of large files. They'll need to be accessed by all Workers and Edge servers.
Thus, you'll need a shared storage solution. We use [Ceph](https://www.ceph.com/en/).
The reliability and performance of the storage solution is critical for the performance of GoCast, setting it up and running it is not trivial.
Operating a network storage solution is out of scope for this documentation.

:::important
Make sure that all machines have access to a shared storage where the recordings for your lectures will be stored:

- In the following, `/path/to/vod` will need to be shared across your Edge Server and VoD Service deployments. This is where the lecture streams will be uploaded by the Worker via the VoD Service.

- The following paths need to be accessible by all your Worker deployments:
  ```sh
  '/path/to/mass' # this will store all lectures
  '/path/to/workerlog' # this will store Worker logs
  '/path/to/persist' # system related storage
  '/path/to/recordings' # this will store past live stream recordings
  ```
  :::

For this documentation, we assume that you have some sort of high performance shared filesystem mounted to the same directory on all your servers.

:::tip
If you don't have a shared storage solution and just want to try using GoCast with small amounts of data and user, check out [Network File System (NFS)](https://ubuntu.com/server/docs/network-file-system-nfs).
:::

## Notes

For the examples in the next pages, replace `<your-edge-server-addr>` with your Edge server's IP or FQDN (assuming that you have created a DNS A record).

> e.g., 'https://edge.your-organization-domain.com'

Replace `<your-vod-server-addr>` with your Edge server's IP or FQDN (Assuming that you have created an according DNS A record).

> e.g., 'https://vod.your-organization-domain.com' or, if you deploy everything on one machine: 'https://edge.your-organization-domain.com'
> :::note
> Note: the Edge server will be reachable on port `8090` and the VoD server on port `8089`, so you can technically have everything running on one machine. However, to avoid complete outage in case of failure, we recommend you to have multiple dedicated machines (e.g., one for the Edge server and one for the VoD Service + Worker)
> :::
