# GOCAST API V2

## Documentation

You can find the api-doc for the new api [here](http://localhost:8081/api/v2/docs).

## Config

Install protobuf by running `cd api_v2 && ./    installBuf.sh`.

To build and start the server on port 8081, run:
`./generate.sh && ../tumlive`.

To compile, build and run using one command, run:
`go build ./cmd/tumlive/tumlive.go && ./api_v2/generate.sh && ./api_v2/tumlive`