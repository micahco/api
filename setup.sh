#!/bin/bash

go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
go install honnef.co/go/tools/cmd/staticcheck@latest
go install github.com/rakyll/hey@latest

cp -n .env.public .env
source .env

migrate -path ./migrations -database $DATABASE_URL up

make audit
