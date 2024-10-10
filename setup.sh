#!/bin/bash

# Install go CLI tools
go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
go install honnef.co/go/tools/cmd/staticcheck@latest

# Create dotenv
cp -n .env.public .env

# Execute latest migration
echo "apply all up database migrations?"
make db/migrations/up
