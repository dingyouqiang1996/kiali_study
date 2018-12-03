import pytest
import tests.conftest as conftest

OBJECT_TYPE = 'virtualservices'
OBJECT = 'bookinfo-vs'

def test_istio_config_list(kiali_client):
    bookinfo_namespace = conftest.get_bookinfo_namespace()
    json = kiali_client.request(method_name='istioConfigList', path={'namespace': bookinfo_namespace}).json()

    assert json != None
    assert "destinationRules" in json
    assert json.get('destinationRules') != None
    assert bookinfo_namespace in json.get('namespace').get('name')

def test_istio_namespace_validations_endpoint(kiali_client):
    bookinfo_namespace = conftest.get_bookinfo_namespace()
    istio_validations = kiali_client.request(method_name='namespaceValidations', path={'namespace': bookinfo_namespace}).json()

    assert istio_validations != None
    assert bookinfo_namespace in istio_validations

def test_istio_object_type(kiali_client):
    bookinfo_namespace = conftest.get_bookinfo_namespace()

    istio_object_type = kiali_client.request(method_name='istioConfigDetails',
                        path={'namespace': bookinfo_namespace, 'object_type': OBJECT_TYPE, 'object': OBJECT}).json()
    assert istio_object_type != None
    assert "destinationRule" in istio_object_type
    assert bookinfo_namespace in istio_object_type.get('namespace').get('name')

def test_istio_object_istio_validations(kiali_client):
    bookinfo_namespace = conftest.get_bookinfo_namespace()

    istio_validations = kiali_client.request(method_name='objectValidations',
                            path={'namespace': bookinfo_namespace, 'object_type': OBJECT_TYPE, 'object': OBJECT}).json()
    assert istio_validations != None
    assert istio_validations.get('virtualservice') != None
    assert OBJECT in istio_validations.get('virtualservice')

