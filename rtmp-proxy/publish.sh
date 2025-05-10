#!/bin/bash

LOG_FILE="/tmp/rtmp_exec.log" # TODO: change if necessary

# Extract the stream key (first argument)
STREAM_KEY="$1"

# echo "Received stream key: $STREAM_KEY" >> $LOG_FILE 2>&1 # TODO: for debugging only

# Exchange the stream key for a stream-specific URL
API_URL="https://live.rbg.tum.de/api/token/proxy/$STREAM_KEY"
RELAY_URL=$(curl -s -X POST "$API_URL" | jq -r '.url')

if [ -z "$RELAY_URL" ]; then
    echo "No relay URL found for stream key" >> $LOG_FILE 2>&1
    exit 1
fi
echo "Relay URL found: [$RELAY_URL]" >> $LOG_FILE 2>&1

# Push the stream to the stream specific RTMP URL using ffmpeg
ffmpeg -re -i rtmp://localhost:1935/live/$STREAM_KEY -c copy -f flv "$RELAY_URL" >> $LOG_FILE 3>&1