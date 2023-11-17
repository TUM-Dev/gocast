#!/bin/bash

# needs buf: https://docs.buf.build/installation#github-releases
BASEDIR=$(dirname "$0")
echo making sure that this script is run from $BASEDIR
pushd $BASEDIR > /dev/null

echo updating the generated files
export PATH="$PATH:$(go env GOPATH)/bin"
buf mod update || exit 1
buf generate || exit 1

echo making sure that all artifacts we don\'t need are cleaned up
rm -rf docs/google docs/protoc-gen-openapiv2 protobuf/google protobuf/protoc-gen-openapiv2

echo maing sure that the generated files are formatted
go fmt protobuf/*.go || exit 1

# clean up the stack
popd > /dev/null