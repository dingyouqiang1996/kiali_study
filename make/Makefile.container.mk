#
# These targets build the containers without any cluster environment in mind.
# Instead, the containers built are tagged for publishing to quay.io and/or docker.io.
#

.prepare-kiali-image-files:
	@CONSOLE_VERSION=${CONSOLE_VERSION} CONSOLE_LOCAL_DIR=${CONSOLE_LOCAL_DIR} deploy/get-console.sh
	@echo Preparing container image files
	@mkdir -p ${OUTDIR}/docker
	@cp -r deploy/docker/* ${OUTDIR}/docker
	@cp ${GOPATH}/bin/kiali* ${OUTDIR}/docker

.download-operator-sdk-if-needed:
	@if [ "$(shell which operator-sdk 2>/dev/null || echo -n "")" == "" ]; then \
	  mkdir -p "${OUTDIR}/operator-sdk-install" ;\
	  if [ -x "${OUTDIR}/operator-sdk-install/operator-sdk" ]; then \
	    echo "You do not have operator-sdk installed in your PATH. Will use the one found here: ${OUTDIR}/operator-sdk-install/operator-sdk" ;\
	  else \
	    echo "You do not have operator-sdk installed in your PATH. The binary will be downloaded to ${OUTDIR}/operator-sdk-install/operator-sdk" ;\
	    curl -L https://github.com/operator-framework/operator-sdk/releases/download/v0.16.0/operator-sdk-v0.16.0-x86_64-linux-gnu > "${OUTDIR}/operator-sdk-install/operator-sdk" ;\
	    chmod +x "${OUTDIR}/operator-sdk-install/operator-sdk" ;\
	  fi ;\
	fi

.ensure-operator-sdk-exists: .download-operator-sdk-if-needed
	@$(eval OP_SDK ?= $(shell which operator-sdk 2>/dev/null || echo "${OUTDIR}/operator-sdk-install/operator-sdk"))
	@"${OP_SDK}" version

## container-build-kiali: Build Kiali container image.
container-build-kiali: .prepare-kiali-image-files
ifeq ($(DORP),docker)
	@echo Building container image for Kiali using docker
	docker build --pull -t ${QUAY_TAG} -f ${OUTDIR}/docker/${KIALI_DOCKER_FILE} ${OUTDIR}/docker
else
	@echo Building container image for Kiali using podman
	podman build --pull -t ${QUAY_TAG} -f ${OUTDIR}/docker/${KIALI_DOCKER_FILE} ${OUTDIR}/docker
endif

## container-build-operator: Build Kiali operator container image.
container-build-operator: .ensure-operator-repo-exists .ensure-operator-sdk-exists
	@echo Building container image for Kiali operator using operator-sdk
	cd "${ROOTDIR}/operator" && "${OP_SDK}" build --image-builder ${DORP} --image-build-args "--pull" "${OPERATOR_QUAY_TAG}"

## container-build: Build Kiali and Kiali operator container images
# On x86_64 machine, build both kiali and operator images.
ifeq ($(GOARCH),amd64)
container-build: container-build-kiali container-build-operator
# On other achitectures, only build kiali image.
else
container-build: container-build-kiali
endif

## container-push-kiali-quay: Pushes the Kiali image to quay.
container-push-kiali-quay:
ifeq ($(DORP),docker)
	@echo Pushing Kiali image to ${QUAY_TAG} using docker
	docker push ${QUAY_TAG}
else
	@echo Pushing Kiali image to ${QUAY_TAG} using podman
	podman push ${QUAY_TAG}
endif

## container-push: Pushes all container images to quay
container-push: container-push-kiali-quay

# Ensure "docker buildx" is available and enabled. For more details, see: https://github.com/docker/buildx/blob/master/README.md
.ensure-docker-buildx:
	@required_buildx_version="v0.4.2"; \
	current_buildx_version=$$(DOCKER_CLI_EXPERIMENTAL="enabled" docker buildx version); \
	if [[ "$${current_buildx_version}" == *"$${required_buildx_version}"* ]]; then \
	  echo "'docker buildx' is available and enabled."; \
	else \
		buildx_download_url=https://github.com/docker/buildx/releases/download/$${required_buildx_version}/buildx-$${required_buildx_version}.${GOOS}-${GOARCH}; \
		echo "You do not have 'docker buildx' installed. Will now download from $${buildx_download_url} and install it to [${HOME}/.docker/cli-plugins]."; \
		mkdir -p ${HOME}/.docker/cli-plugins; \
		curl -L --output ${HOME}/.docker/cli-plugins/docker-buildx "$${buildx_download_url}"; \
		chmod a+x ${HOME}/.docker/cli-plugins/docker-buildx; \
		DOCKER_CLI_EXPERIMENTAL="enabled" docker buildx version; \
		echo "'docker buildx' is available and enabled."; \
	fi

# Ensure a local builder for multi-arch build. For more details, see: https://github.com/docker/buildx/blob/master/README.md#building-multi-platform-images
.ensure-buildx-builder: .ensure-docker-buildx
	@if ! DOCKER_CLI_EXPERIMENTAL="enabled" docker buildx inspect kiali-builder > /dev/null 2>&1; then \
	  echo "The buildx builder instance named 'kiali-builder' does not exist. Creating one now."; \
	  if ! DOCKER_CLI_EXPERIMENTAL="enabled" docker buildx create --name=kiali-builder --driver-opt=image=moby/buildkit:v0.8.0-rc2; then \
	    echo "Failed to create the buildx builder 'kiali-builder'"; \
	    exit 1; \
	  fi \
	fi; \
	if [[ $$(uname -s) == "Linux" ]]; then \
	  echo "Ensuring QEMU is set up for this Linux host"; \
	  if ! docker run --privileged --rm tonistiigi/binfmt --install all; then \
	    echo "Failed to ensure QEMU is set up. This build will be allowed to continue, but it may fail at a later step."; \
	  fi \
	fi; \

## container-multi-arch-push-kiali-quay: Pushes the Kiali multi-arch image to quay.
container-multi-arch-push-kiali-quay: .ensure-buildx-builder .prepare-kiali-image-files
	@echo Pushing Kiali multi-arch image to ${QUAY_TAG} using docker buildx
	DOCKER_CLI_EXPERIMENTAL="enabled" docker buildx build --push --builder=kiali-builder $(foreach arch,${TARGET_ARCHS},--platform=linux/${arch}) $(foreach tag,${QUAY_TAG},--tag=${tag}) -f ${OUTDIR}/docker/Dockerfile-multi-arch ${OUTDIR}/docker
