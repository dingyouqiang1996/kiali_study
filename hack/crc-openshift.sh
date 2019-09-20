#!/bin/bash

##############################################################################
# crc-openshift.sh
#
# Run this script to start/stop an OpenShift 4 cluster.
# This can also optionally install Maistra/Istio.
#
# This script takes one command whose value is one of the following:
#        start: starts the OpenShift environment
#         stop: stops the OpenShift environment
#       delete: deletes the OpenShift environment removing persisted data
#       status: outputs the current status of the OpenShift environment
#          ssh: logs into the CRC VM via ssh so you can probe in the VM
#       routes: outputs all known route URLs
#     services: outputs all known service endpoints (excluding internal openshift services)
#   sm-install: installs service mesh into the cluster
# sm-uninstall: removes all service mesh components
#   bi-install: installs bookinfo demo into the cluster
#
# This script accepts several options - see --help for details.
#
# This script utilizes the crc tool and its bundle.
# If you do not have it, this script will download it for you.
# Downloading will take some time, the images are large.
#
##############################################################################

infomsg() {
  echo "HACK: $1"
}

debug() {
  if [ "$_VERBOSE" == "true" ]; then
    infomsg "DEBUG: $1"
  fi
}

get_downloader() {
  if [ -z "$DOWNLOADER" ] ; then
    # Use wget command if available, otherwise try curl
    if which wget > /dev/null 2>&1 ; then
      DOWNLOADER="wget -O"
    else
      if which curl > /dev/null 2>&1 ; then
        DOWNLOADER="curl -L -o"
      fi
    fi

    if [ ! "$DOWNLOADER" ] ; then
      infomsg "ERROR: You must install either curl or wget to allow downloading."
      exit 1
    fi
  fi
  debug "Downloader command to be used: ${DOWNLOADER}"
}

get_installer() {
  if [ -z "$INSTALLER" ] ; then
    # Use dnf command if available, otherwise try yum
    if which dnf > /dev/null 2>&1 ; then
      INSTALLER="sudo dnf"
    else
      if which yum > /dev/null 2>&1 ; then
        INSTALLER="sudo yum"
      fi
    fi

    if [ ! "$INSTALLER" ] ; then
      infomsg "ERROR: Cannot determine your machine's installer (cannot find dnf or yum)."
      exit 1
    fi
  fi
  debug "Installer command to be used: ${INSTALLER}"
}

check_crc_running() {
  if [ -z ${_CRC_RUNNING} ]; then
    if crc status | grep "CRC VM:.*Running" > /dev/null 2>&1; then
      _CRC_RUNNING="true"
    else
      _CRC_RUNNING="false"
    fi
    debug "CRC running status: ${_CRC_RUNNING}"
  fi
}

get_console_url() {
  CONSOLE_URL="$(${CRC_OC} get console cluster -o jsonpath='{.status.consoleURL}' 2>/dev/null)"
  if [ "$?" != "0" -o "$CONSOLE_URL" == "" ]; then
    CONSOLE_URL="console-not-available"
  fi
}

get_api_server_url() {
  OPENSHIFT_API_SERVER_URL="$(${CRC_OC} whoami --show-server)"
}

get_status() {
  check_crc_running
  echo "====================================================================="
  echo "Status from crc command [${CRC_COMMAND}]"
  ${CRC_COMMAND} status
  echo "====================================================================="
  echo "oc:  ${CRC_OC}"
  echo "crc: ${CRC_COMMAND}"
  echo "====================================================================="

  if [ "${_CRC_RUNNING}" == "true" ]; then
    get_registry_names
    check_insecure_registry
    get_console_url
    get_api_server_url
    echo "Version from oc command [${CRC_OC}]"
    ${CRC_OC} version
    echo "====================================================================="
    echo "Status from oc command [${CRC_OC}]"
    ${CRC_OC} status
    echo "====================================================================="
    echo "Console:    ${CONSOLE_URL}"
    echo "API URL:    ${OPENSHIFT_API_SERVER_URL}"
    echo "IP address: $(${CRC_COMMAND} ip)"
    echo "Image Repo: ${EXTERNAL_IMAGE_REGISTRY} (${INTERNAL_IMAGE_REGISTRY})"
    echo "====================================================================="
    echo "To install 'oc' in your environment:"
    ${CRC_COMMAND} oc-env
    echo "====================================================================="
    echo "kubeadmin password: $(cat ${CRC_KUBEADMIN_PASSWORD_FILE})"
    echo "kiali password:     kiali"
    echo "johndoe password:   johndoe"
    echo "====================================================================="
    echo "To push images to the image repo you need to log in."
    echo "You can use docker or podman, and you can use kubeadmin or kiali user."
    echo "  oc login -u kubeadmin -p $(cat ${CRC_KUBEADMIN_PASSWORD_FILE}) ${OPENSHIFT_API_SERVER_URL}"
    echo '  docker login -u kubeadmin -p $(oc whoami -t)' ${EXTERNAL_IMAGE_REGISTRY}
    echo "or"
    echo "  oc login -u kiali -p kiali ${OPENSHIFT_API_SERVER_URL}"
    echo '  podman login --tls-verify=false -u kiali -p $(oc whoami -t)' ${EXTERNAL_IMAGE_REGISTRY}
    echo "====================================================================="
  fi
}

check_istio_app() {
  local expected="$1"
  apps=$(${CRC_OC} get deployment.apps -n istio-system -o jsonpath='{range .items[*]}{.metadata.name}{" "}{end}' 2> /dev/null)
  for app in ${apps[@]}
  do
    if [[ "$expected" == "$app" ]]; then
      return 0
    fi
  done
  return 1
}

get_registry_names() {
  local ext="not running"
  local int="not running"
  check_crc_running
  if [ "${_CRC_RUNNING}" == "true" ]; then
    ext=$(${CRC_OC} get image.config.openshift.io/cluster -o custom-columns=EXT:.status.externalRegistryHostnames[0] --no-headers 2>/dev/null)
    int=$(${CRC_OC} get image.config.openshift.io/cluster -o custom-columns=INT:.status.internalRegistryHostname --no-headers 2>/dev/null)
  fi
  EXTERNAL_IMAGE_REGISTRY=${ext:-<unknown>}
  INTERNAL_IMAGE_REGISTRY=${int:-<unknown>}
}

check_insecure_registry() {
  # make sure docker insecure registry is defined
  pgrep -a dockerd | grep "[-]-insecure-registry.*${EXTERNAL_IMAGE_REGISTRY}" > /dev/null 2>&1
  if [ "$?" != "0" ]; then
    grep "OPTIONS=.*--insecure-registry.*${EXTERNAL_IMAGE_REGISTRY}" /etc/sysconfig/docker > /dev/null 2>&1
    if [ "$?" != "0" ]; then
      grep "insecure-registries.*${EXTERNAL_IMAGE_REGISTRY}" /etc/docker/daemon.json > /dev/null 2>&1
      if [ "$?" != "0" ]; then
        infomsg "WARNING: You must tell Docker about the CRC insecure registry (e.g. --insecure-registry ${EXTERNAL_IMAGE_REGISTRY})."
      else
        debug "/etc/docker/daemon.json has the insecure-registry setting. This is good."
      fi
    else
      debug "/etc/sysconfig/docker has defined the insecure-registry setting. This is good."
    fi
  else
    debug "Docker daemon is running with --insecure-registry setting. This is good."
  fi
}

get_route_url() {
  # takes as input "routeName:routeNamespace"
  local routename=$(echo ${1} | cut -d: -f1)
  local routenamespace=$(echo ${1} | cut -d: -f2)
  local protocol="https"
  local termination=$(${CRC_OC} get route ${routename} -n ${routenamespace} -o custom-columns=T:spec.tls.termination --no-headers)
  if [ "${termination}" == "<none>" ]; then
    protocol="http"
  fi
  local host=$(${CRC_OC} get route ${routename} -n ${routenamespace} -o custom-columns=H:spec.host --no-headers)

  ROUTE_URL="${protocol}://${host}"
}

print_all_route_urls() {
  allnames_namespaces="$(${CRC_OC} get routes --all-namespaces --no-headers -o custom-columns=NAME:.metadata.name,NS:.metadata.namespace | sed ${SEDOPTIONS} 's/  */:/g')"
  for n in ${allnames_namespaces}
  do
    get_route_url ${n}
    printf '=====\n%s\n  %s\n' "${n}" "${ROUTE_URL}"
  done
}

get_service_endpoint() {
  # takes as input "serviceName:serviceNamespace"
  local servicename=$(echo ${1} | cut -d: -f1)
  local servicenamespace=$(echo ${1} | cut -d: -f2)
  local data="$(${CRC_OC} get service ${servicename} -n ${servicenamespace} -o custom-columns=I:spec.clusterIP,T:spec.type,NP:spec.ports[*].nodePort,P:spec.ports[*].port --no-headers | sed ${SEDOPTIONS} 's/  */:/g')"
  local clusterIP=$(echo ${data} | cut -d: -f1)
  local servicetype=$(echo ${data} | cut -d: -f2)
  local nodeports=$(echo ${data} | cut -d: -f3)
  local ports=$(echo ${data} | cut -d: -f4)
  local host="$(${CRC_COMMAND} ip)"
  # really only NodePort services are exposed outside of the CRC VM, so we just report those
  if [ ${servicetype} == "NodePort" ]; then
    SERVICE_ENDPOINT="${host}:${nodeports}"
  else
    if [ "${nodeports}" == "<none>" ]; then
      SERVICE_ENDPOINT="inaccessible - (${servicetype}) ${clusterIP}, ports=${ports}"
    else
      SERVICE_ENDPOINT="inaccessible - (${servicetype}) ${clusterIP}, ports=${nodeports}"
    fi
  fi
}

print_all_service_endpoints() {
  # we do filter out services from the internal openshift* and default namespaces
  allnames_namespaces="$(${CRC_OC} get services --all-namespaces --no-headers -o custom-columns=NAME:.metadata.name,NS:.metadata.namespace | sed ${SEDOPTIONS} 's/  */:/g' | grep -v ':openshift*' | grep -v ':default')"
  for n in ${allnames_namespaces}
  do
    get_service_endpoint ${n}
    printf '=====\n%s\n  %s\n' "${n}" "${SERVICE_ENDPOINT}"
  done
}

install_service_mesh() {
  local create_smcp="$1"
  infomsg "Installing the operators..."
  cat <<EOM | ${CRC_OC} apply -f -
---
apiVersion: operators.coreos.com/v1
kind: CatalogSourceConfig
metadata:
  name: hack-redhat-openshift-operators
  namespace: openshift-marketplace
spec:
  csDisplayName: Hack Red Hat Operators
  csPublisher: Hack Red Hat
  packages: 'elasticsearch-operator,jaeger-product,kiali-ossm,servicemeshoperator'
  targetNamespace: openshift-operators
---
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: elasticsearch-operator
  namespace: openshift-operators
spec:
  channel: preview
  installPlanApproval: Automatic
  name: elasticsearch-operator
  source: hack-redhat-openshift-operators
  sourceNamespace: openshift-operators
---
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: jaeger-product
  namespace: openshift-operators
spec:
  channel: stable
  installPlanApproval: Automatic
  name: jaeger-product
  source: hack-redhat-openshift-operators
  sourceNamespace: openshift-operators
---
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: kiali-ossm
  namespace: openshift-operators
spec:
  channel: stable
  installPlanApproval: Automatic
  name: kiali-ossm
  source: hack-redhat-openshift-operators
  sourceNamespace: openshift-operators
---
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: servicemeshoperator
  namespace: openshift-operators
spec:
  channel: '1.0'
  installPlanApproval: Automatic
  name: servicemeshoperator
  source: hack-redhat-openshift-operators
  sourceNamespace: openshift-operators
EOM
  if [ "${create_smcp}" == "true" ] ; then

    infomsg "Waiting for the operator CRDs to come online"
    for crd in servicemeshcontrolplanes.maistra.io servicemeshmemberrolls.maistra.io kialis.kiali.io jaegers.jaegertracing.io elasticsearches.logging.openshift.io
    do
      echo -n "Waiting for $crd ..."
      while ! ${CRC_OC} get crd $crd > /dev/null 2>&1
      do
        sleep 2
        echo -n '.'
      done
      echo "done."
    done

    infomsg "Waiting for operator Deployments to start..."
    for op in elasticsearch-operator jaeger-operator kiali-operator istio-operator
    do
      echo -n "Waiting for ${op} to be ready..."
      readyReplicas="0"
      while [ "$?" != "0" -o "$readyReplicas" == "0" ]
      do
        sleep 1
        echo -n '.'
        readyReplicas="$(${CRC_OC} get deployment ${op} -n openshift-operators -o jsonpath='{.status.readyReplicas}' 2> /dev/null)"
      done
      echo "done."
    done

    infomsg "Creating istio-system namespace."
    ${CRC_OC} create namespace istio-system
    infomsg "Installing Maistra via ServiceMeshControlPlane Custom Resource."
    if [ "${MAISTRA_SMCP_YAML}" != "" ]; then
      ${CRC_OC} create -n istio-system -f ${MAISTRA_SMCP_YAML}
    else
      debug "Using example SMCP/SMMR"
      rm -f /tmp/maistra-smcp.yaml
      get_downloader
      eval ${DOWNLOADER} /tmp/maistra-smcp.yaml "https://raw.githubusercontent.com/Maistra/istio-operator/maistra-1.0/deploy/examples/maistra_v1_servicemeshcontrolplane_cr_minimal.yaml"
      ${CRC_OC} create -n istio-system -f /tmp/maistra-smcp.yaml
    fi
  else
    infomsg "The operators should be available but the Maistra SMCP CR will not be created."
  fi
}

# Change to the directory where this script is and set our environment
SCRIPT_ROOT="$( cd "$(dirname "$0")" ; pwd -P )"
cd ${SCRIPT_ROOT}

# The default version of the crc tool to be downloaded
DEFAULT_CRC_DOWNLOAD_VERSION="1.0.0-beta.3"

# The default version of the crc bundle - this is typically the version included with the CRC download
DEFAULT_CRC_LIBVIRT_DOWNLOAD_VERSION="4.1.11"

# The default virtual CPUs assigned to the CRC VM
DEFAULT_CRC_CPUS="5"

# The default memory (in GB) assigned to the CRC VM
DEFAULT_CRC_MEMORY="20"

# The default virtual disk size (in GB) assigned to the CRC VM
DEFAULT_CRC_VIRTUAL_DISK_SIZE="30"

# set the default openshift address here so that it can be used for the usage text
# This is the IP address where OpenShift will bind its master.
# This should be a valid IP address for the machine where OpenShift is installed.
# TODO: Today, this is not used with CRC. Leaving this here in case a future version of CRC lets you configure this
if which ip > /dev/null 2>&1 ; then
  DEFAULT_OPENSHIFT_IP_ADDRESS=`echo $(ip -f inet addr | grep 'state UP' -A1 | tail -n1 | awk '{print $2}' | cut -f1 -d'/')`
else
  DEFAULT_OPENSHIFT_IP_ADDRESS="127.0.0.1"
fi

# process command line args to override environment
_CMD=""
while [[ $# -gt 0 ]]; do
  key="$1"
  case $key in
    start|up)
      _CMD="start"
      shift
      ;;
    stop|down)
      _CMD="stop"
      shift
      ;;
    delete)
      _CMD="delete"
      shift
      ;;
    status)
      _CMD="status"
      shift
      ;;
    ssh)
      _CMD="ssh"
      shift
      ;;
    routes)
      _CMD="routes"
      shift
      ;;
    services)
      _CMD="services"
      shift
      ;;
    sm-install)
      _CMD="sm-install"
      shift
      ;;
    sm-uninstall)
      _CMD="sm-uninstall"
      shift
      ;;
    bi-install)
      _CMD="bi-install"
      shift
      ;;
    -a|--address)
      OPENSHIFT_IP_ADDRESS="$2"
      shift;shift
      ;;
    -b|--bin-dir)
      OPENSHIFT_BIN_PATH="$2"
      shift;shift
      ;;
    -ca|--crc-arch)
      CRC_DOWNLOAD_ARCH="$2"
      shift;shift
      ;;
    -clv|--crc-libvirt-version)
      CRC_LIBVIRT_DOWNLOAD_VERSION="$2"
      shift;shift
      ;;
    -ccpus|--crc-cpus)
      CRC_CPUS="$2"
      shift;shift
      ;;
    -cmem|--crc-memory)
      CRC_MEMORY="$2"
      shift;shift
      ;;
    -cp|--crc-platform)
      CRC_DOWNLOAD_PLATFORM="$2"
      shift;shift
      ;;
    -cv|--crc-version)
      CRC_DOWNLOAD_VERSION="$2"
      shift;shift
      ;;
    -cvdisk|--crc-virtual-disk-size)
      CRC_VIRTUAL_DISK_SIZE="$2"
      shift;shift
      ;;
    -ie|--istio-enabled)
      ISTIO_ENABLED="$2"
      shift;shift
      ;;
    -smcp|--maistra-smcp-yaml)
      MAISTRA_SMCP_YAML="$2"
      shift;shift
      ;;
    -kuca|--kiali-user-cluster-admin)
      KIALI_USER_IS_CLUSTER_ADMIN="$2"
      shift;shift
      ;;
    -nw|--no-wait-for-istio)
      WAIT_FOR_ISTIO=false
      shift
      ;;
    -p|--pull-secret-file)
      PULL_SECRET_ARG="-p $2"
      shift;shift
      ;;
    -v|--verbose)
      _VERBOSE=true
      shift
      ;;
    -h|--help)
      cat <<HELPMSG

$0 [option...] command

Valid options:
  -a|--address <address>
      The public IP or named address bound to by the OpenShift cluster.
      THIS IS NOT USED TODAY
      Default: ${DEFAULT_OPENSHIFT_IP_ADDRESS}
      Used only for the 'start' command.
  -b|--bin-dir <dir>
      Directory where the OpenShift binaries are or will be stored when downloaded.
      Default: ${HOME}/bin
  -ca|--crc-arch <architecture>
      The architecture of the crc binary to use.
      Default: amd64
  -clv|--crc-libvirt-version <version>
      The version of the crc libvirt bundle to use.
      If one does not exist in the bin directory, it will be downloaded there.
      Default: ${DEFAULT_CRC_LIBVIRT_DOWNLOAD_VERSION}
  -ccpus|--crc-cpus <num CPUs>
      Number of virtual CPUs to assign to the VM.
      Default: ${DEFAULT_CRC_CPUS}
      Used only for the 'start' command.
  -cmem|--crc-memory <memory size>
      Amount of memory (in GB) to assign to the VM.
      Default: ${DEFAULT_CRC_MEMORY}
      Used only for the 'start' command.
  -cp|--crc-platform <platform>
      The platform indicator to determine what crc binary to download.
      Default: linux (darwin if Mac is detected)
  -cv|--crc-version <version>
      The version of the crc binary to use.
      If one does not exist in the bin directory, it will be downloaded there.
      Default: ${DEFAULT_CRC_DOWNLOAD_VERSION}
  -cvdisk|--crc-virtual-disk-size <disk size>
      The size of the virtual disk (in GB) to assign to the VM.
      Default: ${DEFAULT_CRC_VIRTUAL_DISK_SIZE}
      Used only for the 'start' command.
  -h|--help : this message
  -ie|--istio-enabled (true|false)
      When set to true, Maistra/Istio will be installed in OpenShift.
      Default: true
      Used only for the 'start' command.
  -smcp|--maistra-smcp-yaml <file or url>
      Points to the YAML file that defines the ServiceMeshControlPlane custom resource which declares what to install.
      If not defined, a basic one will be used.
      Default: Not set (a basic SMCP will be generated by this script)
      Used only for the 'start' command.
  -kuca|--kiali-user-cluster-admin (true|false)
      Determines if the "kiali" OpenShift user is to be given cluster admin rights.
      Default: not set - you will be prompted during startup
      Used only for the 'start' command.
  -nj|--no-jaeger
      When specified, this script will remove Jaeger and Elasticsearch installations from the
      cluster after start-up.
      Used only for the 'start' command.
  -nw|--no-wait-for-istio
      When specified, this script will not wait for Maistra/Istio to be up and running before exiting.
      This will also be ignored when --istio-enabled is false.
      Used only for the 'start' command.
  -p|--pull-secret-file <filename>
      Specifies the file containing your Image pull secret.
      You can download it from https://cloud.redhat.com/openshift/install/metal/user-provisioned
      CRC will ignore this if the pull secret was already installed during a previous start.
      Used only for the 'start' command.
      Default: not set (you will be prompted for the pull secret json at startup if it does not exist yet)
  -v|--verbose
      Enable logging of debug messages from this script.

The command must be one of:

  * start: Starts the CRC VM with OpenShift and optionally installs Maistra/Istio.
  * stop: Stops the CRC VM retaining all data. 'start' will then bring up the CRC VM in the same state.
  * delete: Stops the CRC VM and removes all persistent data. 'start' will then bring up a clean CRC VM.
  * status: Information about the CRC VM and the OpenShift cluster running inside it.
  * ssh: Provides a command line prompt with root access inside the CRC VM.
  * routes: Outputs URLs for all known routes.
  * services: Outputs URLs for all known service endpoints (excluding internal openshift services).
  * sm-install: Installs Service Mesh into the cluster.
  * sm-uninstall: Removes Service Mesh from the cluster.
  * bi-install: Installs Bookinfo demo into the cluster.

HELPMSG
      exit 1
      ;;
    *)
      infomsg "Unknown argument [$key]. Aborting."
      exit 1
      ;;
  esac
done

########################################
# Environment setup section starts here.

#--------------------------------------------------------------
# Variables below have values that can be overridden by
# command line options (see above) or by environment variables.
#--------------------------------------------------------------

# if sed is gnu-sed then set option to work in posix mode to be compatible with non-gnu-sed versions
if sed --posix 's/ / /' < /dev/null > /dev/null 2>&1 ; then
  SEDOPTIONS="--posix"
fi

# This is where you want the OpenShift binaries to go
OPENSHIFT_BIN_PATH="${OPENSHIFT_BIN_PATH:=${HOME}/bin}"

# This is the IP address where OpenShift will bind its master.
# This should be a valid IP address for the machine where OpenShift is installed.
# TODO: Today, this is not used with CRC. Leaving this here in case a future version of CRC lets you configure this
if [ ! "$OPENSHIFT_IP_ADDRESS" ] ; then
  OPENSHIFT_IP_ADDRESS=${DEFAULT_OPENSHIFT_IP_ADDRESS}
fi

# The version is the tag from the openshift-istio/origin release builds.
# The platform is either "linux" or "darwin".
DEFAULT_OS_PLATFORM=linux
DETECTED_OS_PLATFORM=`uname | tr '[:upper:]' '[:lower:]'`
if [ "${DETECTED_OS_PLATFORM}" = "linux" -o "${DETECTED_OS_PLATFORM}" = "darwin" ] ; then
  DEFAULT_OS_PLATFORM=${DETECTED_OS_PLATFORM}
  debug "The operating system has been detected as ${DEFAULT_OS_PLATFORM}"
fi
CRC_DOWNLOAD_VERSION="${CRC_DOWNLOAD_VERSION:-${DEFAULT_CRC_DOWNLOAD_VERSION}}"
CRC_DOWNLOAD_PLATFORM="${CRC_DOWNLOAD_PLATFORM:-${DEFAULT_OS_PLATFORM}}"
CRC_DOWNLOAD_ARCH="${CRC_DOWNLOAD_ARCH:-amd64}"
CRC_LIBVIRT_DOWNLOAD_VERSION="${CRC_LIBVIRT_DOWNLOAD_VERSION:-${DEFAULT_CRC_LIBVIRT_DOWNLOAD_VERSION}}"
CRC_ROOT_DIR="${HOME}/.crc"
CRC_KUBEADMIN_PASSWORD_FILE="${CRC_ROOT_DIR}/cache/crc_libvirt_${CRC_LIBVIRT_DOWNLOAD_VERSION}/kubeadmin-password"
CRC_KUBECONFIG="${CRC_ROOT_DIR}/cache/crc_libvirt_${CRC_LIBVIRT_DOWNLOAD_VERSION}/kubeconfig"
CRC_MACHINE_IMAGE="${CRC_ROOT_DIR}/machines/crc/crc"
CRC_OC="${CRC_ROOT_DIR}/bin/oc"

# The version of Maistra/Istio to be installed if enabled
MAISTRA_VERSION="maistra-0.11.0"

# If ISTIO_ENABLED=true, then a version of Maistra/Istio will be installed for you.
ISTIO_ENABLED="${ISTIO_ENABLED:-true}"

# By default, wait for Maistra/Istio to be up and running before the script ends.
WAIT_FOR_ISTIO="${WAIT_FOR_ISTIO:-true}"

# VM configuration
CRC_CPUS=${CRC_CPUS:-${DEFAULT_CRC_CPUS}}
CRC_MEMORY=${CRC_MEMORY:-${DEFAULT_CRC_MEMORY}}
CRC_VIRTUAL_DISK_SIZE=${CRC_VIRTUAL_DISK_SIZE:-${DEFAULT_CRC_VIRTUAL_DISK_SIZE}}

#--------------------------------------------------------------
# Variables below have values derived from the variables above.
# These variables below are not meant for users to change.
#--------------------------------------------------------------

# See if sudo is required. It is required if the user is not in the docker group.
if groups ${USER} | grep >/dev/null 2>&1 '\bdocker\b'; then
  DOCKER_SUDO=
else
  DOCKER_SUDO=sudo
fi

# Determine where to get the binaries and their full paths and how to execute them.
CRC_DOWNLOAD_LOCATION="https://mirror.openshift.com/pub/openshift-v4/clients/crc/${CRC_DOWNLOAD_VERSION}/crc-${CRC_DOWNLOAD_PLATFORM}-${CRC_DOWNLOAD_ARCH}.tar.xz"
CRC_EXE_NAME=crc
CRC_EXE_PATH="${OPENSHIFT_BIN_PATH}/${CRC_EXE_NAME}"
CRC_COMMAND="${CRC_EXE_PATH}"
if [ "${_VERBOSE}" == "true" ]; then
  CRC_COMMAND="${CRC_COMMAND} --log-level debug"
fi

CRC_LIBVIRT_DOWNLOAD_LOCATION="http://cdk-builds.usersys.redhat.com/builds/crc/bundles/${CRC_LIBVIRT_DOWNLOAD_VERSION}/crc_libvirt_${CRC_LIBVIRT_DOWNLOAD_VERSION}.crcbundle"
CRC_LIBVIRT_PATH="${OPENSHIFT_BIN_PATH}/crc_libvirt_${CRC_LIBVIRT_DOWNLOAD_VERSION}.crcbundle"

# Environment setup section stops here.
########################################

debug "ENVIRONMENT:
  command=$_CMD
  CRC_COMMAND=$CRC_COMMAND
  CRC_CPUS=$CRC_CPUS
  CRC_DOWNLOAD_LOCATION=$CRC_DOWNLOAD_LOCATION
  CRC_DOWNLOAD_PLATFORM=$CRC_DOWNLOAD_PLATFORM
  CRC_DOWNLOAD_VERSION=$CRC_DOWNLOAD_VERSION
  CRC_EXE_NAME=$CRC_EXE_NAME
  CRC_EXE_PATH=$CRC_EXE_PATH
  CRC_KUBEADMIN_PASSWORD_FILE=$CRC_KUBEADMIN_PASSWORD_FILE
  CRC_KUBECONFIG=$CRC_KUBECONFIG
  CRC_LIBVIRT_DOWNLOAD_LOCATION=$CRC_LIBVIRT_DOWNLOAD_LOCATION
  CRC_LIBVIRT_DOWNLOAD_VERSION=$CRC_LIBVIRT_DOWNLOAD_VERSION
  CRC_LIBVIRT_PATH=$CRC_LIBVIRT_PATH
  CRC_MACHINE_IMAGE=$CRC_MACHINE_IMAGE
  CRC_MEMORY=$CRC_MEMORY
  CRC_OC=$CRC_OC
  CRC_ROOT_DIR=$CRC_ROOT_DIR
  CRC_VIRTUAL_DISK_SIZE=$CRC_VIRTUAL_DISK_SIZE
  DOCKER_SUDO=$DOCKER_SUDO
  ISTIO_ENABLED=$ISTIO_ENABLED
  MAISTRA_SMCP_YAML=$MAISTRA_SMCP_YAML
  MAISTRA_VERSION=$MAISTRA_VERSION
  OPENSHIFT_BIN_PATH=$OPENSHIFT_BIN_PATH
  OPENSHIFT_IP_ADDRESS=$OPENSHIFT_IP_ADDRESS
  OPENSHIFT_ISTIO_MASTER_PUBLIC_URL=$OPENSHIFT_ISTIO_MASTER_PUBLIC_URL
  "

# Fail fast if we don't even have the correct location where the oc client should be
if [ ! -d "${OPENSHIFT_BIN_PATH}" ]; then
  infomsg "ERROR: You must define OPENSHIFT_BIN_PATH to an existing location where you want the downloaded tools to be. It is currently set to: ${OPENSHIFT_BIN_PATH}"
  exit 1
fi

# Download the crc tool if we do not have it yet
if [[ -f "${CRC_EXE_PATH}" ]]; then
  _existingVersion=$(${CRC_EXE_PATH} version | tail -n 1 | sed ${SEDOPTIONS} "s/^version: \([A-Za-z0-9.]*\)-[A-Za-z0-9.-]*+[a-z0-9]*$/\1/")
  _crc_major_minor_patch_version="$(echo -n ${CRC_DOWNLOAD_VERSION} | sed -E 's/([0-9]+.[0-9]+.[0-9]+).*/\1/')"
  if [ "${_existingVersion}" != "${CRC_DOWNLOAD_VERSION}" -a "${_existingVersion}" != "${_crc_major_minor_patch_version}" ]; then
    infomsg "===== WARNING ====="
    infomsg "You already have the crc tool but it does not match the version you want."
    infomsg "Either delete your existing binary and let this script download another one,"
    infomsg "or change the version passed to this script to match the version of your crc tool."
    infomsg "crc is here: ${CRC_EXE_PATH}"
    infomsg "The version of the crc binary is: ${_existingVersion}"
    infomsg "You asked for version: ${CRC_DOWNLOAD_VERSION}"
    infomsg "===== WARNING ====="
    exit 1
  fi
else
  infomsg "Downloading crc binary to ${CRC_EXE_PATH}"

  get_downloader
  eval ${DOWNLOADER} "${CRC_EXE_PATH}.tar.xz" ${CRC_DOWNLOAD_LOCATION}
  if [ "$?" != "0" ]; then
    infomsg "===== WARNING ====="
    infomsg "Could not download the client binary for the version you want."
    infomsg "Make sure this is valid: ${CRC_DOWNLOAD_LOCATION}"
    infomsg "===== WARNING ====="
    rm "${CRC_EXE_PATH}.tar.xz"
    exit 1
  fi
  tar xvf "${CRC_EXE_PATH}.tar.xz" -C "$(dirname ${CRC_EXE_PATH})" --strip 1 '*/crc'
  chmod +x ${CRC_EXE_PATH}
  rm "${CRC_EXE_PATH}.tar.xz"
fi

debug "crc command that will be used: ${CRC_COMMAND}"
debug "$(${CRC_COMMAND} version)"

# Download the crc libvirt image if we do not have it yet
CRC_BUNDLE_ARG="-b ${CRC_LIBVIRT_PATH}"
if [ -f "${CRC_LIBVIRT_PATH}" ]; then
  debug "crc libvirt bundle that will be used: ${CRC_LIBVIRT_PATH}"
elif [ "$(stat -c '%s' ${CRC_EXE_PATH})" -gt "1000000000" ]; then
  debug "crc appears to have the bundle already included. It will be used: $(stat -c '%n (%s bytes)' ${CRC_EXE_PATH})"
  CRC_BUNDLE_ARG=""
else
  infomsg "Downloading crc libvirt bundle to ${CRC_LIBVIRT_PATH}"

  get_downloader
  eval ${DOWNLOADER} "${CRC_LIBVIRT_PATH}" ${CRC_LIBVIRT_DOWNLOAD_LOCATION}
  if [ "$?" != "0" ]; then
    infomsg "===== WARNING ====="
    infomsg "Could not download the crc libvirt bundle."
    infomsg "Make sure this is valid: ${CRC_LIBVIRT_DOWNLOAD_LOCATION}"
    infomsg "===== WARNING ====="
    rm "${CRC_LIBVIRT_PATH}"
    exit 1
  fi
fi

cd ${OPENSHIFT_BIN_PATH}
export KUBECONFIG="${CRC_KUBECONFIG}"

if [ "$_CMD" = "start" ]; then

  infomsg "Setting up the requirements for the OpenShift cluster..."
  debug "${CRC_COMMAND} setup"
  ${CRC_COMMAND} setup

  if [ "$?" != "0" ]; then
    infomsg "ERROR: failed to setup the requirements for OpenShift."
    exit 1
  fi

  infomsg "Starting the OpenShift cluster..."
  # if you change the command line here, also change it below during the restart
  debug "${CRC_COMMAND} start ${PULL_SECRET_ARG} ${CRC_BUNDLE_ARG} -m $(expr ${CRC_MEMORY} '*' 1024) -c ${CRC_CPUS}"
  ${CRC_COMMAND} start ${PULL_SECRET_ARG} ${CRC_BUNDLE_ARG} -m $(expr ${CRC_MEMORY} '*' 1024) -c ${CRC_CPUS}

  if [ "$?" != "0" ]; then
    infomsg "ERROR: failed to start the VM."
    exit 1
  fi

  debug "Checking the memory of the VM..."
  _CURRENT_CRC_MEMORY="$(virsh -c qemu:///system dommemstat crc | grep actual | sed ${SEDOPTIONS} 's/actual \([0-9]*\)/\1/')"
  if [ "${_CURRENT_CRC_MEMORY}" -lt "${CRC_MEMORY}000000" ]; then
    infomsg "Configuring memory for your VM: memory=${CRC_MEMORY}G."
    virsh -c qemu:///system setmaxmem crc ${CRC_MEMORY}000000 --config
    virsh -c qemu:///system setmem crc ${CRC_MEMORY}000000 --config
    _NEED_VM_STOP="true"
    _NEED_VM_START="true"
  else
    debug "VM already configured with ${CRC_MEMORY}G memory."
  fi

  debug "Checking the CPU count of the VM..."
  if [ "$(virsh -c qemu:///system vcpucount crc --live)" != "${CRC_CPUS}" ]; then
    infomsg "Configuring CPUs for your VM: number of CPUs=${CRC_CPUS}"
    virsh -c qemu:///system setvcpus crc ${CRC_CPUS} --maximum --config
    virsh -c qemu:///system setvcpus crc ${CRC_CPUS} --config
    _NEED_VM_STOP="true"
    _NEED_VM_START="true"
  else
    debug "VM already configured with ${CRC_CPUS} CPUs."
  fi

  # See: https://fatmin.com/2016/12/20/how-to-resize-a-qcow2-image-and-filesystem-with-virt-resize/
  # Do this part as the last configuration change since this will require the VM to be stopped.
  debug "Checking the virtual disk size of the VM image..."
  _QEMU_IMG_STDOUT="$(sudo qemu-img info ${CRC_MACHINE_IMAGE})"
  if [ "$?" != "0" ]; then
    infomsg "Will attempt to get shared write lock to obtain disk size"
    _QEMU_IMG_STDOUT="$(sudo qemu-img info -U ${CRC_MACHINE_IMAGE})"
    if [ "$?" != "0" ]; then
      infomsg "Cannnot determine current disk size of VM - will assume there is enough"
      _QEMU_IMG_STDOUT="virtual size: 9999G (99999999999 bytes)"
    fi
  fi
  _CURRENT_VIRTUAL_DISK_SIZE="$(echo "${_QEMU_IMG_STDOUT}" | grep 'virtual size' | sed ${SEDOPTIONS} 's/virtual size: \([0-9]*\)[G].*$/\1/')"
  if [ "${_CURRENT_VIRTUAL_DISK_SIZE}" -lt "${CRC_VIRTUAL_DISK_SIZE}" ]; then
    _INCREASE_VIRTUAL_DISK_SIZE="+$(expr ${CRC_VIRTUAL_DISK_SIZE} - ${_CURRENT_VIRTUAL_DISK_SIZE})G"
    infomsg "The virtual disk size is currently ${_CURRENT_VIRTUAL_DISK_SIZE}G."
    infomsg "You asked for a virtual disk size of ${CRC_VIRTUAL_DISK_SIZE}G."
    infomsg "The virtual disk size will be increased by ${_INCREASE_VIRTUAL_DISK_SIZE}."
    infomsg "This multi-step process will take a long time. Be patient."

    # cannot resize disk while VM is running, shut it down now
    ${CRC_COMMAND} stop
    _NEED_VM_START="true"

    get_installer
    if ! which virt-resize > /dev/null 2>&1 ; then
      infomsg "To set the virtual disk size, installing 'virt-resize' from the 'libguestfs-tools' package."
      eval ${INSTALLER} install libguestfs-tools
    fi

    if ! ${INSTALLER} list installed libguestfs-xfs > /dev/null 2>&1 ; then
      infomsg "To resize the filesystem properly, installing 'libguestfs-xfs'."
      eval ${INSTALLER} install libguestfs-xfs
    fi

    sudo qemu-img resize ${CRC_MACHINE_IMAGE} ${_INCREASE_VIRTUAL_DISK_SIZE}
    debug "Resizing the underlying file systems."
    sudo cp ${CRC_MACHINE_IMAGE} ${CRC_MACHINE_IMAGE}.ORIGINAL
    sudo virt-resize --expand /dev/sda3 ${CRC_MACHINE_IMAGE}.ORIGINAL ${CRC_MACHINE_IMAGE}
    sudo rm ${CRC_MACHINE_IMAGE}.ORIGINAL
    infomsg "The new disk image details:"
    sudo qemu-img info ${CRC_MACHINE_IMAGE}
    sudo virt-filesystems --long -h --all -a ${CRC_MACHINE_IMAGE}
  else
    debug "VM already configured with ${CRC_VIRTUAL_DISK_SIZE}G of virtual disk space."
  fi

  if [ "${_NEED_VM_STOP}" == "true" ]; then
    infomsg "Stopping the VM..."
    ${CRC_COMMAND} stop
  fi

  if [ "${_NEED_VM_START}" == "true" ]; then
    infomsg "Restarting the VM to pick up the new configuration."
    ${CRC_COMMAND} start ${PULL_SECRET_ARG} ${CRC_BUNDLE_ARG} -m ${CRC_MEMORY}000 -c ${CRC_CPUS}
    if [ "$?" != "0" ]; then
      infomsg "ERROR: failed to restart the VM."
      exit 1
    fi
    get_console_url
    echo -n "Waiting for OpenShift console at ${CONSOLE_URL} ..."
    sleep 5
    while ! curl --head -s -k ${CONSOLE_URL} | head -n 1 | grep -q "200[[:space:]]*OK"
    do
      sleep 5
      get_console_url
      echo -n "."
    done
    echo "Done."
    infomsg "VM has been rebooted with the new configuration and OpenShift is ready."
  fi

  # see https://docs.openshift.com/container-platform/4.1/authentication/identity_providers/configuring-htpasswd-identity-provider.html
  # we need to be admin in order to create the htpasswd oauth and users
  infomsg "Creating users 'kiali' and 'johndoe'"
  ${CRC_OC} login -u system:admin
  cat <<EOM | ${CRC_OC} apply -f -
---
# Secret containing two htpasswd credentials:
#   kiali:kiali
#   johndoe:johndoe
apiVersion: v1
metadata:
  name: htpasswd
  namespace: openshift-config
data:
  htpasswd: a2lhbGk6JDJ5JDA1JHhrV1NNY0ZIUXkwZ2RDMUltLnJDZnVsV2NuYkhDQ2w2bDhEdjFETWEwV1hLRzc4U2tVcHQ2CmpvaG5kb2U6JGFwcjEkRzhhL2x1My4kRnc5RjJUczFKNUFKRUNJc05KN1RWLgo=
kind: Secret
type: Opaque
---
apiVersion: config.openshift.io/v1
kind: OAuth
metadata:
  name: cluster
spec:
  identityProviders:
  - name: htpasswd
    type: HTPasswd
    mappingMethod: claim
    htpasswd:
      fileData:
        name: htpasswd
EOM

  if [ "${KIALI_USER_IS_CLUSTER_ADMIN}" == "" ]; then
    infomsg 'Do you want the kiali user to be assigned the cluster-admin role?'
    infomsg 'Select "1" for Yes and "2" for No:'
    select yn in "Yes" "No"; do
      case $yn in
        Yes )
          KIALI_USER_IS_CLUSTER_ADMIN="true"
          break;;
        No )
          KIALI_USER_IS_CLUSTER_ADMIN="false"
          break;;
      esac
    done
  fi

  if [ "${KIALI_USER_IS_CLUSTER_ADMIN}" == "true" ]; then
    infomsg "Will assign the cluster-admin role to the kiali user."
    ${CRC_OC} adm policy add-cluster-role-to-user cluster-admin kiali
    _CREATE_SMCP_RESOURCE="true"
  else
    infomsg "Kiali user will not be assigned the cluster-admin role."
    _CREATE_SMCP_RESOURCE="true" # still try to install Maistra/Istio, it should work with system:admin logged in
  fi

  # Make sure the image registry is exposed via the default route
  if [ "$(${CRC_OC} get config.imageregistry.operator.openshift.io/cluster -o jsonpath='{.spec.defaultRoute}')" != "true" ]; then
    infomsg "Manually patching image registry operator to expose the internal image registry"
    ${CRC_OC} patch config.imageregistry.operator.openshift.io/cluster --patch '{"spec":{"defaultRoute":true}}' --type=merge
  else
    debug "The image registry operator has exposed the internal image registry"
  fi

  # Install Maistra
  ${CRC_OC} get -n istio-system ServiceMeshControlPlane > /dev/null 2>&1
  if [ "$?" != "0" ]; then
    if [ "${ISTIO_ENABLED}" == "true" ] ; then
      install_service_mesh "${_CREATE_SMCP_RESOURCE}"
    else
      infomsg "You asked that Maistra not be enabled - neither the operators nor a SMCP CR will be created."
    fi
  else
    if [ "${ISTIO_ENABLED}" == "true" ] ; then
      infomsg "It appears Maistra has already been installed - will not attempt to do so again."
    else
      infomsg "You asked that Maistra not be enabled, but it appears to have already been installed. You might want to uninstall it."
    fi
  fi

  # If Maistra/Istio is enabled, it should be installing now - if we need to, wait for it to finish
  if [ "${ISTIO_ENABLED}" == "true" ] ; then
    if [ "${WAIT_FOR_ISTIO}" == "true" ]; then
      infomsg "Wait for Maistra/Istio to fully start this is going to take a while..."

      infomsg "Waiting for Maistra/Istio Deployments to be created."
      _EXPECTED_APPS=(istio-citadel prometheus istio-galley istio-policy istio-telemetry istio-pilot istio-egressgateway istio-ingressgateway istio-sidecar-injector)
      for expected in ${_EXPECTED_APPS[@]}
      do
        echo -n "Waiting for $expected ..."
        while ! check_istio_app $expected
        do
             sleep 5
             echo -n '.'
        done
        echo "done."
      done

      infomsg "Waiting for Maistra/Istio Deployments to start..."
      for app in $(${CRC_OC} get deployment.apps -n istio-system -o jsonpath='{range .items[*]}{.metadata.name}{" "}{end}' 2> /dev/null)
      do
         echo -n "Waiting for ${app} to be ready..."
         readyReplicas="0"
         while [ "$?" != "0" -o "$readyReplicas" == "0" ]
         do
            sleep 1
            echo -n '.'
            readyReplicas="$(${CRC_OC} get deployment.app/${app} -n istio-system -o jsonpath='{.status.readyReplicas}' 2> /dev/null)"
         done
         echo "done."
      done
    fi
  fi

  # show the status message
  get_status

elif [ "$_CMD" = "stop" ]; then

  infomsg "Will shutdown the OpenShift cluster."
  ${CRC_COMMAND} stop

elif [ "$_CMD" = "delete" ]; then

  infomsg "Will delete the OpenShift cluster - this removes all persisted data."
  ${CRC_COMMAND} delete

elif [ "$_CMD" = "status" ]; then

  get_status

elif [ "$_CMD" = "ssh" ]; then

  infomsg "Logging into the CRC VM..."
  ssh -i ${CRC_ROOT_DIR}/cache/crc_libvirt_${CRC_LIBVIRT_DOWNLOAD_VERSION}/id_rsa_crc core@$(${CRC_COMMAND} ip)

elif [ "$_CMD" = "routes" ]; then

  print_all_route_urls

elif [ "$_CMD" = "services" ]; then

  print_all_service_endpoints

elif [ "$_CMD" = "sm-install" ]; then

  install_service_mesh "true"

elif [ "$_CMD" = "sm-uninstall" ]; then

  # remove the SMCP and SMMR CRs which uninstalls all the Service Mesh components
  ${CRC_OC} delete -n istio-system $(${CRC_OC} get smcp -n istio-system -o name)
  ${CRC_OC} delete -n istio-system $(${CRC_OC} get smmr -n istio-system -o name)

  # clean up the control plane namespace
  ${CRC_OC} delete namespace istio-system

  # clean up OLM Subscriptions
  for sub in servicemeshoperator kiali-ossm jaeger-product elasticsearch-operator
  do
    ${CRC_OC} delete subscription -n openshift-operators ${sub}
  done

  # clean up OLM CatalogSourceConfig
  ${CRC_OC} delete catalogsourceconfig -n openshift-marketplace hack-redhat-openshift-operators

  # clean up OLM CSVs for all the different operators
  for csv in $(${CRC_OC} get csv -n openshift-operators -o name | grep servicemesh)
  do
    ${CRC_OC} delete -n openshift-operators ${csv}
  done
  for csv in $(${CRC_OC} get csv -n openshift-operators -o name | grep kiali)
  do
    ${CRC_OC} delete -n openshift-operators ${csv}
  done
  for csv in $(${CRC_OC} get csv -n openshift-operators -o name | grep jaeger)
  do
    ${CRC_OC} delete -n openshift-operators ${csv}
  done
  for csv in $(${CRC_OC} get csv -n openshift-operators -o name | grep elasticsearch)
  do
    ${CRC_OC} delete -n openshift-operators ${csv}
  done

  # These should have been deleted when the CSV was removed but for some reason are not. Purge them now
  for r in $(${CRC_OC} get clusterrolebindings -o name | grep -E 'elastic|jaeger|kiali|servicemesh|istio')
  do
    ${CRC_OC} delete ${r}
  done
  for r in $(${CRC_OC} get clusterroles -o name | grep -E 'elastic|jaeger|kiali|servicemesh|istio')
  do
    ${CRC_OC} delete ${r}
  done
  for r in $(${CRC_OC} get rolebindings -n openshift-operators -o name | grep -E 'elastic|jaeger|kiali|servicemesh|istio')
  do
    ${CRC_OC} delete -n openshift-operators ${r}
  done
  for r in $(${CRC_OC} get roles -n openshift-operators -o name | grep -E 'elastic|jaeger|kiali|servicemesh|istio')
  do
    ${CRC_OC} delete -n openshift-operators ${r}
  done
  for r in $(${CRC_OC} get sa -n openshift-operators -o name | grep -E 'elastic|jaeger|kiali|servicemesh|istio')
  do
    ${CRC_OC} delete -n openshift-operators ${r}
  done
  for r in $(${CRC_OC} get configmaps -n openshift-operators -o name | grep -E 'elastic|jaeger|kiali|servicemesh|istio')
  do
    ${CRC_OC} delete -n openshift-operators ${r}
  done
  for r in $(${CRC_OC} get secrets -n openshift-operators -o name | grep -E 'elastic|jaeger|kiali|servicemesh|istio')
  do
    ${CRC_OC} delete -n openshift-operators ${r}
  done

  # clean up additional leftover items
  # see: https://docs.openshift.com/container-platform/4.1/service_mesh/service_mesh_install/removing-ossm.html#ossm-remove-cleanup_removing-ossm
  ${CRC_OC} delete validatingwebhookconfiguration/openshift-operators.servicemesh-resources.maistra.io
  ${CRC_OC} delete -n openshift-operators daemonset/istio-node
  ${CRC_OC} delete clusterrole/istio-admin
  ${CRC_OC} get crds -o name | grep '.*\.istio\.io' | xargs -r -n 1 ${CRC_OC} delete
  ${CRC_OC} get crds -o name | grep '.*\.maistra\.io' | xargs -r -n 1 ${CRC_OC} delete
  ${CRC_OC} get crds -o name | grep '.*\.kiali\.io' | xargs -r -n 1 ${CRC_OC} delete
  ${CRC_OC} get crds -o name | grep '.*\.jaegertracing\.io' | xargs -r -n 1 ${CRC_OC} delete
  ${CRC_OC} get crds -o name | grep '.*\.logging\.openshift\.io' | xargs -r -n 1 ${CRC_OC} delete

elif [ "$_CMD" = "bi-install" ]; then

  # see: https://maistra.io/docs/examples/bookinfo/
  BOOKINFO_NAMESPACE="bookinfo"
  ${CRC_OC} new-project ${BOOKINFO_NAMESPACE}
  ${CRC_OC} patch -n istio-system --type='json' smmr default -p '[{"op": "add", "path": "/spec/members", "value":["'"${BOOKINFO_NAMESPACE}"'"]}]'
  ${CRC_OC} apply -n ${BOOKINFO_NAMESPACE} -f https://raw.githubusercontent.com/Maistra/bookinfo/maistra-1.0/bookinfo.yaml
  ${CRC_OC} apply -n ${BOOKINFO_NAMESPACE} -f https://raw.githubusercontent.com/Maistra/bookinfo/maistra-1.0/bookinfo-gateway.yaml
  infomsg "Bookinfo URL: http://$(${CRC_OC} get route istio-ingressgateway -n istio-system -o jsonpath='{.spec.host}')/productpage"

else
  infomsg "ERROR: Required command must be either: start, stop, delete, status, ssh, routes, services, sm-install, sm-uninstall, bi-install"
  exit 1
fi
