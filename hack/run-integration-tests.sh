#!/bin/bash

infomsg() {
  echo "[INFO] ${1}"
}

ISTIO_VERSION=""
TEST_SUITE="backend"
SETUP_ONLY="false"

# process command line args
while [[ $# -gt 0 ]]; do
  key="$1"
  case $key in
    -iv|--istio-version)
      ISTIO_VERSION="${2}"
      shift;shift
      ;;
    -ts|--test-suite)
      TEST_SUITE="${2}"
      if [ "${TEST_SUITE}" != "backend" -a "${TEST_SUITE}" != "frontend" -a "${TEST_SUITE}" != "frontend-multi-cluster" ]; then
        echo "--test-suite option must be one of 'backend', 'frontend', or 'frontend-multi-cluster'"
        exit 1
      fi
      shift;shift
      ;;
    -so|--setup-only)
      SETUP_ONLY="${2}"
      if [ "${SETUP_ONLY}" != "true" -a "${SETUP_ONLY}" != "false" ]; then
        echo "--setup-only option must be one of 'true' or 'false'"
        exit 1
      fi
      shift;shift
      ;;
    -h|--help)
      cat <<HELPMSG
Valid command line arguments:
  -iv|--istio-version <version>
    Which Istio version to test with. For releases, specify "#.#.#". For dev builds, specify in the form "#.#-dev"
    Default: The latest release
  -ts|--test-suite <backend|frontend|frontend-multi-cluster>
    Which test suite to run.
    Default: backend
  -so|--setup-only <true|false>
    If true, only setup the test environment and exit without running the tests.
    Default: false
  -h|--help:
    This message

NOTE: When running the multi-cluster tests locally, it might be necessary to
edit some kernel settings to allow for the kind clusters to be created.

The following settings added to your sysctl config file should work (the filename will be something like '/etc/sysctl.d/local.conf' - refer to your operating system 'man sysctl' docs to determine which file should be changed):
fs.inotify.max_user_watches=524288
fs.inotify.max_user_instances=512
HELPMSG
      exit 1
      ;;
    *)
      echo "ERROR: Unknown argument [$key]. Aborting."
      exit 1
      ;;
  esac
done

# print out our settings for debug purposes
cat <<EOM
=== SETTINGS ===
ISTIO_VERSION=$ISTIO_VERSION
SETUP_ONLY=$SETUP_ONLY
TEST_SUITE=$TEST_SUITE
=== SETTINGS ===
EOM

set -e

if [ -n "${ISTIO_VERSION}" ]; then
  ISTIO_VERSION_ARG="--istio-version ${ISTIO_VERSION}"
else
  ISTIO_VERSION_ARG=""
fi

# Determine where this script is and make it the cwd
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" &> /dev/null && pwd)"

ensureCypressInstalled() {
  cd "${SCRIPT_DIR}"/../frontend
  if ! yarn cypress --help &> /dev/null; then
    echo "cypress binary was not detected in your PATH. Did you install the frontend directory? Before running the frontend tests you must run 'make build-ui'."
    exit 1
  fi
  cd -
}

ensureKialiServerReady() {
  local KIALI_URL="$1"

  infomsg "Waiting for Kiali server pods to be healthy"
  kubectl rollout status deployment/kiali -n istio-system --timeout=120s

  # Ensure the server is responding to health checks externally.
  # It can take a minute for the Kube service and ingress to sync
  # and wire up the endpoints.
  infomsg "Waiting for Kiali server to respond externally to health checks"
  local start_time=$(date +%s)
  local end_time=$((start_time + 30))
  while true; do
    if curl -k -s --fail "${KIALI_URL}/healthz"; then
      break
    fi
    local now=$(date +%s)
    if [ "${now}" -gt "${end_time}" ]; then
      echo "Timed out waiting for Kiali server to respond to health checks"
      exit 1
    fi
    sleep 1
  done
}

ensureKialiTracesReady() {
  local KIALI_URL="$1"

  infomsg "Waiting for Kiali to have traces"
  local start_time=$(date +%s)
  local end_time=$((start_time + 60))

  # Get traces from the last 5m
  local traces_date=$((($(date +%s) - 300) * 1000))
  local trace_url="${KIALI_URL}/api/namespaces/bookinfo/workloads/productpage-v1/traces?startMicros=${traces_date}&tags=&limit=100"
  infomsg "Traces url: ${trace_url}"
  while true; do
    result=$(curl -k -s --fail "$trace_url" \
        -H 'Accept: application/json, text/plain, */*' \
        -H 'Content-Type: application/json' | jq -r '.data')

    if [ "$result" == "[]" ]; then
      local now=$(date +%s)
      if [ "${now}" -gt "${end_time}" ]; then
        echo "Timed out waiting for Kiali to get any trace"
        break
      fi
      sleep 1
    else
      break
    fi

  done
}

infomsg "Running ${TEST_SUITE} integration tests"
if [ "${TEST_SUITE}" == "backend" ]; then
  "${SCRIPT_DIR}"/setup-kind-in-ci.sh ${ISTIO_VERSION_ARG}

  ISTIO_INGRESS_IP="$(kubectl get svc istio-ingressgateway -n istio-system -o=jsonpath='{.status.loadBalancer.ingress[0].ip}')"

  # Install demo apps
  "${SCRIPT_DIR}"/istio/install-testing-demos.sh -c "kubectl" -g "${ISTIO_INGRESS_IP}"

  URL="http://${ISTIO_INGRESS_IP}/kiali"
  echo "kiali_url=$URL"
  export URL

  ensureKialiServerReady "${URL}"

  if [ "${SETUP_ONLY}" == "true" ]; then
    exit 0
  fi

  # Run backend integration tests
  cd "${SCRIPT_DIR}"/../tests/integration/tests
  go test -v -failfast
elif [ "${TEST_SUITE}" == "frontend" ]; then
  ensureCypressInstalled
  "${SCRIPT_DIR}"/setup-kind-in-ci.sh --auth-strategy token ${ISTIO_VERSION_ARG}

  ISTIO_INGRESS_IP="$(kubectl get svc istio-ingressgateway -n istio-system -o=jsonpath='{.status.loadBalancer.ingress[0].ip}')"
  # Install demo apps
  "${SCRIPT_DIR}"/istio/install-testing-demos.sh -c "kubectl" -g "${ISTIO_INGRESS_IP}"

  # Get Kiali URL
  KIALI_URL="http://${ISTIO_INGRESS_IP}/kiali"
  export CYPRESS_BASE_URL="${KIALI_URL}"
  export CYPRESS_NUM_TESTS_KEPT_IN_MEMORY=0
  # Recorded video is unusable due to low resources in CI: https://github.com/cypress-io/cypress/issues/4722
  export CYPRESS_VIDEO=false

  ensureKialiServerReady "${KIALI_URL}"
  ensureKialiTracesReady "${KIALI_URL}"

  if [ "${SETUP_ONLY}" == "true" ]; then
    exit 0
  fi

  cd "${SCRIPT_DIR}"/../frontend
  yarn run cypress:run
elif [ "${TEST_SUITE}" == "frontend-multi-cluster" ]; then
  ensureCypressInstalled
  "${SCRIPT_DIR}"/setup-kind-in-ci.sh --multicluster "true" ${ISTIO_VERSION_ARG}

  # Get Kiali URL
  KIALI_URL="http://$(kubectl --context kind-east get svc istio-ingressgateway -n istio-system -o=jsonpath='{.status.loadBalancer.ingress[0].ip}')/kiali"
  export CYPRESS_BASE_URL="${KIALI_URL}"
  export CYPRESS_NUM_TESTS_KEPT_IN_MEMORY=0
  # Recorded video is unusable due to low resources in CI: https://github.com/cypress-io/cypress/issues/4722
  export CYPRESS_VIDEO=false

  if [ "${SETUP_ONLY}" == "true" ]; then
    exit 0
  fi

  ensureKialiServerReady "${KIALI_URL}"

  cd "${SCRIPT_DIR}"/../frontend
  yarn run cypress:run:multi-cluster
fi
