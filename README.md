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

## Getting Started

To get this running locally follow these steps:

### Setup IPs

In `/etc/hosts` add this: 
```
127.0.0.1  db
```

### Setup Database
- Follow the steps [here](https://mariadb.com/kb/en/installing-and-using-mariadb-via-docker/) to install mariadb via docker.
- Then run the docker container using the following command.
```bash
docker run --detach --name mariadb-tumlive --env MARIADB_USER=root --env MARIADB_PASSWORD=root --env MARIADB_ROOT_PASSWORD=example --restart always -p 3306:3306 mariadb:latest`
```
- Alternatively, install mariadb on its own.
- Create the database `tumlive` using [this]([tum-live-starter.zip](https://github.com/joschahenningsen/TUM-Live/files/8505487/tum-live-starter.zip)) script.
- Or: Use [JetBrains DataGrip](https://www.jetbrains.com/datagrip/) to open the database and then run the script there to automatically set up a demo database.

### Installing go

- Install **go >=1.18** by following the steps [here](https://go.dev/doc/install)
- Preferably use [JetBrains GoLand](https://www.jetbrains.com/go/) and open this project as it simplifies this entire process
- Run `npm i` in the `./web` directory to install the required node modules
- Run `go get ./...` to install the required go modules
- If you want to customize the configuration (for example mariadb username and password), copy the `config.yaml` file over to `$HOME/.TUM-Live/config.yaml` and make your changes there to prevent accidentally committing them.
- Start the app by building and running `./cmd/tumlive/tumlive.go`
- Head over to `http://localhost:8081` in your browser of choice and confirm that the page has loaded without any problems.
- Voilà! Happy coding! :sparkles:

### Enable pre-commit hooks

- Make sure you have [staticcheck](https://staticcheck.io/docs/getting-started/)
and [pre-commit](https://pre-commit.com/#install) installed. If you have `pip` installed on your machine, you can install them with the following command
```bash
go install honnef.co/go/tools/cmd/staticcheck@latest & pip install pre-commit
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

## Credit & Licenses

- [Check out our dependencies](https://github.com/joschahenningsen/TUM-Live/network/dependencies)
