#!/usr/bin/env bash
set -e

cleanup() {
    echo
    echo "Stopping services..."

    pkill -f "cmd/tumlive" || true
    pkill -f "runner/cmd/runner/main.go" || true
    pkill -f "worker/cmd/worker" || true
    pkill -f "worker/edge" || true
    pkill -f "vod-service/cmd/vod-service" || true
    pkill -f "mediamtx" || true

    docker stop meilisearch mariadb-tumlive meilisearch >/dev/null 2>&1 || true

    echo "Done."
    exit 0
}

trap cleanup INT TERM

MASS=/home/mkm00/dev/storage/mass
LIVE=/home/mkm00/dev/storage/live

sed -i 's/ListenAndServe(":8089"/ListenAndServe(":8080"/' vod-service/internal/vodService.go
sed -i "s|var vodPath = .*|var vodPath = \"$MASS\"|" worker/edge/edge.go

grep -q '^token:' config.yaml && sed -i 's/^token:.*/token: abc/' config.yaml || echo 'token: abc' >> config.yaml
sed -Ei 's|^([[:space:]]*)externalAuthenticationURL:|\1#externalAuthenticationURL:|' ingest/mediamtx.yml
sed -Ei 's|^([[:space:]]*)#externalAuthenticationURL:[[:space:]]*http://localhost:8081/api/selfstream/onPublish|\1externalAuthenticationURL: http://localhost:8081/api/selfstream/onPublish|' ingest/mediamtx.yml
sed -Ei 's|http://localhost:8081|http://127.0.0.1:8081|' ingest/mediamtx.yml
sed -Ei 's|^ingestbase:.*|ingestbase: rtmp://127.0.0.1|' config.yaml

docker start meilisearch mariadb-tumlive

# Wait for TCP (fast)
until nc -z localhost 3306; do sleep 1; done

until docker logs mariadb-tumlive 2>&1 | grep -q "ready for connections"; do
  sleep 1
done

sleep 5   # <-- THIS is the missing piece

# Wait for Meili
until curl -sf http://localhost:7700/health >/dev/null; do sleep 1; done

echo "DB + Meili ready"

go run ./cmd/tumlive & 
until nc -z localhost 8081; do sleep 1; done
echo "tum-live ready"

STORAGE_PATH="$MASS" SEGMENT_PATH="$LIVE" go run runner/cmd/runner/main.go &
sleep 2
echo "runner started"

cd web
npm install
npm run build-dev &
cd ..

mediamtx ./ingest/mediamtx.yml &
# until nc -z localhost 8554; do sleep 1; done
sleep 2
echo "mediamtx ready"

LrzUploadUrl="http://localhost:8080" \
VodURLTemplate="http://localhost:8089/vod/%s.mp4/playlist.m3u8" \
Token=abc \
MassStorage="$MASS" \
go run ./worker/cmd/worker &
sleep 2
echo "worker started"

go run ./vod-service/cmd/vod-service &
until nc -z localhost 8080; do sleep 1; done
echo "vod-service ready"

go run ./worker/edge &
until nc -z localhost 8089; do sleep 1; done
echo "edge started"

wait