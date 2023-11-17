#!/bin/bash

BASEDIR=$(dirname "$0")
echo making sure that this script is run from $BASEDIR
pushd $BASEDIR > /dev/null

echo downloading...
go get github.com/bufbuild/buf/cmd/buf \
       github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway \
       github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2 \
       google.golang.org/protobuf/cmd/protoc-gen-go \
       google.golang.org/grpc/cmd/protoc-gen-go-grpc

echo installing...
go install \
       github.com/bufbuild/buf/cmd/buf \
       github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway \
       github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2 \
       google.golang.org/protobuf/cmd/protoc-gen-go \
       google.golang.org/grpc/cmd/protoc-gen-go-grpc


echo tidiing up
go mod tidy

popd
