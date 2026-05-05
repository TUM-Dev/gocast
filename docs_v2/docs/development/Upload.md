# Uploading a video

To upload a video, you can use the `docker-compose-selfstreawm.yml` file to start all the services required for the upload process.

## Starting the services

1. in vod-service.go change the port to 8080: err := http.ListenAndServe(":8080", nil)
2. Run the following command to start the services:

```bash
docker compose -f docker-compose-selfstream.yml up --build
```

## Uploading the video

1. Open the frontend in your browser at `http://localhost:8081` and navigate to a lecture or create a new one.
2. Choose the Upload option and select continue. You have to give it a name and select the date and time of the lecture.
3. After that, you can select the video file you want to upload and start the uploading process. 

:::info
Check the logs as you are uploading the video
:::
