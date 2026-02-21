---
title: "Self Streaming"
sidebar_position: 4
description: "Information on how to stream from your own computer."
---

This guide contains information on how to stream from your own computer.

## Gather streaming key.

- Open the course's admin page
- Create a stream with the location "self stream" if it doesn't exist already.
- Navigate to the stream and click `show keys`.

## OBS

:::warning
Self streaming can be unreliable and we cannot guarantee proper recording of your lecture just yet. In some cases, recordings might be corrupted. **Please save a local copy just in case using the streaming software of your choice.**
:::

- Download and install OBS from [here](https://obsproject.com/).
- Open OBS and click on `Settings` in the bottom right corner.
- Click on `Stream` in the left sidebar.
- Select `Custom` from the dropdown menu.
- Paste the stream key and the stream server from the course's admin page into the `Stream key` and `Stream server` field.
- Click on `Output` in the left sidebar.
- Click on `Streaming` in the top menu.
- Select `Simple` from the `Output Mode` dropdown menu.
- Insert the following settings:
  - Video Bitrate: 2500 – 4000
  - Audio Bitrate: 192 kbp/s (or 128kbp/s)
  - Video Encoder: x264
- Please ensure that your output is scaled to 1920x1080.
