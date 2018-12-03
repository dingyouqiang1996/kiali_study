import os
import pytest
import yaml
import re
import requests
from kiali import KialiClient
from utils.command_exec import command_exec

ENV_FILE = './config/env.yaml'
CIRCUIT_BREAKER_FILE = 'assets/bookinfo-reviews-all-cb.yaml'
VIRTUAL_SERVICE_FILE = 'assets/bookinfo-ratings-delay.yaml'

@pytest.fixture(scope='session')
def kiali_client():
    config = __get_environment_config__(ENV_FILE)
    yield __get_kiali_client__(config)
    __remove_assets()

def get_bookinfo_namespace():
    return __get_environment_config__(ENV_FILE).get('mesh_bookinfo_namespace')

def __get_kiali_client__(config):
    if(config.get('kiali_ssl_enabled') is True):
        return KialiClient(hostname=config.get('kiali_hostname'), username=config.get('kiali_username'), password=config.get('kiali_password'))
    else:
        return KialiClient(host=config.get('kiali_hostname'), username=config.get('kiali_username'), password=config.get('kiali_password'), auth_type='https')

def __get_environment_config__(env_file):
    with open(env_file) as yamlfile:
        config = yaml.load(yamlfile)
    return config

def __remove_assets():
  print('Cleanning up: ')
  namespace = get_bookinfo_namespace()
  file_count = 0
  for root, dirs, files in os.walk('./assets'):
    file_count = len(files)

    for name in files:
      command_exec.oc_delete('./assets/' + name, namespace)

  print('Assets deleted: {}'.format(file_count))

def get_istio_clusterrole_file():
      yaml_content = requests.get( __get_environment_config__(ENV_FILE).get('istio_clusterrole')).content.decode("utf-8")

      yaml_content = re.sub("app: {{.+}}", "app: kiali", yaml_content)
      yaml_content = re.sub("chart: {{.+}}", "version: 0.10 ", yaml_content)
      yaml_content = re.sub("heritage: {{.+}}\n", "", yaml_content)
      yaml_content = re.sub("release: {{.+}}", "", yaml_content)
      return yaml.load(yaml_content)


def get_kiali_clusterrole_file(file_type):

    if(file_type == "Openshift"):
        yaml_content = requests.get(__get_environment_config__(ENV_FILE).get('kiali_openshift_clusterrole')).content.decode("utf-8")
    elif(file_type == "Kubernetes"):
        yaml_content = requests.get(__get_environment_config__(ENV_FILE).get('kiali_kubernetes_clusterrole')).content.decode("utf-8")


    yaml_content = re.sub("\${VERSION_LABEL}", "0.10", yaml_content)
    return yaml.load(yaml_content)

