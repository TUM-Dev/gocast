FROM bluenviron/mediamtx:1.17.1 as mediamtx


FROM alpine:3.23
ADD entrypoint.sh /entrypoint.sh
ADD mediamtx.yml /mediamtx.yml
RUN chmod +x /entrypoint.sh

RUN apk add --no-cache \
  ffmpeg \
  tzdata

COPY --from=mediamtx /mediamtx /mediamtx
RUN chmod +x /mediamtx

CMD ["/entrypoint.sh"]