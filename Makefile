#-----------------------------------------------------------------------------
# configuration - see also 'make help' for list of targets
#-----------------------------------------------------------------------------

# name of container
CONTAINER_NAME = swatto/promtotwilio

# name of instance and other options you want to pass to docker run for testing
INSTANCE_NAME = promtotwilio
RUN_OPTS = -p 9090:9090 --env-file ./.env

# Go parameters
GOCMD = go
GOBUILD = $(GOCMD) build
GOTEST = $(GOCMD) test
GOFMT = $(GOCMD) fmt
GOVET = $(GOCMD) vet
BINARY_NAME = promtotwilio
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS = -ldflags "-X main.Version=$(VERSION)"

#-----------------------------------------------------------------------------
# default target
#-----------------------------------------------------------------------------

all: ## Build the container - this is the default action
all: build-docker

#-----------------------------------------------------------------------------
# Go targets
#-----------------------------------------------------------------------------

build: ## Build the Go binary
	$(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME) ./cmd/promtotwilio

build-all: ## Build binaries for all platforms (used by CI release)
	@mkdir -p dist
	GOOS=linux GOARCH=amd64 $(GOBUILD) -ldflags="-s -w -X main.Version=$(VERSION)" -o dist/$(BINARY_NAME)-linux-amd64 ./cmd/promtotwilio
	GOOS=linux GOARCH=arm64 $(GOBUILD) -ldflags="-s -w -X main.Version=$(VERSION)" -o dist/$(BINARY_NAME)-linux-arm64 ./cmd/promtotwilio
	GOOS=darwin GOARCH=amd64 $(GOBUILD) -ldflags="-s -w -X main.Version=$(VERSION)" -o dist/$(BINARY_NAME)-darwin-amd64 ./cmd/promtotwilio
	GOOS=darwin GOARCH=arm64 $(GOBUILD) -ldflags="-s -w -X main.Version=$(VERSION)" -o dist/$(BINARY_NAME)-darwin-arm64 ./cmd/promtotwilio

test: ## Run tests
	$(GOTEST) -v -race ./...

coverage: ## Run tests with coverage
	$(GOTEST) -v -race -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

fmt: ## Format code
	$(GOFMT) ./...

vet: ## Run go vet
	$(GOVET) ./...

lint: ## Run golangci-lint
	golangci-lint run

#-----------------------------------------------------------------------------
# Docker targets
#-----------------------------------------------------------------------------

build-docker: ## Build the Docker container
	docker build -t $(CONTAINER_NAME):latest .

clean: ## Delete the image from docker
clean: stop
	-docker rmi $(CONTAINER_NAME):latest
	-rm -f $(BINARY_NAME) coverage.out coverage.html
	-rm -rf dist

re: ## Clean and rebuild
re: clean all

run: ## Run the container as a daemon locally for testing
run: build-docker stop
	docker run -it --rm --name=$(INSTANCE_NAME) $(RUN_OPTS) $(CONTAINER_NAME)

stop: ## Stop local test started by run
	-docker stop $(INSTANCE_NAME)
	-docker rm $(INSTANCE_NAME)

e2e: ## Run E2E tests using docker-compose
	@echo "Building E2E test containers..."
	docker compose -f docker-compose.e2e.yml build
	@echo "Running E2E tests..."
	docker compose -f docker-compose.e2e.yml run --rm e2e-tests; \
	EXIT_CODE=$$?; \
	if [ $$EXIT_CODE -ne 0 ]; then \
		echo "E2E tests failed, showing logs..."; \
		docker compose -f docker-compose.e2e.yml logs; \
	fi; \
	echo "Cleaning up..."; \
	docker compose -f docker-compose.e2e.yml down -v --remove-orphans; \
	exit $$EXIT_CODE

#-----------------------------------------------------------------------------
# Development targets
#-----------------------------------------------------------------------------

dev: ## Run locally for development
	$(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME) ./cmd/promtotwilio
	./$(BINARY_NAME)

check: fmt vet lint test ## Run all checks (fmt, vet, lint, test)

#-----------------------------------------------------------------------------
# supporting targets
#-----------------------------------------------------------------------------

help: ## Show this help.
	@fgrep -h "##" $(MAKEFILE_LIST) | fgrep -v fgrep | sed -e 's/\\$$//' | sed -e 's/##//'

.PHONY: all build build-all test coverage fmt vet lint build-docker clean re run stop e2e dev check help
