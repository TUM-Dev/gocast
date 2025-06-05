# rtmp-relay

This docker container acts as a rtmp receiver and offers a pullable stream via rtmp.
This Server can be used as a lecture hall in TUM-Live as proxy to devices that only push rtmp streams.

## Usage

edit `rtmp-simple-server.yml` and change the default username and password.

```bash
docker build -t tumlive/rtmp-relay .
docker run -p 1935:1935 -e VALID_PATHS=somestreamname,someotherstream tumlive/rtmp-relay
```

Keep the VALID_PATHS secret, they allow streaming to the relay.

Publishing a stream to `rtmp://localhost:1935/someValidPath` will make a pullable stream under `rtmp://localhost:1935/someValidPath` available.
