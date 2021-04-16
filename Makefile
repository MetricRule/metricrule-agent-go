all: bin/proxy
test: unit-test

DOCKER_BUILDKIT=1
PLATFORM=local

.PHONY: bin/proxy
bin/proxy:
	@docker build . --target bin --output bin/ --platform ${PLATFORM}

.PHONY: unit-test
unit-test:
	@docker build . --target unit-test

.PHONY: lint
lint:
	@docker build . --target lint

.EXPORT_ALL_VARIABLES: