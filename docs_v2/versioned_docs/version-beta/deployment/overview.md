---
title: "Overview"
sidebar_position: 1
description: "Deploy GoCast at your organization/school/department."
---

# Deploy GoCast

If you are an admin or maintainer, you can add new resources to one of your administered organizations.

You can use GoCast all while keeping full control over your resources and files by connecting to the GoCast network.
In a nutshell, this involves the following steps and (depending on your technical abilities, system and requirements) should take between 30 minutes and a couple of hours:

1. [Add Workers](./step-by-step/add-worker) or [Runners](./step-by-step/add-runner) (responsible for processing the VoDs and streams)

2. [Add VoD Services](./step-by-step/add-vodservice) (responsible for uploading the files processed by a worker to the shared storage)

3. [Set up Edge Servers](./step-by-step/setup-edge) (proxy to access uploaded files on the shared storage)

4. _Optional:_ Add additional services (such as automatic lecture transcribing, logging, application proxies, etc.)

:::tip
If you plan to host your resources on a Virtual Machine with Docker, we recommend using [Docker Swarm](https://docs.docker.com/engine/swarm/). To quickly setup docker swarm for GoCast, refer to the [Docker Swarm guide](./deploy-with-docker-swarm). Otherwise check out the [step-by-step guide](/docs/beta/category/step-by-step-guide). In both cases, make sure to check the [Prerequisites](./prerequisites) beforehand.
:::

<!-- ## GoCast's Architecture

```
                                          ┌──────────────────────┐
                              ┌───────────►   Campus Management  │
                              │           │ System (CAMPUSOnline)│
               ┌──────────┐   │Enrollments└──────────────────────┘
               │Identity  │   │
               │Management│   │                 - Users,
               │  - SAML  ◄─┐ │                 - Courses,
               │  - LDAP  │ │ │                 - Streams, ...                               ┌────────────────────┐
               └──────────┘ │ │              ┌──────────┐                                    │Lecture Hall        │
                      Users │ │  ┌──────────►│ Database │                                    │ - Streaming Device │
                            │ │  │           └──────────┘                               ┌────┤ - Camera           │
                            │ │  │                                                      │    │ - Slides (HDMI)    │
                         ┌──┴─┴──┴────┐         Task Distribution (gRPC)            RTSP│    │ - Microphone       │
            ┌────────────►  TUM-Live  │◄─────────────────────────────────────┐      pull│    └────────────────────┘
            │Website     └────────────┘                                      │          │
            │(HTTP)                                                          ▼          │
            │                                                             ┌─────────────▼─────┬─┐
            │                               ┌────────────────┐            │TUM-Live Worker #1 │ │ Streaming,
┌───────────┴──┐                            │ Shared Storage │            ├───────────────────┘ │ Converting,
│Student/Viewer│                            │ S3, Ceph, etc. │            │  TUM-Live Worker #n │ Transcribing, ...
└───────────┬──┘                            └─▲────▲─────────┘            └──────┬──▲────▲──────┘
            │                        Serve Vod│    │HLS Files            Push VoD│  │    │RTMP
            │                          Content│    │         ┌───────────┐ (HTTP)│  │    │push
            │                                 │    └─────────┤VoD Service◄───────┘  │    │       ┌──────────────┐
            │Videos      ┌──────────────────┬─┴┐             └───────────┘          │    └───────┤Selfstreamer  │
            │(HLS, HTTP) │ TUM-Live Edge #1 │  │                                    │            │  - OBS,      │
            └────────────►──────────────────┘  ├────────────────────────────────────┘            │  - Zoom, ... │
                         │   TUM-Live Edge #n  │       Proxy, Cache (HTTP)                       └──────────────┘
                         └─────────────────────┘
```

-->

## Multitenancy

Initially, GoCast was used primarily by the former faculty of Informatics at TUM. However, with increasing demand, GoCast needs to be extended for university-wide lecture streaming. The solution to this are [_organizations_](./../features/Organizations).

To start using GoCast for your organization/school/department, you only need to deploy the TUM-Live Worker or Runner, VoD Service and TUM-Live Edge yourself. All other services are already provided by the GoCast network.

:::info
For more information, see the [example deployment diagram](./step-by-step/example-deployment).
:::
