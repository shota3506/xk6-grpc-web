MAKEFLAGS += --silent

all: clean lint test build

## help: Prints a list of available build targets.
.PHONY: help
help:
	echo "Usage: make <OPTIONS> ... <TARGETS>"

## build: Builds a custom 'k6' with the local extension.
.PHONY: build
build:
	xk6 build --with $(shell go list -m)=.

## test: Executes any tests.
.PHONY: test
test:
	echo "Running tests..."
	go test --shuffle on -race ./...

# lint: Runs the linters.
.PHONY: lint
lint:
	echo "Running linters..."
	go vet ./...

## clean: Removes any previously created artifacts.
.PHONY: clean
clean:
	echo "Cleaning up..."
	rm -f ./k6

## run-server: Starts the example server.
.PHONY: run-server
run-server:
	docker compose -f examples/server/compose.yaml up -d

## stop-server: Stops the example server.
.PHONY: stop-server
stop-server:
	docker compose -f examples/server/compose.yaml down
