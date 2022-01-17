# Administration

## Server Setup

Follow this guide if you don't use (the currently unstable) docker deployment

### Tested server software

Currently, we successfully run TUM-Live on/with:

- Ubuntu 20.04 LTS
- Nginx 1.18.0
- Go 1.17.*
- MariaDB 15.1 Distrib 10.3.32-MariaDB
- Node 16.3.*
- NPM 8.3.*
- curl 7.68.0
- git 2.25.1

We recommend having all of these installed on your server.

### Tested worker software
The worker runs on Ububntu 20.04 LTS and needs the following software:

- Go 1.17.*
- curl 7.68.0
- git 2.25.1
- ffmpeg 4.2.1

This is the full ffmpeg configuration, any recent apt version should do:
```
ffmpeg version 4.2.4-1ubuntu0.1 Copyright (c) 2000-2020 the FFmpeg developers
  built with gcc 9 (Ubuntu 9.3.0-10ubuntu2)
  configuration: --prefix=/usr --extra-version=1ubuntu0.1 --toolchain=hardened --libdir=/usr/lib/x86_64-linux-gnu --incdir=/usr/include/x86_64-linux-gnu --arch=amd64 --enable-gpl --disable-stripping --enable-avresample --disable-filter=resample --enable-avisynth --enable-gnutls --enable-ladspa --enable-libaom --enable-libass --enable-libbluray --enable-libbs2b --enable-libcaca --enable-libcdio --enable-libcodec2 --enable-libflite --enable-libfontconfig --enable-libfreetype --enable-libfribidi --enable-libgme --enable-libgsm --enable-libjack --enable-libmp3lame --enable-libmysofa --enable-libopenjpeg --enable-libopenmpt --enable-libopus --enable-libpulse --enable-librsvg --enable-librubberband --enable-libshine --enable-libsnappy --enable-libsoxr --enable-libspeex --enable-libssh --enable-libtheora --enable-libtwolame --enable-libvidstab --enable-libvorbis --enable-libvpx --enable-libwavpack --enable-libwebp --enable-libx265 --enable-libxml2 --enable-libxvid --enable-libzmq --enable-libzvbi --enable-lv2 --enable-omx --enable-openal --enable-opencl --enable-opengl --enable-sdl2 --enable-libdc1394 --enable-libdrm --enable-libiec61883 --enable-nvenc --enable-chromaprint --enable-frei0r --enable-libx264 --enable-shared
  libavutil      56. 31.100 / 56. 31.100
  libavcodec     58. 54.100 / 58. 54.100
  libavformat    58. 29.100 / 58. 29.100
  libavdevice    58.  8.100 / 58.  8.100
  libavfilter     7. 57.100 /  7. 57.100
  libavresample   4.  0.  0 /  4.  0.  0
  libswscale      5.  5.100 /  5.  5.100
  libswresample   3.  5.100 /  3.  5.100
  libpostproc    55.  5.100 / 55.  5.100
```

#### nginx

Nginx proxies the requests to TUM-Live. We a config similar to this:

```
user www-data;
worker_processes auto;
pid /run/nginx.pid;
include /etc/nginx/modules-enabled/*.conf;
error_log /path/to/log/nginx/error.log error;

events {
        worker_connections 10000;
}

http {
    include /etc/nginx/mime.types;
    include /etc/nginx/conf.d/*.conf;
    sendfile off;
    tcp_nopush on;
    access_log /var/log/nginx/access.log combined;
    limit_req_zone $binary_remote_addr zone=one:10m rate=100r/m;
    limit_req_zone $binary_remote_addr zone=pwd:10m rate=10r/m;
    limit_req_zone $binary_remote_addr zone=strict:10m rate=15r/m;
    limit_conn_zone $binary_remote_addr zone=addr:10m;

    map $sent_http_content_type $expires {
        default                    off;
        text/css                   max;
        application/javascript     max;
        font/woff2                 max;
        ~image/                    max;
    }

    map $http_upgrade $connection_upgrade {
        default upgrade;
        ''      close;
    }

    server {
        listen 80;
        listen [::]:80;
        server_name example.com;

        location / {
            return 301 https://live.rbg.tum.de$request_uri;
        }
    }

    server {
        listen 443 ssl http2;
        listen [::]:443 ssl http2;
        server_name example.com;
        ssl_certificate /path/to/fullchain.pem;
        ssl_certificate_key /path/to/privkey.pem;
        client_max_body_size 2000m;

        error_page 503 /error_503.html;
        location = /error_503.html {
                root /usr/share/nginx/html;
                internal;
        }

        error_page 502 /error_502.html;
        location = /error_502.html {
                root /usr/share/nginx/html;
                internal;
        }


        location ~ ^/api/chat/[0-9]+/ws$ {
            limit_req zone=one burst=100 nodelay;
            proxy_pass http://127.0.0.1:8081;
            proxy_http_version 1.1;
            proxy_set_header Upgrade $http_upgrade;
            proxy_set_header Connection $connection_upgrade;
        }

        location /login {
            client_max_body_size 1M;
            limit_req zone=pwd burst=5 nodelay;
            proxy_pass http://127.0.0.1:8081;
        } 

        location /api/course/ {
            # no limit, only admins here limit_req zone=strict burst=5 nodelay;
            proxy_pass http://127.0.0.1:8081;
        }

        location /static/ {
            limit_req zone=one burst=200 nodelay;
            expires 1y;
            add_header Pragma public;
            add_header Cache-Control "public";
            proxy_pass http://127.0.0.1:8081;
        }

        location /public/ {
            # limit_req zone=one burst=200 nodelay;
            expires 1y;
            add_header Pragma public;
            add_header Access-Control-Allow-Origin *;
            add_header Cache-Control "public";
            proxy_pass http://127.0.0.1:8081;
        }

        location / {
            limit_req zone=one burst=50 nodelay;
            expires $expires;
            proxy_pass http://127.0.0.1:8081;
            proxy_set_header Host            $host;
            proxy_set_header X-Forwarded-For $remote_addr;
        }
    }
}
```

#### Systemd service:

This is our systemd service definition:

```
Unit]
Description=TUM-Live
After=network.target
Requires=mariadb.service

[Service]
LimitNOFILE=1048576:2097152
Type=simple
ExecStart=/bin/tum-live
TimeoutStopSec=5
KillMode=mixed
Restart=on-failure
StandardOutput=append:/path/to/log/tum-live/logs.log
StandardError=append:/path/to/log/tum-live/error.log

[Install]
WantedBy=multi-user.target
```

#### Setup the database

Create a database in MariaDB and setup a user with read and write permissions.

#### Configure the server

Create a config file for the server located at /etc/TUM-Live/config.yaml

```yaml
lrz:
  name: "LRZ Uploader Name"
  email: "example@tum.de"
  phone: "555-123-456"
  uploadUrl: "https://server.of.lrz.de/video_upload.cgi"
  subDir: "RBG"
mail:
  sender: "server sender @ domain"
  server: "mailrelay.your.org:25"
  SMIMECert: "/path/to/mail.p12.crt.pem"
  SMIMEKey: "/path/to/mail.p12.key.pem"
db:
  user: "user"
  password: "password"
  database: "database"
campus:
  base: "https://campus.tum.de/tumonlinej/ws/webservice_v1.0"
  tokens:
    - "token"
    - "token2"
    - "token3"
ldap:
  url: "ldaps://iauth.somewhere:636"
  user: "cn=abv,ou=bindDNs,ou=iauth,dc=tum,dc=de"
  password: "secret_password"
  baseDn: "ou=users,ou=data,ou=prod,ou=iauth,dc=tum,dc=de"
  userDn: "cn=%v,ou=users,ou=data,ou=prod,ou=iauth,dc=tum,dc=de"
paths:
  static: "/var/www/public"
  mass: "/path/to/cephfs/livestream/rec/TUM-Live/"
auths:
  smpUser: "user"
  smpPassword: "password"
  pwrCrtlAuth: "user:password"
  camAuth: "user:password"
ingestBase: "rtmp://ingest.some.tum.de/"
cookieStoreSecret: "put a bunch of secred characters here"
```

#### Installation:

TUM-Live can easily be installed with the following commands:

```bash
git clone git@github.com:joschahenningsen/TUM-Live.git
cd TUM-Live#
make all
sudo make install
sudo service tum-live restart
```

#### Update:

Updating TUM-Live works by pulling and rebuilding the source code:

```bash
cd TUM-Live
git pull -X theirs
make all
sudo make install
sudo service tum-live restart
```
