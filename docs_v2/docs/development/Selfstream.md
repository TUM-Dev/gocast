---
title: Selfstream
---

# Selfstream

We first have to start all services, and then we can set up the stream.

There are two ways to run Selfstream-services:

## Setting up the services:

### 1. **Using Docker**: 

   This is the recommended way to run Selfstream, as it provides an isolated environment and simplifies the setup process. To run Selfstream using Docker, follow these steps:

   ```bash
   docker compose -f docker-compose-selfstream.yml up --build
   ```

Now you can follow the instructions in the [Starting the stream](#starting-the-stream) section to start your stream.

---

### 2. **Starting services locally**:

:::warning
Warning: This method doesn't insure that the services are running with the correct configuration, so it is recommended to see the logs after each service is started.
:::

If you prefer to run the services locally, you can start each service individually. This method requires more setup and configuration, but it allows for more flexibility in development. To start the services locally, follow these steps:

:::info
You have to change the `externalAuthenticationURL` in the `ingest/mediamtx.yml` file by uncommenting the the line and changing the URL to `http://localhost:8081/api/selfstream/onPublish`. This is required for the `mediamtx` server to authenticate the stream correctly.
:::

   - Start the db and meilisearch first. We use hybrid approach to run db and meilisearch in docker. You can run the following command to start them:
       ```bash
       docker start meilisearch mariadb-tumlive
       ```
        :::info 
        If you don't have them set up, please follow the instructions in the [DevSetup](./DevSetup.md#setup-database) guide to set them up.
        ::: 

   - Start the backend. run this command in the `root`

     ```bash
     go run ./cmd/tumlive
     ```

- Start the frontend:

  ```bash
  # in the web/ directory
  
  npm install
  npm run build-dev
  ```

- Start the runner. Configure the path accordingly: You can set the `STORAGE_PATH` and `SEGMENT_PATH` environment variables to point to the correct locations if you don't want to use default ones:

  ```bash
  # in the root
     
  STORAGE_PATH=/home/<path-to-cache-location>/storage/mass SEGMENT_PATH=/home/<path-to-cache-location>/dev/storage/live go run runner/cmd/runner/main.go
  ```

- Start the worker:

  ```bash
  # in the root
     
  go run ./worker/edge
  ```
- Start the `mediamtx` server:

   ```bash
   # in the root
    
   mediamtx ./ingest/mediamtx.yml
   ```

---

## Starting the stream

After setting up the services you can follow these instructions to start your stream. We recommend using OBS as it has been tested and also used to showcase here:

1. Login to an admin account (`admin`, `prof1` or `prof2`).
    
    In the admin section confirm that the runner is `active` 

2. In the admin section you can either create a new course or use one of the existing ones to create a new lecture. Just fill the required fields and click the create button.

3. After creating the lecture you can click the `show keys` button to see the `URL` and `key`. Copy them into the streaming software of your choice.
4. Change the `tum.ingest.live/` to `localhost/`.
5. Start streaming, and you should see the stream in the admin section after a few seconds. You can also check the `meidamtx` logs to see if the stream is being ingested correctly.






