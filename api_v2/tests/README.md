# GRPC Endpoints Tests

> **Note**: _Currently only the courses endpoint tests are implemented._

## General Information

This directory contains all tests for the gRPC endpoints of the API/v2.
Currently the tests are executed by starting a test-instance of tumlive and calling the actual endpoints (without mocking any db-access services in contrast to the other unit test suites).   
Currently the tests only check whether the returned gRPC response status codes match the expected gRPC response status codes.

## Getting Started

To run the tests:

1. Create a test database in your mariadb instance using the `tum-live-test.sql` file (can be copy-pasted into a mariadb terminal)

2. Run the tests using `go test ./...`