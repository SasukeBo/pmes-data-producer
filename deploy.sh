#!/bin/bash

echo "change work directory to" $GOPATH/src/github.com/SasukeBo/pmes-data-producer ...
cd $GOPATH/src/github.com/SasukeBo/pmes-data-producer

echo "start service ..."
go run server.go
