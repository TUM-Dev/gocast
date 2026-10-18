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

## When does the recording get created?

The recording (VoD) is created once the stream is over. For a self stream, *you* decide when that is, not the schedule:

- **Going over the scheduled end is fine.** The stream is not cut off at the scheduled end time as long as you keep streaming.
- **Connection drops and breaks are fine.** If your connection drops, or you stop streaming for a coffee break, the stream stays open. When you start streaming again with the same settings, it continues in the same recording. You don't need to do anything else.
- **The recording is created 15 minutes after you stop streaming**, but never before the scheduled end of the stream. So if you stop early, the recording still waits for the scheduled end. If you stop after the scheduled end, you get it about 15 minutes later. The recording is available a little while after that, depending on its length.
- **You can only reconnect until 30 minutes after the scheduled end.** After that, new connections are rejected, so don't plan breaks that run past it.
- **You can end the stream right away** with `End stream` on the stream's page in the course admin area. The recording is then created from what was captured so far. You can also choose to discard the recording there.

:::tip
Before a break, there is no need to end the stream. Just stop streaming in OBS and start again afterwards. Only end the stream manually if you want the recording now.
:::

:::note
Stopping OBS does not end the stream. If you are done and don't end it manually, the recording is created after the delay described above.
:::
