# Uploading a video

Uploaded videos are converted by a [runner](Architecture.md). Locally you need `tum-live`, a `runner` and the `edge`
that serves the result; `vod-service` is not involved.

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

## What the lecturer sees

`tum-live` records the conversion in `transcoding_progresses` as soon as a runner has taken the upload, so the lecture
card shows it as converting right away:

- While ffmpeg runs, the runner sends `TranscodingProgress` (at most once a second, in steps of 2 percent). These are best
  effort and never retried: a lost one is replaced by the next, and one delivered late could land after the notification
  that ended the conversion. The server therefore only ever updates a row that exists and has not failed.
- `VODReady` removes the row.
- If the import fails, the runner sends `VODFailed` with a reason meant for the uploader (it names no paths; the details
  are in the runner's log). The row is kept, marked failed, and the card shows the reason until the next upload of that
  version replaces it. Unlike progress, `VODFailed` is retried until it is delivered.

Not covered:

- If the runner itself dies mid-conversion, nothing reports it and the card keeps showing the last percentage.
  Uploading the version again replaces it.
- Notifications carry no upload ID. `VODFailed` and `VODReady` are retried until delivered, so if the server was down
  while a version was uploaded a second time, the first upload's late notification can land on the second one's row
  (marking it failed, or removing it while it still converts). It takes a server outage plus a re-upload of the same
  version within the retry window; fixing it needs an ID on the row and in the notifications.
- If `tum-live` is asked to replace an upload and the runner then refuses the new file, the version's row is put back as
  it was, so a conversion still running or a failure the lecturer has not seen yet is not lost.

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
