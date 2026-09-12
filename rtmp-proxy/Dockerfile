FROM alpine:3.22

RUN apk add --no-cache nginx nginx-mod-rtmp bash curl ffmpeg jq

# Copy preconfigured nginx.conf and publish.sh into the container
COPY nginx.conf /etc/nginx/nginx.conf
COPY publish.sh /etc/nginx/scripts/publish.sh

# Make publish.sh executable
RUN chmod +x /etc/nginx/scripts/publish.sh

EXPOSE 1935

CMD ["nginx", "-g", "daemon off;"]
