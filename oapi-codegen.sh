#!/bin/bash

# "types", "client", "chi-server", "server", "gin", "gorilla", "spec", "skip-fmt", "skip-prune", "fiber", "iris", "std-http". (default "types,client,server,spec")
DIR=internal/adapters/in/rest/openapi
PACKAGE=openapi
SPEC=internal/app/docs/openapi.yaml

# models once
oapi-codegen -generate types      -o $DIR/types.gen.go   -package $PACKAGE $SPEC

# server + different response suffix
oapi-codegen -generate chi-server -o $DIR/server.gen.go  -package $PACKAGE $SPEC
# client + same different response suffix
oapi-codegen -generate client     -o $DIR/client.gen.go  -package $PACKAGE $SPEC

# (optional) embed the spec
oapi-codegen -generate spec       -o $DIR/spec.gen.go    -package $PACKAGE $SPEC
