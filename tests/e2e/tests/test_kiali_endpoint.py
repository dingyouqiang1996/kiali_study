import pytest
import json
import conftest

# Note: Number of services +1 Views Group Node
# Note: Node and Edge counts are based on traffic origainating from the Ingress
BOOKINFO_EXPECTED_NODES=7
BOOKINFO_EXPECTED_EDGES=6

PARAMS = {'graphType': 'versionedApp', 'duration': '60s'}

def test_service_graph_rest_endpoint(kiali_json):

    assert kiali_json != None, "Json: {}".format(kiali_json)

    # Validate that there are Nodes
    assert len(kiali_json.get('elements').get('nodes')) >= 1

    # Validate that there are Edges
    assert len(kiali_json.get('elements').get('edges')) >= 1

def test_service_graph_bookinfo_namespace_(kiali_client):
    environment_configmap = conftest.__get_environment_config__(conftest.ENV_FILE)
    bookinfo_namespace = environment_configmap.get('mesh_bookinfo_namespace')

    # Validate Node count
    nodes = kiali_client.graph_namespace(namespace=bookinfo_namespace, params=PARAMS)["elements"]['nodes']
    #print "Node count: {}".format(len(nodes))
    assert len(nodes) >=  BOOKINFO_EXPECTED_NODES
    
    # validate edge count
    edges = kiali_client.graph_namespace(namespace=bookinfo_namespace, params=PARAMS)["elements"]['edges']
    #print "Edge count: {}".format(len(edges))
    assert len(edges) >= BOOKINFO_EXPECTED_EDGES
