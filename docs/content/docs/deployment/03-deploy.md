---
title: "Deploy"
draft: false
weight: 40
---

## Configuration

To run GoCast, copy the contents of the `/docs/static/deployment` directory to your server into a shared location that is available to all nodes.
Edit the `docker-compose.yml` file to match your environment (domains, file locations,...). This is a demo of the file:

```yaml
version: '3.8'
services:
  tumlive:
    image: ghcr.io/joschahenningsen/tum-live/tum-live:latest
    depends_on:
      - tumlivedb
    ports:
      - target: 50052
        published: 50052
        protocol: tcp
        mode: host
    volumes:
      - /share/deployment/config.yaml:/etc/TUM-Live/config.yaml # todo make sure /share is available on all nodes
      - /share/branding:/etc/TUM-Live/branding
      - /share:/mass
      - /var/lib/rbg-cert/live:/var/lib/rbg-cert/live
      - /path/to/mail.p12.crt.pem:/path/to/mail.p12.crt.pem # todo change this to your mail cert
      - /path/to/mail.p12.key.pem:/path/to/mail.p12.key.pem
      - /var/www/public:/var/www/public
    networks:
      - default
    environment:
      - TMPDIR=/tmp
      - TMP=/tmp
      - GIN_MODE=release
      - TZ=Europe/Berlin
    deploy:
      mode: replicated
      replicas: 1
      restart_policy:
        condition: on-failure
      placement:
        constraints:
          - "node.labels.tumlive==true"
      labels:
        - "traefik.enable=true"
        - "traefik.http.middlewares.webs-redirectscheme.redirectscheme.scheme=https"
        - "traefik.http.middlewares.webs-redirectscheme.redirectscheme.permanent=true"

        # dynamic
        - "traefik.http.routers.tumlive.entrypoints=web"
        - "traefik.http.routers.tumlive.rule=Host(`live.rbg.tum.de`) || Host(`tum.live`)" # todo change url/s
        - "traefik.http.routers.tumlive.middlewares=webs-redirectscheme"
        - "traefik.http.routers.tumlive.service=tumlive-secure"


        - "traefik.http.routers.tumlive-secure-static.entrypoints=webs"
        - "traefik.http.routers.tumlive-secure-static.tls=true"
        - "traefik.http.routers.tumlive-secure-static.tls.certresolver=liveresolver"
        - "traefik.http.routers.tumlive-secure-static.rule=(Host(`live.rbg.tum.de`) || Host(`tum.live`)) && PathPrefix(`/static/`, `/public/`)"  # todo change url/s
        - "traefik.http.routers.tumlive-secure-static.service=tumlive-secure"
        - "traefik.http.services.tumlive-secure-static.loadbalancer.server.port=8081"
        - "traefik.http.routers.tumlive-secure-static.middlewares=cache-headers"


        - "traefik.http.routers.tumlive-secure.entrypoints=webs"
        - "traefik.http.routers.tumlive-secure.tls=true"
        - "traefik.http.routers.tumlive-secure.tls.certresolver=liveresolver"
        - "traefik.http.routers.tumlive-secure.rule=Host(`live.rbg.tum.de`) || Host(`tum.live`)" # todo change url/s
        - "traefik.http.routers.tumlive-secure.service=tumlive-secure"
        - "traefik.http.services.tumlive-secure.loadbalancer.server.port=8081"

  tumlivedb:
    image: mariadb:latest
    environment:
      - MARIADB_USER=root
      - MARIADB_ROOT_PASSWORD=abc123 # todo change this here and in gocasts config.yaml
      - TZ=Europe/Berlin
    networks:
      - default
    deploy:
      mode: replicated
      replicas: 1
      placement:
        constraints:
          - "node.labels.db==true"
    volumes:
      - mariadb_data:/var/lib/mysql

  worker:
    image: ghcr.io/joschahenningsen/tum-live/worker:latest
    networks:
      - default
    environment:
      - Token=abc123 # todo change this here and in gocasts config.yaml
      - MainBase=tumlive
      - Host={{.Node.Hostname}}
      - LrzUploadUrl=http://vodservice:8089
      - LogLevel=debug
      - PersistDir=/persist
      - VodURLTemplate=https://edge.live.rbg.tum.de/vod/%s.mp4/playlist.m3u8 # todo change this depending on your edge server url
    ports:
      - target: 1935
        published: 1935
        mode: host
        protocol: tcp
    volumes:
      - recordings:/recordings
      - persist:/persist
      - /share:/mass # todo make sure /share is available on all nodes
      - workerlog:/var/log/stream
    deploy:
      mode: global # replicate to every node
      placement:
        constraints:
          - "node.labels.worker==true"
      restart_policy:
        condition: on-failure

  # optional
  voice-service:
    image: ghcr.io/tum-dev/tum-live-voice-service-nvidia:0.0.5
    volumes:
      - /share:/mass # todo make sure /share is available on all nodes
    networks:
      - default
    deploy:
      resources:
        reservations:
          generic_resources:
            - discrete_resource_spec:
                kind: 'gpu'
                value: 0
      mode: global
      placement:
        constraints:
          - "node.labels.voiceservice==true"
    environment:
      - TRANSCRIBER=whisper
      - WHISPER_MODEL=medium
      - MAX_WORKERS=1
      - DEBUG=1
      - REC_HOST=tumlive
      - REC_PORT=50053

  edge:
    image: ghcr.io/joschahenningsen/tum-live/worker-edge:latest
    networks:
      - default
    ports:
      - target: 8089
        published: 80
        mode: host
        protocol: tcp
      - target: 8443
        published: 443
        mode: host
        protocol: tcp
    environment:
      - CERT_DIR=/var/lib/rbg-cert/live/ # todo, this directory must exist on all edge nodes and contain ssl certificates valid for the domain the nodes use.
      - VOD_DIR=/vod
      - MAIN_INSTANCE=http://tumlive:8081
      - ADMIN_TOKEN=abc123 # todo changeme
    volumes:
      - /share/vod:/vod # todo make sure /share is available on all hosts
      - /var/lib/rbg-cert/:/var/lib/rbg-cert/
    deploy:
      mode: global
      endpoint_mode: dnsrr
      placement:
        constraints:
          - "node.labels.edge==true"

  vodservice:
    image: ghcr.io/joschahenningsen/tum-live/vod-service:latest
    networks:
      - default
    ports:
      # web
      - target: 8089
        published: 8089
        protocol: tcp
        mode: host
    environment:
      - OUTPUT_DIR=/out
    volumes:
      - /share/vod:/out # todo make sure /share is available on all hosts
    deploy:
      mode: global
      placement:
        constraints:
          - "node.labels.worker==true"

  meilisearch:
    image: getmeili/meilisearch:v0.30
    volumes:
      - meilisearch:/meili_data
    networks:
      - default
    environment:
      - MEILI_MASTER_KEY=abc123 # todo change me
    deploy:
      mode: replicated
      replicas: 1
      restart_policy:
        condition: on-failure
      placement:
        constraints:
          - "node.labels.meilisearch==true"

  grafana:
    image: grafana/grafana
    volumes:
      - grafana:/var/lib/grafana
      - /shared/deployment/grafana.ini:/etc/grafana/grafana.ini # todo make sure /shared is available on all hosts
    networks:
      - default
    deploy:
      mode: replicated
      replicas: 1
      restart_policy:
        condition: on-failure
      placement:
        constraints:
          - "node.labels.grafana==true"
      labels:
        - "traefik.enable=true"
        - "traefik.http.middlewares.webs-redirectscheme.redirectscheme.scheme=https"
        - "traefik.http.middlewares.webs-redirectscheme.redirectscheme.permanent=true"

        # dynamic
        - "traefik.http.routers.grafana.entrypoints=web"
        - "traefik.http.routers.grafana.rule=Host(`grafana.my.domain`)" # todo pick a domain
        - "traefik.http.routers.grafana.middlewares=webs-redirectscheme"
        - "traefik.http.routers.grafana.service=grafana-secure"

        - "traefik.http.routers.grafana-secure.entrypoints=webs"
        - "traefik.http.routers.grafana-secure.tls=true"
        - "traefik.http.routers.grafana-secure.tls.certresolver=liveresolver"
        - "traefik.http.routers.grafana-secure.rule=Host(`grafana.my.domain`)" # todo pick a domain
        - "traefik.http.routers.grafana-secure.service=grafana-secure"
        - "traefik.http.services.grafana-secure.loadbalancer.server.port=3000"

  prometheus:
    image: prom/prometheus
    volumes:
      - /shared/deployment/prometheus.yml:/etc/prometheus/prometheus.yml # todo, /shared has to be available on all hosts
      - prometheus:/prometheus
    networks:
      - default
    deploy:
      mode: replicated
      replicas: 1
      restart_policy:
        condition: on-failure
      placement:
        constraints:
          - "node.labels.prometheus==true"

  traefik:
    image: traefik:v2.9
    networks:
      - default
    deploy:
      mode: replicated
      replicas: 1
      placement:
        constraints:
          - "node.labels.traefik==true"
      labels:
        - "traefik.enable=true"
        #General purpose redirect middleware used throughout
        - "traefik.http.middlewares.webs-redirectscheme.redirectscheme.scheme=https"
        - "traefik.http.middlewares.webs-redirectscheme.redirectscheme.permanent=true"
        - "traefik.http.middlewares.cache-headers.headers.customresponseheaders.Cache-Control=public,max-age=2592000"

        - "traefik.http.routers.traefik.entrypoints=web"
        - "traefik.http.routers.traefik.rule=Host(`traefik.my.domain`)" # todo in case you need the traefik interface
        - "traefik.http.routers.traefik.middlewares=webs-redirectscheme"
        - "traefik.http.routers.traefik.service=traefik-secure"

        - "traefik.http.routers.traefik-secure.entrypoints=webs"
        - "traefik.http.routers.traefik-secure.tls=true"
        - "traefik.http.routers.traefik-secure.tls.certresolver=liveresolver"
        - "traefik.http.routers.traefik-secure.rule=Host(`traefik.my.domain`)" # todo in case you need the traefik interface
        - "traefik.http.routers.traefik-secure.service=traefik-secure"

        - "traefik.http.services.traefik-secure.loadbalancer.server.port=8080"
    ports:
      # web
      - target: 80
        published: 80
        protocol: tcp
        mode: host
      - target: 443
        published: 443
        protocol: tcp
        mode: host
    volumes:
      # So that Traefik can listen to the Docker events
      - /var/run/docker.sock:/var/run/docker.sock
      - /srv/cephfs/livestream/TUM-Live/deployment/traefik.toml:/etc/traefik/traefik.toml:ro
      - /srv/cephfs/livestream/TUM-Live/deployment/acme:/acme
      - /var/log/traefik:/var/log/traefik

  whoami:
    # A container that exposes an API to show its IP address
    image: traefik/whoami
    networks:
      - default
    deploy:
      mode: global

  campus-proxy:
    image: ghcr.io/tum-dev/campusproxy/proxy:latest
    networks:
      - default
    deploy:
      mode: replicated
      replicas: 2
      labels:
        - "traefik.enable=true"
        - "traefik.http.routers.proxy.entrypoints=web"
        - "traefik.http.routers.proxy.rule=Host(`campus-proxy.my.domain`)" # todo pick a url
        - "traefik.http.routers.proxy.middlewares=webs-redirectscheme"
        - "traefik.http.routers.proxy-service=proxy-secure"

        - "traefik.http.routers.proxy-secure.entrypoints=webs"
        - "traefik.http.routers.proxy-secure.tls=true"
        - "traefik.http.routers.proxy-secure.tls.certresolver=liveresolver"
        - "traefik.http.routers.proxy-secure.rule=Host(`campus-proxy.my.domain`)" # todo pick a url
        - "traefik.http.routers.proxy-secure.service=proxy-secure"

        - "traefik.http.services.proxy-secure.loadbalancer.server.port=8020"
  googleSiteVerification:
    image: nginx:latest
    volumes:
      # todo: use your own google verification file
      - /srv/cephfs/livestream/TUM-Live/deployment/google695ffe94aec91c5d.html:/usr/share/nginx/html/google695ffe94aec91c5d.html
    deploy:
      mode: replicated
      replicas: 2
      labels:
        - "traefik.enable=true"

        - "traefik.http.routers.gsv-secure.entrypoints=webs"
        - "traefik.http.routers.gsv-secure.tls=true"
        - "traefik.http.routers.gsv-secure.tls.certresolver=liveresolver"
        - "traefik.http.routers.gsv-secure.rule=Host(`live.rbg.tum.de`) && Path(`/google695ffe94aec91c5d.html`)"
        - "traefik.http.routers.gsv-secure.service=gsv-secure"

        - "traefik.http.services.gsv-secure.loadbalancer.server.port=80"

volumes:
  recordings:
  persist:
  mariadb_data:
  workerlog:
  meilisearch:
  grafana:
  prometheus:

networks:
  agent_network:
    driver: overlay
    attachable: true
  default:
    driver: overlay
  host:
    name: host
    external: true
```

Edit the `traefik.toml` file to your needs. You can find an example in the `deployment` folder.

Edit the `config.yaml` file to your needs:

```yaml
alerts:
  matrix:
    homeserver: matrix.org # todo changeme
    password: password # todo changeme
    alertRoomID: '!abc:in.tum.de' # todo changeme
    logRoomID: '!abc123:matrix.org' # todo changeme
    username: username # todo changeme
auths:
  camauth: user:password # todo changeme
  pwrcrtlauth: user:password # todo changeme
  smppassword: "password" # todo changeme
  smpuser: user # todo changeme
campus:
  base: https://campus.tum.de/tumonlinej/ws/webservice_v1.0 # todo changeme
  tokens:
    - abc123 # todo changeme
  campusProxy: # new services use this proxy from now on
    host: campus-proxy.my.domain # todo changeme
    scheme: https
  relevantOrgs: # 0 = all
    - 51897 # cit
    - 30361 # studentische vertretung
    - 30290 # fachschaften
    - 14189 # institut für informatik
    - 14178 # fakultät für mathematik
    - 14179 # fakultät für physik
    - 51267 # tum school of engineering and design
    - 51900 # tum school of management
db:
  database: tumlive
  password: abc123 # todo changeme
  user: root
  host: tumlivedb
  port: 3306
ingestbase: rtmp://vmrbg458.in.tum.de/
jwtkey:
ldap:
  useForLogin: true
  basedn: ou=users,ou=data,ou=prod,ou=iauth,dc=tum,dc=de  # todo changeme
  password: abc123  # todo changeme
  url: ldaps://iauth.tum.de:636
  user: cn=usernameChangeme,ou=bindDNs,ou=iauth,dc=tum,dc=de # todo changeme
  userdn: cn=%s,ou=users,ou=data,ou=prod,ou=iauth,dc=tum,dc=de  # todo changeme
saml: # todo changeme
  idpMetadataURL: https://login.tum.de/idp-metadata.xml
  idpName: TUM Login
  idpColor: "#3070B3"
  cert: /var/lib/cert/live/host:intum:vmrbg451.fullchain.pem
  privkey: /var/lib/cert/live/host:intum:vmrbg451.privkey.pem
  entityID: https://live.rbg.tum.de/shib
  rootURLs:
    - https://live.rbg.tum.de/shib
    - https://tum.live/shib
mail:
  sender: live@my.domain # todo changeme
  server: mailrelay.my.domain:25 # todo changeme
  smimecert: /path/to/mail.p12.crt.pem
  smimekey: /path/to/mail.p12.key.pem
paths:
  mass: /share
  static: /var/www/public
  branding: /etc/TUM-Live/branding
workertoken: abc123 # todo changeme
weburl: https://live.rbg.tum.de
monitoring:
  sentryDSN: https://abc@sentry.com/2 # todo changeme
  sampleRate: 0.1
meili:
  host: http://meilisearch:7700
  apiKey: abc123 # todo changeme
vodURLTemplate: "https://edge.live.rbg.tum.de/vod/%s.mp4/playlist.m3u8"
voiceservice:
  host: voice-service
  port: 50055
canonicalURL: https://live.rbg.tum.de
```

## Deployment


```bash
$ docker stack deploy -c docker-compose.yml gocast
```

After a few minutes, everything should be up and running, certificates are issued automatically