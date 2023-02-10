# [TUM-Live](https://live.rbg.tum.de)

[![volkswagen status](https://auchenberg.github.io/volkswagen/volkswargen_ci.svg?v=1)](https://github.com/auchenberg/volkswagen) [![Better Uptime Badge](https://betteruptime.com/status-badges/v1/monitor/7hms.svg)](https://tum-live.betteruptime.com)


TUMs lecture streaming service, currently serving up to 100 courses every semester with up to 2000 active students.

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

## Architecture

```
                                          ┌──────────────────────┐
                              ┌───────────►   Campus Management  │
                              │           │ System (CAMPUSOnline)│
               ┌──────────┐   │Enrollments└──────────────────────┘
               │Identity  │   │
               │Management│   │                 - Users,
               │  - SAML  ◄─┐ │                 - Courses,
               │  - LDAP  │ │ │                 - Streams, ...                               ┌────────────────────┐
               └──────────┘ │ │              ┌──────────┐                                    │Lecture Hall        │
                      Users │ │  ┌──────────►│ Database │                                    │ - Streaming Device │
                            │ │  │           └──────────┘                               ┌────┤ - Camera           │
                            │ │  │                                                      │    │ - Slides (HDMI)    │
                         ┌──┴─┴──┴────┐         Task Distribution (gRPC)            RTSP│    │ - Microphone       │
            ┌────────────►  TUM-Live  │◄─────────────────────────────────────┐      pull│    └────────────────────┘
            │Website     └────────────┘                                      │          │
            │(HTTP)                                                          ▼          │
            │                                                             ┌─────────────▼─────┬─┐
            │                               ┌────────────────┐            │TUM-Live-Worker #1 │ │ Streaming,
┌───────────┴──┐                            │ Shared Storage │            ├───────────────────┘ │ Converting,
│Student/Viewer│                            │ S3, Ceph, etc. │            │  TUM-Live-Worker #n │ Transcribing, ...
└───────────┬──┘                            └─▲────▲─────────┘            └──────┬──▲────▲──────┘
            │                        Serve Vod│    │HLS Files            Push VoD│  │    │RTMP
            │                          Content│    │         ┌───────────┐ (HTTP)│  │    │push
            │                                 │    └─────────┤VoD Service◄───────┘  │    │       ┌──────────────┐
            │Videos      ┌──────────────────┬─┴┐             └───────────┘          │    └───────┤Selfstreamer  │
            │(HLS, HTTP) │ TUM-Live Edge #1 │  │                                    │            │  - OBS,      │
            └────────────►──────────────────┘  ├────────────────────────────────────┘            │  - Zoom, ... │
                         │   TUM-Live Edge #n  │       Proxy, Cache (HTTP)                       └──────────────┘
                         └─────────────────────┘
```

## Getting Started

The easiest way of running and testing TUM-Live is by using the provided docker-compose file:

```bash
docker compose build && docker compose up
```
Be advised that the compose file is not indented for production use as it runs everything on one machine.

If you want to get TUM-Live running natively follow these steps:

### Setup Database
- Follow the steps [here](https://mariadb.com/kb/en/installing-and-using-mariadb-via-docker/) to install mariadb via docker.
- Then run the docker container using the following command.
```bash
docker run --detach \
  --name mariadb-tumlive \
  --env MARIADB_USER=root \
  --env MARIADB_ROOT_PASSWORD=example \
  --restart always \
  -p 3306:3306 \
  --volume "$(pwd)"/docs/static/tum-live-starter.sql:/init.sql \
  mariadb:latest --init-file /init.sql
```
- Alternatively, install mariadb on its own.
  - Create the database `tumlive` using [this](https://github.com/joschahenningsen/TUM-Live/files/8505487/tum-live-starter.zip) script.
  - Or: Use [JetBrains DataGrip](https://www.jetbrains.com/datagrip/) to open the database and then run the script there to automatically set up a demo database.
- The database contains the users `admin`, `prof1`, `prof2`, `studi1`, `studi2` and `studi3` with the password `password`.

### Install go

- Install **go >=1.18** by following the steps [here](https://go.dev/doc/install)
- Preferably use [JetBrains GoLand](https://youtu.be/vetAfxQxyJE) and open this project as it simplifies this entire process
- Go to File -> Settings -> Go -> Go Modules and enable go modules integration.
- Run `npm i` in the `./web` directory to install the required node modules
- Run `go get ./...` to install the required go modules
- If you want to customize the configuration (for example mariadb username and password), copy the `config.yaml` file over to `$HOME/.TUM-Live/config.yaml` and make your changes there to prevent accidentally committing them.
- Start the app by building and running `./cmd/tumlive/tumlive.go`
- Head over to `http://localhost:8081` in your browser of choice and confirm that the page has loaded without any problems.
- To keep automatically rebuilding the frontend code during development, run the command `npm run build-dev` in `./web` (and keep it running).
- Voilà! Happy coding! :sparkles:

### Enable pre-commit hooks

- Make sure you have [staticcheck](https://staticcheck.io/docs/getting-started/)
and [pre-commit](https://pre-commit.com/#install) installed. If you have `pip` installed on your machine, you can install them with the following command
```bash
go install honnef.co/go/tools/cmd/staticcheck@latest && pip install pre-commit
```
- Run`pre-commit install`. It will install the pre-commit hook scripts for this repository.

Now the hook scripts will be triggered for every new commit, which should improve overall code quality.
You can also run the pre-commit hooks manually for all files by executing `pre-commit run --all-files`. If you get the error message `The unauthenticated git protocol on port 9418 is no longer supported.`, try running the following command
```bash
git config --global url."https://github.com/".insteadOf git://github.com/
```
See [this](https://github.blog/2021-09-01-improving-git-protocol-security-github/) blogpost for more information on this error message.
### Linting and formatting typescript files

The following scripts are provided:

- `npm run lint`: Runs `eslint` and `prettier` on the code to find stylistic issues.
- `npm run lint-fix`: Same as above but also fixes the found issues.

If you use GoLand, you can use follow this [guide](https://www.jetbrains.com/help/idea/prettier.html) to integrate
prettier. There is also a [guide](https://www.jetbrains.com/help/go/eslint.html) for integrating `eslint`. For both configs are provided that should be automatically detected. If you set everything up correctly,
`prettier` and `eslint` should run everytime you save. Additionally, GoLands formatter will now respect the `prettier`
style rules.

### Add Database Models:

To create database models and their corresponding daos there is a helper script that can be used to automate this task:

```shell
go run cmd/modelGen/modelGen.go <NameOfYourModel(UpperCamelCase)>
```

### Customization

An exemplary configuration can be found in `/branding`.

#### logo, favicon, manifest.json

For customization mount a directory containing the files in the docker container. Make sure to 
specify the location of the directory in the container in the configuration file as `paths > branding`. 
See `/config.yaml` for an exemplary configuration.

#### title, description
If intended, put a `branding.yaml` file at the same location as `config.yaml`.

## Credit & Licenses

- [Check out our dependencies](https://github.com/joschahenningsen/TUM-Live/network/dependencies)
