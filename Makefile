GOOSE_DRIVER := postgres
GOOSE_MIGRATION_DIR := ./database/migrations

# Extract everything after the first target word as arguments
ARGS := $(wordlist 2,$(words $(MAKECMDGOALS)),$(MAKECMDGOALS))

# Turn arguments into dummy targets so Make doesn't throw a "No rule to make target" error
$(eval $(ARGS):;@:)

.PHONY: migrate migrate-rag

migrate:
	go tool goose $(GOOSE_DRIVER) `go run main.go --get_db_uri` -dir $(GOOSE_MIGRATION_DIR)/standard $(ARGS)

migrate-rag:
	go tool goose $(GOOSE_DRIVER) `go run main.go --get_rag_db_uri` -dir $(GOOSE_MIGRATION_DIR)/rag_knowledge_base $(ARGS)
