# Uploading a video

Uploaded videos are converted by a [runner](Architecture.md). Locally you need `tum-live`, a `runner` and the `edge`
that serves the result; `vod-service` and the legacy `worker` are not involved.

## How it works

1. The browser sends the file to `POST /api/v2/courses/{course}/streams/{stream}/media?type=COMB|PRES|CAM` on `tum-live`.
2. `tum-live` reserves a runner over gRPC (`PrepareUpload`). The runner answers with a single-use token and the port of
   its upload endpoint (`UPLOAD_PORT`, `8188` by default).
3. `tum-live` streams the file to `http://<runner>:<port>/upload/<token>`. The upload endpoint has no authentication of
   its own: the token is the credential, so the port must only be reachable by `tum-live`.
4. The runner answers as soon as the file is on disk (`UPLOAD_PATH`) and converts it in the background. It repackages
   the video into HLS without re-encoding when it is already H.264/AAC and transcodes it otherwise.
5. The result is written to `STORAGE_PATH/<stream id>/<version>/`, where the edge serves it, and the runner reports it
   to `tum-live` (`VODReady`, `ThumbnailReady`). Only then does the lecture show the video.

If the conversion fails, the runner only logs it: look at the runner's logs when an uploaded lecture stays empty.

## Starting the services

With docker compose everything is set up, including the volume the runner and the edge share:

```bash
docker compose up --build
```

## Setting up services locally

1. Start the backend and (optional) frontend. Don't forget the DB. See the main docs for more information.
2. Start the runner, with a `STORAGE_PATH` that the edge serves as its `vodPath`:
	```shell
	STORAGE_PATH=/home/<you>/dev/storage/mass \
	SEGMENT_PATH=/home/<you>/dev/storage/live \
	UPLOAD_PATH=/home/<you>/dev/storage/uploads \
	go run ./runner/cmd/runner
	```
3. Start the edge (set `var vodPath = ...` in `edge.go` to the same `STORAGE_PATH`):
	```shell
	go run ./edge-service
	```

`tum-live` reaches the runner by the `REALHOST` it registered with (default `localhost`), so run the runner on the
same machine or set `REALHOST`.

## Uploading the video

1. Open the frontend in your browser at `http://localhost:8081` and navigate to a lecture or create a new one.
2. Choose the Upload option and select continue. You have to give it a name and select the date and time of the lecture.
3. After that, you can select the video file you want to upload and start the uploading process.

:::info
Check the runner's logs as you are uploading the video
:::
