MAKEFLAGS += --silent

## help: Prints a list of available build targets.
help:
	echo "Usage: make <OPTIONS> ... <TARGETS>"

## build: Builds a custom 'k6' with the local extension.
build:
	xk6 build --with $(shell go list -m)=.
