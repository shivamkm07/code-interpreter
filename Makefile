# Makefile

# install-docker:
#     curl -fsSL https://get.docker.com -o get-docker.sh
#     sh get-docker.sh
export GO111MODULE ?= on
export GOPROXY ?= https://proxy.golang.org
export GOSUMDB ?= sum.golang.org
TEST_OUTPUT_FILE_PREFIX ?= ./test_report
K6_VERSION ?= 0.49.0



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
	docker run --name perfapp-container -d --rm -p 8080:8080 -e AZURE_TENANT_ID=$(AZURE_TENANT_ID) -e AZURE_CLIENT_ID=$(AZURE_CLIENT_ID) -e AZURE_CLIENT_SECRET=$(AZURE_CLIENT_SECRET) perfapp:latest

delete-perfapp-container:
	docker rm -f perfapp-container

install-perf-deps:
	# curl https://hey-release.s3.us-east-2.amazonaws.com/hey_linux_amd64 -o hey
	# chmod +x hey
	wget https://github.com/grafana/k6/releases/download/v$(K6_VERSION)/k6-v$(K6_VERSION)-linux-amd64.tar.gz
	tar -xvf k6-v$(K6_VERSION)-linux-amd64.tar.gz --strip-components=1
	rm k6-v$(K6_VERSION)-linux-amd64.tar.gz
	chmod +x k6


run-perf-test: install-perf-deps
	# ./hey -n 5 -c 5 -m POST -T 'application/json' -d '{"code":"1+2"}' http://localhost:8080/execute
	./k6 run -e TEST_START_TIME="$(shell date -Iseconds)" -e RUN_ID=$(GITHUB_RUN_ID) tests/perf/test.js -e SCENARIO_TYPE=$(SCENARIO_TYPE) -e QPS=$(QPS) -e DURATION=$(DURATION) -e REGION=$(REGION) --summary-trend-stats="min,max,med,avg,p(90),p(95),p(98),p(99),p(99.9)"
