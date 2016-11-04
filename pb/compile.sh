#!/usr/bin/env sh

# Install proto3 from source
#  brew install autoconf automake libtool
#  git clone https://github.com/google/protobuf
#  ./autogen.sh ; ./configure ; make ; make install
#
# Update protoc Go bindings via
#  go get -u github.com/golang/protobuf/{proto,protoc-gen-go}
#
# See also
#  https://github.com/grpc/grpc-go/tree/master/examples

google_dep="Mgoogle/protobuf/timestamp.proto=github.com/golang/protobuf/ptypes/timestamp,Mgoogle/protobuf/any.proto=github.com/golang/protobuf/ptypes/any,Mgoogle/protobuf/proto=github.com/golang/protobuf/proto"

#protoc init.proto --go_out=plugins=grpc,$google_dep:.
#protoc seal.proto --go_out=plugins=grpc,$google_dep:.

protoc vault.proto --go_out=plugins=grpc,$google_dep:.
