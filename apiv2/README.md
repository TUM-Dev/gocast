# GOCAST API V2

This API is designed to be a user-friendly and easy-to-use interface for third party services. It provides access to all non-administrative features of the GoCast platform via gRPC methods and exposes a REST API proxy for easy access.

In the future, this API might be extended to include more features and endpoints and replace the current REST API.

## Documentation

You can find the docs for the new API [here](https://tum.live/api/v2/docs).

You can generate the code documentation using `godoc` and find it at [http://localhost:6060/pkg/github.com/TUM-Dev/gocast](`http://localhost:6060/pkg/github.com/TUM-Dev/gocast`).

## File structure

All proto messages can be found in `apiv2.proto`.
The actual endpoints are implemented in `<./endpoint.go>.go`, custom erros in `./errors` and helper functions such as parsers, custom database queries, etc. in `./helpers`.

## Config

Install protobuf by running `./apiv2/installBuf.sh`.

To generate the files in `./protobuf`, run:
`./apiv2/generate.sh`.

## Running the server

To build and start the new API, start GoCast as usual with:
`go run ./cmd/tumlive/tumlive.go`.

The gRPC server will be running on port 12544 and the API proxy on [localhost:8081/api/v2](http://localhost:8081/api/v2/status).<br>
The docs can be found at [http://localhost:8081/api/v2/docs](http://localhost:8081/api/v2/docs).
