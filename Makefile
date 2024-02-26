# Makefile

# install-docker:
#     curl -fsSL https://get.docker.com -o get-docker.sh
#     sh get-docker.sh
export GO111MODULE ?= on
export GOPROXY ?= https://proxy.golang.org
export GOSUMDB ?= sum.golang.org
TEST_OUTPUT_FILE_PREFIX ?= ./test_report


build-jupyterpython-image:
	docker build -t jupyterpython .

run-jupyterpython-container:
	docker run -p 6000:6000 jupyterpython

# start all e2e tests
test-e2e-all:
	go test ./tests/e2e/...

delete-jupyterpython-container:
	docker rmi -f jupyterpython
	CONTAINER_ID=$$(docker ps -a -q --filter ancestor=jupyterpython); \
	echo "Container ID: $$CONTAINER_ID"; \
	if [ -n "$$CONTAINER_ID" ]; then \
		docker rm -f $$CONTAINER_ID; \
	fi

delete-jupyterpython-image:
	docker rmi -f jupyterpython

build-perfapp-image:
	docker build -t perfapp:latest tests/perf/app/

run-perfapp-container: build-perfapp-image
	docker run --name perfapp-container -d --rm -p 8080:8080 -e ACCESS_TOKEN=$(ACCESS_TOKEN) perfapp:latest

delete-perfapp-container:
	docker rm -f perfapp-container

install-perf-deps:
	apt install hey

run-perf-test: install-perf-deps
	hey -n 5 -c 5 -m POST -T 'application/json' -d '{"code":"1+2"}' http://localhost:8080/execute
	