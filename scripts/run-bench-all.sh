#!/bin/bash

cd "$( dirname "${BASH_SOURCE[0]}" )/.."

echo "##### Downloading test software (hey) and building artifacts #####"
go get -u github.com/rakyll/hey
go install github.com/ory/hydra
go build -buildmode=plugin -o=datastore.so ./plugin
go build -o=hydragcp ./example

# OPAQUE vs JWT
echo "##### Memory Benchmark (opaque tokens) #####"
DATABASE_URL=memory OAUTH2_ACCESS_TOKEN_STRATEGY=opaque ./scripts/run-bench.sh hydra
echo "##### Memory Benchmark (jwt tokens) #####"
DATABASE_URL=memory OAUTH2_ACCESS_TOKEN_STRATEGY=jwt ./scripts/run-bench.sh hydra

# PostgreSQL - passed in by caller
if [ "$POSTGRES_URL" = "" ]
then
  echo "##### Skipping PostgreSQL Benchmark since $POSTGRES_URL is missing or emtpy #####"
else
    echo "##### PostgreSQL Benchmark (opaque tokens) #####"
    DATABASE_URL=$POSTGRES_URL OAUTH2_ACCESS_TOKEN_STRATEGY=opaque ./scripts/run-bench.sh hydra
fi

# Datastore - passed in by caller
echo "##### Datastore Benchmark (opaque tokens) #####"
DATABASE_URL=datastore://?namespace=opaque OAUTH2_ACCESS_TOKEN_STRATEGY=opaque DATABASE_PLUGIN=datastore.so ./scripts/run-bench.sh hydra

# IAM API
echo "##### Memory Benchmark (IAM API jwt tokens) #####"
DATABASE_URL=memory OAUTH2_ACCESS_TOKEN_STRATEGY=iam ./scripts/run-bench.sh ./hydragcp
echo "##### Datastore Benchmark (IAM API jwt tokens) #####"
DATABASE_URL=datastore://?namespace=iamapi OAUTH2_ACCESS_TOKEN_STRATEGY=iam DATABASE_PLUGIN=datastore.so ./scripts/run-bench.sh ./hydragcp
