# Makefile

# install-docker:
#     curl -fsSL https://get.docker.com -o get-docker.sh
#     sh get-docker.sh
export GO111MODULE ?= on
export GOPROXY ?= https://proxy.golang.org
export GOSUMDB ?= sum.golang.org
TEST_OUTPUT_FILE_PREFIX ?= ./test_report


build-image:
	docker build -t jupyterpython .

run-container:
	docker run -p 8080:8080 jupyterpython

# start all e2e tests
test-e2e-all:
	gotestsum --jsonfile $(TEST_OUTPUT_FILE_PREFIX)_e2e.json --junitfile $(TEST_OUTPUT_FILE_PREFIX)_e2e.xml --format standard-quiet -- -timeout 15m -p 2 -count=1 -v -tags=e2e ./tests/e2e/...