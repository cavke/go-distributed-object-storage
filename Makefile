# -----------------------------------------------------------------
#				Main targets
# -----------------------------------------------------------------

.PHONY: build
## build: builds the application
build:
	go build cmd/main.go

.PHONY: run
## run: runs go run main.go
run: build
	go run cmd/main.go

.PHONY: test
## tests: runs go tests with default values
test:
	go test -v -cover ./internal/...

.PHONY: clean
## clean: cleans the binary
clean:
	go clean

.PHONY: lint
## lint: runs linter
lint:
	golangci-lint run

# -----------------------------------------------------------------
#				Docker targets
# -----------------------------------------------------------------

.PHONY: docker-up
## docker-up starts the program instances
docker-up:
	docker compose up --build

.PHONY: docker-down
## docker-down stop program instances
docker-down:
	docker compose down

.PHONY: docker-cycle
## docker-cycle removes gateway container and starts the program instances
docker-cycle:
	docker compose rm --stop --force gateway-container && docker compose up --build

# -----------------------------------------------------------------
#			 Help
# -----------------------------------------------------------------
.PHONY: help
## help: Prints this help message
help:
	@echo "Usage: \n"
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' |	sed -e 's/^/ /'

