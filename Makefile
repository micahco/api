## help: print this help message
.PHONY: help
help:
	@echo "Usage:"
	@sed -n "s/^##//p" ${MAKEFILE_LIST} | column -t -s ":" |  sed -e "s/^/ /"

# confirmation dialog helper
.PHONY: confirm
confirm:
	@echo -n "Are you sure? [y/N] " && read ans && [ $${ans:-N} = y ]

## audit: tidy dependencies and format, vet and test all code
.PHONY: audit
audit:
	@echo "Tidying and verifying module dependencies..."
	go mod tidy
	go mod verify
	@echo "Formatting code..."
	go fmt ./...
	@echo "Vetting code..."
	go vet ./...
	go tool staticcheck ./...
	@echo "Running tests..."
	go test -race -vet=off ./...
	
## build: build the cmd/api application
current_time = $(shell date --iso-8601=seconds)
git_description = $(shell git describe --always --dirty --tags --long;)
linker_flags = '-s -X main.buildTime=${current_time} -X main.version=${git_description}'

.PHONY: build
build:
	@echo "Building cmd/api..."
	go build -ldflags=${linker_flags} -o=./bin/api ./cmd/api

## run: run the cmd/api application
.PHONY: run
run:
	go run ./cmd/api -dev

## db/psql: connect to the database using psql
.PHONY: db/psql
db/psql:
	psql ${DATABASE_URL}

## db/migrations/new label=$1: create a new database migration
.PHONY: db/migrations/new
db/migrations/new:
	@echo "Creating migration files for ${label}..."
	go tool goose -s create ${label} sql

## db/migrations/up: apply all up database migrations
.PHONY: db/migrations/up
db/migrations/up: confirm
	@echo "Running up migrations..."
	go tool goose up

## db/migrations/reset: drop the entire databse schema
.PHONY: db/migrations/reset
db/migrations/reset:
	@echo "Dropping the entire database schema..."
	go tool goose -s down-to 0
