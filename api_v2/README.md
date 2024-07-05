# GOCAST API V2

## Documentation

You can find the api-doc for the new api [here](http://localhost:8081/api/v2/docs).

You can generate the code documentation using `godoc` and find it at [http://localhost:6060/pkg/github.com/TUM-Dev/gocast](`http://localhost:6060/pkg/github.com/TUM-Dev/gocast`).

## File structure

All proto messages can be found in `api_v2.proto`.
The actual endpoints are implemented in `<./endpoint.go>.go`, the database queries in `./services`, custom erros in `./errors` and helper functions such as parsers etc. in `./helpers`. 

## Config

Install protobuf by running `./api_v2/installBuf.sh`.

To generate the files in `./protobuf`, run:
`./api_v2/generate.sh`.

To build and start the server on port 8081, run:
`go build ./cmd/tumlive/tumlive.go && go run ./cmd/tumlive/tumlive.go`.