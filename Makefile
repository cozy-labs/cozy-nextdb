# # Some interesting links on Makefiles:
# https://danishpraka.sh/2019/12/07/using-makefiles-for-go.html
# https://tech.davis-hansson.com/p/make/

MAKEFLAGS += --warn-undefined-variables
MAKEFLAGS += --no-builtin-rules
SHELL := bash
.ONESHELL:
.SHELLFLAGS := -eu -o pipefail -c
.DELETE_ON_ERROR:

ifeq ($(origin .RECIPEPREFIX), undefined)
        $(error This Make does not support .RECIPEPREFIX. Please use GNU Make 4.0 or later)
endif
.RECIPEPREFIX = >


## install: compile the code and installs in binary in $GOPATH/bin
install:
> @go install
.PHONY: install

## build: compile the code
build:
> @go build
.PHONY: build

## ci: check before commiting, as we don't have a true CI yet
ci: install lint test cli

## serve: start the cozy-nextdb web server for local development
serve:
> @go run . serve --log-level=debug
.PHONY: serve

## work: start the cozy-nextdb workers for local development
work:
> @go run . work --dev --log-level=debug
.PHONY: work

## lint: enforce a consistent code style and detect code smells
lint: scripts/golangci-lint
> @scripts/golangci-lint run -E gofmt -E unconvert -E misspell -E whitespace -E exportloopref -E bidichk -E gocritic -E bodyclose
.PHONY: lint

scripts/golangci-lint: Makefile
> @curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b ./scripts v1.58.1

## cli: build the CLI documentation and shell completions
cli: install
> @rm -rf docs/cli/*
> @cozy-nextdb doc markdown docs/cli
> @cozy-nextdb completion bash > scripts/completion/cozy-nextdb.bash
> @cozy-nextdb completion zsh > scripts/completion/cozy-nextdb.zsh
> @cozy-nextdb completion fish > scripts/completion/cozy-nextdb.fish
.PHONY: cli

## test: run the automated tests
test:
> @go test -shuffle on -timeout 1m ./...
.PHONY: test

## clean: clean the generated files and directories
clean:
> @go clean
.PHONY: clean

## help: print this help message
help:
> @echo "Usage:"
> @sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' |  sed -e 's/^/ /'
.PHONY: help
