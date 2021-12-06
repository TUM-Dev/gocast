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

- Install go1.17
- Preferably use Jetbrains GoLand and open this project
- Edit Configuration > Environment 
  - Add environment variables from `variables-backend.example.env`.
- Start the app
- Head over to localhost:8081
- Happy coding! :sparkles:

### Enable pre-commit hooks

- **Prerequisites**: Make sure you have [staticcheck](https://staticcheck.io/docs/getting-started/)
and [pre-commit](https://pre-commit.com/#install) installed.
- Run`pre-commit install`. It will install the pre-commit hook scripts for this repository.

Now the hook scripts will be triggered for every new commit, which should improve overall code quality.
You can also run the pre-commit hooks manually for all files by executing `pre-commit run --all-files`.

### Linting and formatting typescript files

The following scripts are provided:

- `npm run lint`: Runs `eslint` and `prettier` on the code to find stylistic issues.
- `npm run lint-fix`: Same as above but also fixes the found issues.

If you use Goland Ultimate, you can use follow this [guide](https://www.jetbrains.com/help/idea/prettier.html).
The provided config runs `prettier` on formatting actions as well as on saving.

## Credit & Licenses

- [Check out our dependencies](https://github.com/joschahenningsen/TUM-Live/network/dependencies)
