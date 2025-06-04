FROM tiangolo/nginx-rtmp

# Install jq and ffmpeg
RUN apt-get update && apt-get install -y jq ffmpeg

# Copy preconfigured nginx.conf and publish.sh into the container
COPY nginx.conf /etc/nginx/nginx.conf
COPY publish.sh /etc/nginx/scripts/publish.sh

# Make publish.sh executable
RUN chmod +x /etc/nginx/scripts/publish.sh