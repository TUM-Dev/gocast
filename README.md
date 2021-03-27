# TUM-Live

**Work in progress** of TUMs lecture streaming service

## Target Architecture:

![Target Architecture](https://raw.githubusercontent.com/joschahenningsen/TUM-Live/dev/target_architecture.png "Target Architecture")

## Development

Developing on this locally is a pain (because we can't just provide the secrets). 
To get started you might want to follow these steps to get the system up locally (without docker):

### Setup IPs
In `/etc/hosts` add this: 
```
127.0.0.1  backend
127.0.0.1  db
```

### Install nginx with the rtmp module:

Follow [this](https://github.com/arut/nginx-rtmp-module#build) guide. 
**Or** use the dockerfile in the folder `streaming-backend` (expose ports 80 and 1935).

### Setup Database

- get mariadb from your favourite package manager
- Set up the password `example` for the user `root`.
- create the database `tumlive`

### Get go running locally

- Install go1.16
- Preferably use Jetbrains GoLand and open this project (get from vcs)
- Edit Configuration > Environment 
  - Add environment variables from `variables-backend.example.env`
- Start the app
- Head over to localhost
- Happy coding! :sparkles:

## Disclaimer

~ 95% of modern browsers are supported. IE is not and probably won't be (officially).
Reason being is that I want to avoid using jQuery because it's way too bloated. 
This is about what you can expect:

Browser | Chrome | Edge | Firefox | IE | Opera | Safari | Android Webview | Chrome Android | Firefox Android | Safari iOS | Samsung Internet
--- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | ---
version | 42+ | 14+ | 39+ | no | 29+ | 10.1+ | 42+ | 42+ | 39+ | 29+ | 10.3+ | 4.0+


## Credit & Licenses

- [ffmpeg](https://ffmpeg.org/) Licenced under [lgpl-2.1](http://www.gnu.org/licenses/old-licenses/lgpl-2.1.html)
- [NGINX](https://www.nginx.com/) Licenced under [FreeBSD](http://nginx.org/LICENSE)
- [go](https://golang.org/) Licence [here](https://golang.org/LICENSE)
- [mariadb](https://mariadb.com/) Licenced under [GPL](https://mariadb.com/kb/en/mariadb-license/)
- todo
