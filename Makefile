#-----------------------------------------------------------------------------
# configuration - see also 'make help' for list of targets
#-----------------------------------------------------------------------------

# name of container
CONTAINER_NAME = swatto/promtotwilio

# name of instance and other options you want to pass to docker run for testing
INSTANCE_NAME = promtotwilio
RUN_OPTS = -p 9090:9090 --env-file ./.env

#-----------------------------------------------------------------------------
# default target
#-----------------------------------------------------------------------------

all	 : ## Build the container - this is the default action
all: build

#-----------------------------------------------------------------------------
# build container
#-----------------------------------------------------------------------------

build   : ## build the container
	docker build -t $(CONTAINER_NAME):latest .

clean	: ## delete the image from docker
clean: stop
	-docker rmi $(CONTAINER_NAME):latest

re	: ## clean and rebuild
re: clean all

#-----------------------------------------------------------------------------
# test container
#-----------------------------------------------------------------------------

test : ## Run tests
test:
	go test .

run	 : ## Run the container as a daemon locally for testing
run: build stop
	docker run -it  --rm --name=$(INSTANCE_NAME) $(RUN_OPTS) $(CONTAINER_NAME)

stop	: ## Stop local test started by run
stop:
	-docker stop $(INSTANCE_NAME)
	-docker rm $(INSTANCE_NAME)

#-----------------------------------------------------------------------------
# supporting targets
#-----------------------------------------------------------------------------

help	: ## Show this help.
	@fgrep -h "##" $(MAKEFILE_LIST) | fgrep -v fgrep | sed -e 's/\\$$//' | sed -e 's/##//'

.PHONY : all build clean re run stop help
