#-----------------------------------------------------------------------------
# configuration - see also 'make help' for list of targets
#-----------------------------------------------------------------------------

# name of container
CONTAINER_NAME = swatto/promtotwilio
CONTAINER_VERSION = 1.1

# name of instance and other options you want to pass to docker run for testing
INSTANCE_NAME = promtotwilio
RUN_OPTS = -p 9090:9090 --env-file ./.env

#-----------------------------------------------------------------------------
# default target
#-----------------------------------------------------------------------------

all	 : ## Build the container - this is the default action
all: build

#-----------------------------------------------------------------------------
# get dependencies
#-----------------------------------------------------------------------------

deps : ## Get go depdencies
deps:
	@go get github.com/valyala/fasthttp
	@go get github.com/Sirupsen/logrus
	@go get github.com/buger/jsonparser
	@go get github.com/carlosdp/twiliogo

#-----------------------------------------------------------------------------
# build container
#-----------------------------------------------------------------------------

.built: .
	@CGO_ENABLED=0 GOOS=linux go build -ldflags "-s" -a -installsuffix cgo -o promtotwilio .
	docker build -t $(CONTAINER_NAME):latest -t $(CONTAINER_NAME):$(CONTAINER_VERSION) .
	@docker inspect -f '{{.Id}}' $(CONTAINER_NAME):$(CONTAINER_VERSION) > .built
	@go clean

build	: ## build the container
build: deps .built

clean	: ## delete the image from docker
clean: stop
	@$(RM) .built
	-docker rmi $(CONTAINER_NAME):$(CONTAINER_VERSION)

re	: ## clean and rebuild
re: clean all

#-----------------------------------------------------------------------------
# repository control
#-----------------------------------------------------------------------------

push	: ## Push container to remote repository
push: build
	docker push $(CONTAINER_NAME):$(CONTAINER_VERSION)
	docker push $(CONTAINER_NAME):latest

pull	: ## Pull container from remote repository
pull:
	docker pull $(CONTAINER_NAME):$(CONTAINER_VERSION)

#-----------------------------------------------------------------------------
# test container
#-----------------------------------------------------------------------------

test : ## Run tests
test:
	go test .

run	 : ## Run the container as a daemon locally for testing
run: build stop
	docker run -d --name=$(INSTANCE_NAME) $(RUN_OPTS) $(CONTAINER_NAME)

stop	: ## Stop local test started by run
stop:
	-docker stop $(INSTANCE_NAME)
	-docker rm $(INSTANCE_NAME)

#-----------------------------------------------------------------------------
# supporting targets
#-----------------------------------------------------------------------------

help	: ## Show this help.
	@fgrep -h "##" $(MAKEFILE_LIST) | fgrep -v fgrep | sed -e 's/\\$$//' | sed -e 's/##//'

.PHONY : all build clean re push pull run stop help