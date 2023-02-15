#
# Targets for building of Kiali from source.
#

## clean: Clean ${GOPATH}/bin/kiali, ${GOPATH}/pkg/*, ${OUTDIR}/docker and the kiali binary
clean:
	@echo Cleaning...
	@rm -f kiali
	@rm -rf ${GOPATH}/bin/kiali
	@[ -d ${GOPATH}/pkg ] && chmod -R +rw ${GOPATH}/pkg/* 2>/dev/null || true
	@rm -rf ${GOPATH}/pkg/*
	@rm -rf ${OUTDIR}/docker

## clean-ui: Removes the UI build/ and node_modules/ directories.
clean-ui:
	@echo Cleaning UI ...
	@rm -rf ${ROOTDIR}/frontend/node_modules
	@rm -rf ${ROOTDIR}/frontend/build

## clean-all: Runs `make clean` internally, removes the _output dir, and cleans the UI
clean-all: clean clean-ui
	@rm -rf ${OUTDIR}

## go-check: Check if the go version installed is supported by Kiali
go-check:
	@GO=${GO} hack/check_go_version.sh "${GO_VERSION_KIALI}"

## build: Runs `make go-check` internally and build Kiali binary
build: go-check
	@echo Building...
	${GO_BUILD_ENVVARS} ${GO} build \
		-o ${GOPATH}/bin/kiali -ldflags "-X main.version=${VERSION} -X main.commitHash=${COMMIT_HASH}"

## build-ui: Runs the yarn commands to build the frontend UI
build-ui:
	@cd ${ROOTDIR}/frontend && yarn install && yarn run build

## build-linux-multi-arch: Build Kiali binary with arch suffix for multi-arch
build-linux-multi-arch:
	@for arch in ${TARGET_ARCHS}; do \
		echo "Building for architecture [$${arch}]"; \
		${GO_BUILD_ENVVARS} GOOS=linux GOARCH=$${arch} ${GO} build \
			-o ${GOPATH}/bin/kiali-$${arch} -ldflags "-X main.version=${VERSION} -X main.commitHash=${COMMIT_HASH}"; \
	done

## install: Install missing dependencies. Runs `go install` internally
install:
	@echo Installing...
	${GO_BUILD_ENVVARS} ${GO} install \
		-ldflags "-X main.version=${VERSION} -X main.commitHash=${COMMIT_HASH}"

## format: Format all the files excluding vendor. Runs `gofmt` and `goimports` internally
format:
	@# Exclude more paths find . \( -path './vendor' -o -path <new_path_to_exclude> \) -prune -o -type f -iname '*.go' -print
	@for gofile in $$(find . -path './vendor' -prune -o -type f -iname '*.go' -print); do \
			${GOFMT} -w $$gofile; \
	done; \
	$(shell ./hack/fix_imports.sh)

## build-system-test: Building executable for system tests with code coverage enabled
build-system-test:
	@echo Building executable for system tests with code coverage enabled
	${GO} test -c -covermode=count -coverpkg $(shell ${GO} list ./... | grep -v test |  awk -vORS=, "{ print $$1 }" | sed "s/,$$//") \
	  -o ${GOPATH}/bin/kiali -ldflags "-X main.version=${VERSION} -X main.commitHash=${COMMIT_HASH}"

## test: Run tests, excluding third party tests under vendor and frontend. Runs `go test` internally
test:
	@echo Running tests, excluding third party tests under vendor
	${GO} test $(shell ${GO} list ./... | grep -v -e /vendor/ -e /frontend/)

## test-debug: Run tests in debug mode, excluding third party tests under vendor and frontend. Runs `go test -v`
test-debug:
	@echo Running tests in debug mode, excluding third party tests under vendor
	${GO} test -v $(shell ${GO} list ./... | grep -v -e /vendor/ -e /frontend/)

## test-race: Run tests with race detection, excluding third party tests under vendor and frontend. Runs `go test -race`
test-race:
	@echo Running tests with race detection, excluding third party tests under vendor
	${GO} test -race $(shell ${GO} list ./... | grep -v -e /vendor/ -e /frontend/)

## test-e2e-setup: Setup Python environment for running test suite
test-e2e-setup:
	@echo Setting up E2E tests
	cd tests/e2e && ./setup.sh

## test-e2e: Run E2E test suite
test-e2e:
	@echo Running E2E tests
	cd tests/e2e && source .kiali-e2e/bin/activate && pytest -s tests/

#
# Swagger Documentation
#

## swagger-install: Install swagger from github
swagger-install:
	@echo "Installing swagger binary to ${GOPATH}/bin..."
ifeq ($(GOARCH), ppc64le)
	curl https://github.com/go-swagger/go-swagger/archive/v${SWAGGER_VERSION}.tar.gz --create-dirs -Lo /tmp/v${SWAGGER_VERSION}.tar.gz && tar -xzf /tmp/v${SWAGGER_VERSION}.tar.gz -C /tmp/ && src_dir='pwd' && cd /tmp/go-swagger-${SWAGGER_VERSION} && ${GO} install ./cmd/swagger && cd ${src_dir}
else
	curl https://github.com/go-swagger/go-swagger/releases/download/v${SWAGGER_VERSION}/swagger_$(GOOS)_${GOARCH} --create-dirs -Lo ${GOPATH}/bin/swagger && chmod +x ${GOPATH}/bin/swagger
endif

## swagger-validate: Validate that swagger.json is correctly. Runs `swagger validate` internally
swagger-validate:
	@swagger validate ./swagger.json

## swagger-gen: Generate that swagger.json from Code. Runs `swagger generate` internally
swagger-gen:
	@swagger generate spec -o ./swagger.json
	@swagger generate markdown --quiet --spec ./swagger.json --output ./kiali_api.md

## swagger-serve: Serve the swagger.json in a website in local. Runs `swagger serve` internally
swagger-serve: swagger-validate
	@swagger serve ./swagger.json --no-open

## swagger-ci: Check that swagger.json is the correct one
swagger-ci: swagger-validate
	@swagger generate spec -o ./swagger_copy.json
	@cmp -s swagger.json swagger_copy.json; \
	RETVAL=$$?; \
	if [ $$RETVAL -ne 0 ]; then \
	  echo "swagger.json is not correct, remember to run make swagger-gen to update swagger.json"; exit 1; \
	fi

	@swagger generate markdown --quiet --spec ./swagger.json --output ./kiali_api_copy.md
	@cmp -s kiali_api.md kiali_api_copy.md; \
	RETVAL=$$?; \
	if [ $$RETVAL -ne 0 ]; then \
	  echo "kiali_api.md is not correct, remember to run make swagger-gen to update kiali_api.md"; exit 1; \
	fi

	rm swagger_copy.json kiali_api_copy.md

#
# Lint targets
#

## lint-install: Installs golangci-lint
lint-install:
	curl -sfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $$(${GO} env GOPATH)/bin v1.23.8

## lint: Runs golangci-lint
# doc.go is ommited for linting, because it generates lots of warnings.
lint:
	#golangci-lint run -c ./.github/workflows/config/.golangci.yml --skip-files "doc\.go" --tests --timeout 5m
