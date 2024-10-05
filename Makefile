include .env
export

.PHONY: help
help: ## display this help screen
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

.PHONY: mod
mod: ## go modules update & go mod tidy & go mod vendor
	@go get -u -t ./...
	@go mod tidy
	@go mod vendor

.PHONY: up
up: ## docker compose up
	@docker compose --project-name playground --file ./.docker/compose.yaml up -d

.PHONY: down
down: ## docker compose down
	@docker compose --project-name playground down --volumes

.PHONY: balus
balus: ## destroy everything about docker. (containers, images, volumes, networks.)
	@docker compose --project-name playground down --rmi all --volumes

.PHONY: goedit
goedit:
	@./script/goedit.sh
