include .env

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
	staticcheck ./...
	@echo "Running tests..."
	go test -race -vet=off ./...
	
## build: build the cmd/api application
.PHONY: build
build:
	@echo "Building cmd/api..."
	go build -ldflags="-s" -o=./bin/api ./cmd/api

## run: run the cmd/api application
.PHONY: run
run:
	go run ./cmd/api -port=4000 -dev \
		-url=${API_BASE_URL} \
		-db-dsn=${DATABASE_URL} \
		-smtp-host=${API_SMTP_HOST} \
		-smtp-port=${API_SMTP_PORT} \
		-smtp-username=${API_SMTP_USERNAME} \
		-smtp-password=${API_SMTP_PASSWORD} \
		-smtp-sender=${API_SMTP_SENDER} \
		-limiter-enabled=false

## db/psql: connect to the database using psql
.PHONY: db/psql
db/psql:
	psql ${DATABASE_URL}

## db/migrations/new label=$1: create a new database migration
.PHONY: db/migrations/new
db/migrations/new:
	@echo "Creating migration files for ${label}..."
	migrate create -seq -ext=.sql -dir=./migrations ${label}

## db/migrations/up: apply all up database migrations
.PHONY: db/migrations/up
db/migrations/up: confirm
	@echo "Running up migrations..."
	migrate -path ./migrations -database ${DATABASE_URL} up

## db/migrations/drop: drop the entire databse schema
.PHONY: db/migrations/drop
db/migrations/drop: confirm
	@echo "Dropping the entire database schema..."
	migrate -path ./migrations -database ${DATABASE_URL} drop
