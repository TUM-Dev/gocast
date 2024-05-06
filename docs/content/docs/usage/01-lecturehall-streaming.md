---
title: "Lecture Hall Streaming"
draft: false
weight: 10
---

## Streaming from a lecture hall with installed SMPs

This guide contains information on how to stream from a lecture hall at TUM.


### Gather streaming key.

- Open the courses admin page
- Create a stream with the location "self stream" if it doesn't exist already.
- Navigate to the stream and click `show keys`.

### OBS

- Download and install OBS from [here](https://obsproject.com/).
- Open OBS and click on `Settings` in the bottom right corner.
- Click on `Stream` in the left sidebar.
- Select `Custom` from the dropdown menu.
- Paste the stream key and the stream server from the courses admin page into the `Stream key` and `Stream server` field.
- Click on `Output` in the left sidebar.
- Click on `Streaming` in the top menu.
- Select `Simple` from the `Output Mode` dropdown menu.
- Insert the following settings:
    - Video Bitrate: 2500 - 4000
    - Audio Bitrate: 192 kbp/s (or 128kbp/s)
    - Video Encoder: x264
- Please ensure that your output is scaled to 1920x1080.

#### Zoom

To use zoom for streaming, login to your account, navigate to... Click...