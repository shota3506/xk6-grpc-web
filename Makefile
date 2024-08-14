MAKEFLAGS += --silent

## help: Prints a list of available build targets.
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
