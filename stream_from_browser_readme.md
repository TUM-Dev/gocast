# Setting Up a Local Environment for Browser Streaming

This guide will assist you in setting up a local environment for browser streaming using Livekit and Livekit Egress.

## Prerequisites

Before we start, ensure your system has the following software installed:

- [livego](https://github.com/gwuhaolin/livego): A powerful live streaming server.
- [livekit-server](https://github.com/livekit/livekit): A scalable, open-source video and audio rooms as a service solution.
- [redis instance](https://hub.docker.com/_/redis): A widely-used, open-source in-memory data structure store.

After installing these tools, initialize them to run locally. They will establish a server prepared to manage incoming transmission requests.

## Configuring Livekit Egress

Next, you'll need to set up and configure [Livekit Egress](https://github.com/livekit/egress). Livekit Egress is designed to address interoperability issues with WebRTC and other services, providing a consistent set of APIs to export your LiveKit sessions and tracks. This process is handled automatically irrespective of the method used, thanks to GStreamer, which transcodes streams when moving between protocols, containers, or encodings.

Here are the steps to run Livekit Egress against a local Livekit server:

1. Open `/usr/local/etc/redis.conf` and comment out the line that says `bind 127.0.0.1`.
2. Change `protected-mode yes` to `protected-mode no` in the same file.
3. Determine your IP as seen by Docker. This IP will be used to set the `ws_url`.

Create a new directory to mount. For instance, `~/egress-test`.

Next, create a `config.yaml` file in the directory you've just created with the following information:

```yaml
log_level: debug
api_key: devkey
api_secret: secret
ws_url: ws://<Your Docker IP>:7880
insecure: true
redis:
  address: <Your Docker IP>:6379
```
Replace `<Your Docker IP>` with your Docker IP address.

Now, run the egress service with the following command:

```bash
docker run --rm \
    -e EGRESS_CONFIG_FILE=/out/config.yaml \
    -v ~/egress-test:/out \
    livekit/egress
```

At this point, your Livekit Egress is ready and will automatically transcode streams based on your requirements.

## Starting Services

With everything set up, you can now start the services:

- Begin with starting Redis
- Start the livekit server:

```bash
livekit-server --dev --redis-host 127.0.0.1:6379
```

- Lastly, start the livekit egress server:

```bash
docker run --rm \
    -e EGRESS_CONFIG_FILE=/out/config.yaml \
    -v ~/egress-test:/out \
    livekit/egress
```

You have now successfully established a local environment for browser streaming using Livekit and Livekit Egress. These servers are ready to be incorporated into your larger system.