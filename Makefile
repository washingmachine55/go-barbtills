GOOSE_DRIVER := postgres
GOOSE_MIGRATION_DIR := ./database/migrations

# Read DB_URL from the app config so migrations do not depend on the app compiling.
# Override with: DB_URL=postgres://... make migrate up
DB_URL ?= $(shell awk -F'"' '/^[[:space:]]*DB_URL/ {print $$2; exit}' $(HOME)/.config/barbtils/config.toml)

# Extract everything after the first target word as arguments
ARGS := $(wordlist 2,$(words $(MAKECMDGOALS)),$(MAKECMDGOALS))

# Turn arguments into dummy targets so Make doesn't throw a "No rule to make target" error
$(eval $(ARGS):;@:)

.PHONY: migrate sqlc build vet test check tidy

migrate:
	go tool goose $(GOOSE_DRIVER) "$(DB_URL)" -dir $(GOOSE_MIGRATION_DIR) $(ARGS)

sqlc:
	sqlc generate

vet:
	go vet ./...

build:
	go build -o barbtils .

test:
	go test ./...

tidy:
	go mod tidy

check: sqlc vet build test
