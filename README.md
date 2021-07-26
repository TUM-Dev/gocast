# [TUM-Live](https://live.rbg.tum.de)

[![volkswagen status](https://auchenberg.github.io/volkswagen/volkswargen_ci.svg?v=1)](https://github.com/auchenberg/volkswagen) [![Better Uptime Badge](https://betteruptime.com/status-badges/v1/monitor/7hms.svg)](https://tum-live.betteruptime.com)


TUMs lecture streaming service, in beta since summer semester 2021.
Currently serving 12 courses with up to 1500 active students.

Features include:
- Automatic lecture scheduling and access management coupled with [CAMPUSOnline](https://www.tugraz.at/tu-graz/organisationsstruktur/serviceeinrichtungen-und-stabsstellen/campusonline/)
- Livestreaming from lecture halls
  - Support for Extron SMPs and automatic backup recordings on them.
  - Support for preset management on ip cameras
  - Automatic recordings and video on demand with granular access control.
- Self-streaming
  - Stream ingest from Home using OBS or similar software.
- Live chat 
- Statistics (live and VoD view count)
- Self-service dashboard for lecturers 
  - schedule streams, manage access...

## Architecture:

![Architecture](https://raw.githubusercontent.com/joschahenningsen/TUM-Live/dev/target_architecture.png "Architecture")

## Development

Developing on this locally is a pain (because there are a few secrets involved). 
There is a dockerfile/docker-compose.yml. I don't guarantee that it works because we can't currently use it in production.
To get this running locally follow these steps:

### Setup IPs
In `/etc/hosts` add this: 
```
127.0.0.1  db
```

### Setup Database

- get mariadb from your favourite package manager or docker (I recommend this option)
- `docker run --name mariadb -e MYSQL_ROOT_PASSWORD=example -p 3306:3306 -d mariadb:latest`
- create the database `tumlive`

### Get go running locally

- Install go1.16
- Preferably use Jetbrains GoLand and open this project
- Edit Configuration > Environment 
  - Add environment variables from `variables-backend.example.env`.
- Start the app
- Head over to localhost:8081
- Happy coding! :sparkles:

## Credit & Licenses

- [Check out our dependencies](https://github.com/joschahenningsen/TUM-Live/network/dependencies)
