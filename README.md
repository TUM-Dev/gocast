# TUM-Live

**Work in progress** of TUMs lecture streaming service

## Target Architecture:

![Target Architecture](https://raw.githubusercontent.com/joschahenningsen/TUM-Live/dev/target_architecture.png "Target Architecture")

## Development

Developing on this locally is a pain (because we can't just provide the secrets). 
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
  - Add environment variables from `variables-backend.example.env`. Sentry is currently required but if you just want to test stuff, edit it out of main.go.
- Start the app
- Head over to localhost:8081
- Happy coding! :sparkles:

## Credit & Licenses

- [Check out our dependencies](https://github.com/joschahenningsen/TUM-Live/network/dependencies)
- Also, check out Mariadb, Go and `go.mod`