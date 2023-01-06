# VoD service

The VoD service exposes a simple http interface that accepts file uploads
and packages them to a HLS stream in a configured location.
This stream may then be distributed by the TUM-Live/worker/edge module.

Keep in mind: The input file is not re- encoded,
if its codec or format is infeasible for browsers, so will the HLS stream be.

## usage

```shell
docker build -t vod-service .
docker run -p 8089:8089 -v /path/to/vod/packages:/out -e OUTPUT_DIR=/out vod-service

curl -F 'file=@/path/to/Exiting_video.mp4' http://localhost:8089

ls -lah /path/to/vod/packages/Exiting_video.mp4/
> -r--r-- 1 root   root   3.9K Jan  6 19:11 playlist.m3u8
> -r--r-- 1 root   root   4.2M Jan  6 19:11 segment0000.ts
> -r--r-- 1 root   root   3.3M Jan  6 19:11 segment0001.ts
> -r--r-- 1 root   root   4.8M Jan  6 19:11 segment0002.ts
> -r--r-- 1 root   root   3.4M Jan  6 19:11 segment0003.ts
> -r--r-- 1 root   root   4.3M Jan  6 19:11 segment0004.ts
> -r--r-- 1 root   root   4.5M Jan  6 19:11 segment0005.ts
```

## todos

This module is currently just a 1:1 replacement for an old system we want to get rid of. 
The features can be extended to:
- Automatic transcoding (e.g. into 3 different resolutions)
- Handling of irregular videos (non h264, weirdly placed i-frames, etc.)
- Graceful error handling
- Other protocols than HTTP
- Config stuff (e.g. chunk size)
