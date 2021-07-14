package appender

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kiali/kiali/config"
	"github.com/kiali/kiali/graph"
	"github.com/kiali/kiali/models"
)

func TestNonTrafficScenario(t *testing.T) {
	assert := assert.New(t)

	config.Set(config.NewConfig())

	a := IdleNodeAppender{
		graph.GraphTypeVersionedApp,
		true,
		false,
	}

	// Empty trafficMap
	trafficMap := graph.NewTrafficMap()
	services := mockServices(a)
	workloads := mockWorkloads(a)

	a.addIdleNodes(trafficMap, graph.Unknown, "testNamespace", services, workloads)
	assert.Equal(7, len(trafficMap))

	id, _ := graph.Id(graph.Unknown, "testNamespace", "customer", "testNamespace", "customer-v1", "customer", "v1", a.GraphType)
	n, ok := trafficMap[id]
	assert.Equal(true, ok)
	assert.Equal("customer-v1", n.Workload)
	assert.Equal("customer", n.App)
	assert.Equal("v1", n.Version)
	assert.Equal(true, n.Metadata[graph.IsIdle])

	id, _ = graph.Id(graph.Unknown, "testNamespace", "preference", "testNamespace", "preference-v1", "preference", "v1", a.GraphType)
	n, ok = trafficMap[id]
	assert.Equal(true, ok)
	assert.Equal("preference-v1", n.Workload)
	assert.Equal("preference", n.App)
	assert.Equal("v1", n.Version)
	assert.Equal(true, n.Metadata[graph.IsIdle])

	id, _ = graph.Id(graph.Unknown, "testNamespace", "recommendation", "testNamespace", "recommendation-v1", "recommendation", "v1", a.GraphType)
	n, ok = trafficMap[id]
	assert.Equal(true, ok)
	assert.Equal("recommendation-v1", n.Workload)
	assert.Equal("recommendation", n.App)
	assert.Equal("v1", n.Version)
	assert.Equal(true, n.Metadata[graph.IsIdle])

	id, _ = graph.Id(graph.Unknown, "testNamespace", "recommendation", "testNamespace", "recommendation-v2", "recommendation", "v2", a.GraphType)
	n, ok = trafficMap[id]
	assert.Equal(true, ok)
	assert.Equal("recommendation-v2", n.Workload)
	assert.Equal("recommendation", n.App)
	assert.Equal("v2", n.Version)
	assert.Equal(true, n.Metadata[graph.IsIdle])

	id, _ = graph.Id(graph.Unknown, "testNamespace", "customer", "", "", "", "", a.GraphType)
	n, ok = trafficMap[id]
	assert.Equal(true, ok)
	assert.Equal(graph.NodeTypeService, n.NodeType)
	assert.Equal("customer", n.Service)
	assert.Equal(true, n.Metadata[graph.IsIdle])

	id, _ = graph.Id(graph.Unknown, "testNamespace", "preference", "", "", "", "", a.GraphType)
	n, ok = trafficMap[id]
	assert.Equal(true, ok)
	assert.Equal(graph.NodeTypeService, n.NodeType)
	assert.Equal("preference", n.Service)
	assert.Equal(true, n.Metadata[graph.IsIdle])

	id, _ = graph.Id(graph.Unknown, "testNamespace", "recommendation", "", "", "", "", a.GraphType)
	n, ok = trafficMap[id]
	assert.Equal(true, ok)
	assert.Equal(graph.NodeTypeService, n.NodeType)
	assert.Equal("recommendation", n.Service)
	assert.Equal(true, n.Metadata[graph.IsIdle])
}

func TestOneNodeTrafficScenario(t *testing.T) {
	assert := assert.New(t)

	config.Set(config.NewConfig())

	a := IdleNodeAppender{
		graph.GraphTypeVersionedApp,
		false,
		false,
	}

	trafficMap := a.oneNodeTraffic()
	services := mockServices(a)
	workloads := mockWorkloads(a)

	a.addIdleNodes(trafficMap, graph.Unknown, "testNamespace", services, workloads)

	assert.Equal(5, len(trafficMap))
	id, _ := graph.Id(graph.Unknown, graph.Unknown, "", graph.Unknown, graph.Unknown, graph.Unknown, graph.Unknown, a.GraphType)
	unknown, ok := trafficMap[id]
	assert.Equal(true, ok)
	assert.Equal(graph.Unknown, unknown.Workload)
	assert.Equal(1, len(unknown.Edges))

	e := unknown.Edges[0]
	assert.Equal(float64(0.8), e.Metadata["httpIn"])
	n := e.Dest
	assert.Equal("customer-v1", n.Workload)
	assert.Equal("customer", n.App)
	assert.Equal("v1", n.Version)
	assert.Equal(nil, n.Metadata[graph.IsIdle])

	id, _ = graph.Id(graph.Unknown, "testNamespace", "preference", "testNamespace", "preference-v1", "preference", "v1", a.GraphType)
	n, ok = trafficMap[id]
	assert.Equal(true, ok)
	assert.Equal("preference-v1", n.Workload)
	assert.Equal("preference", n.App)
	assert.Equal("v1", n.Version)
	assert.Equal(true, n.Metadata[graph.IsIdle])

	id, _ = graph.Id(graph.Unknown, "testNamespace", "recommendation", "testNamespace", "recommendation-v1", "recommendation", "v1", a.GraphType)
	n, ok = trafficMap[id]
	assert.Equal(true, ok)
	assert.Equal("recommendation-v1", n.Workload)
	assert.Equal("recommendation", n.App)
	assert.Equal("v1", n.Version)
	assert.Equal(true, n.Metadata[graph.IsIdle])

	id, _ = graph.Id(graph.Unknown, "testNamespace", "recommendation", "testNamespace", "recommendation-v2", "recommendation", "v2", a.GraphType)
	n, ok = trafficMap[id]
	assert.Equal(true, ok)
	assert.Equal("recommendation-v2", n.Workload)
	assert.Equal("recommendation", n.App)
	assert.Equal("v2", n.Version)
	assert.Equal(true, n.Metadata[graph.IsIdle])
}

func TestVersionWithNoTrafficScenario(t *testing.T) {
	assert := assert.New(t)

	config.Set(config.NewConfig())

	a := IdleNodeAppender{
		graph.GraphTypeVersionedApp,
		false,
		false,
	}

	const cluster = "cluster-default"

	trafficMap := a.v1Traffic(cluster)
	services := mockServices(a)
	workloads := mockWorkloads(a)

	a.addIdleNodes(trafficMap, cluster, "testNamespace", services, workloads)

	assert.Equal(5, len(trafficMap))
	id, _ := graph.Id(cluster, graph.Unknown, "", graph.Unknown, graph.Unknown, graph.Unknown, graph.Unknown, a.GraphType)
	unknown, ok := trafficMap[id]
	assert.Equal(true, ok)
	assert.Equal(graph.Unknown, unknown.Workload)
	assert.Equal(1, len(unknown.Edges))

	customer := unknown.Edges[0].Dest
	assert.Equal("customer-v1", customer.Workload)
	assert.Equal("customer", customer.App)
	assert.Equal("v1", customer.Version)
	assert.Equal(float64(0.8), unknown.Edges[0].Metadata["http"])
	assert.Equal(nil, customer.Metadata[graph.IsIdle])
	assert.Equal(1, len(customer.Edges))

	e := customer.Edges[0]
	preference := e.Dest
	assert.Equal("preference-v1", preference.Workload)
	assert.Equal("preference", preference.App)
	assert.Equal("v1", preference.Version)
	assert.Equal(float64(0.8), e.Metadata["http"])
	assert.Equal(nil, preference.Metadata[graph.IsIdle])
	assert.Equal(1, len(preference.Edges))

	e = preference.Edges[0]
	recommendationV1 := e.Dest
	assert.Equal("recommendation-v1", recommendationV1.Workload)
	assert.Equal("recommendation", recommendationV1.App)
	assert.Equal("v1", recommendationV1.Version)
	assert.Equal(float64(0.8), e.Metadata["http"])
	assert.Equal(nil, recommendationV1.Metadata[graph.IsIdle])

	var idleV2Node *graph.Node
	for _, node := range trafficMap {
		if node.App == "recommendation" && node.Version == "v2" {
			idleV2Node = node
		}
	}
	assert.NotNil(idleV2Node)

	assert.Equal(true, idleV2Node.Metadata[graph.IsIdle])
	assert.Equal(idleV2Node.Cluster, cluster)
}

func mockServices(a IdleNodeAppender) []models.ServiceDetails {
	if !(a.GraphType == graph.GraphTypeService || a.InjectServiceNodes) {
		return []models.ServiceDetails{}
	}

	return []models.ServiceDetails{
		{
			Service: models.Service{
				Name: "customer",
			},
		},
		{
			Service: models.Service{
				Name: "preference",
			},
		},
		{
			Service: models.Service{
				Name: "recommendation",
			},
		},
		{
			Service: models.Service{
				Name: "recommendation",
			},
		},
	}
}

func mockWorkloads(a IdleNodeAppender) []models.WorkloadListItem {
	if a.GraphType == graph.GraphTypeService {
		return []models.WorkloadListItem{}
	}

	return []models.WorkloadListItem{
		{
			Name:   "customer-v1",
			Labels: map[string]string{"app": "customer", "version": "v1"},
		},
		{
			Name:   "preference-v1",
			Labels: map[string]string{"app": "preference", "version": "v1"},
		},
		{
			Name:   "recommendation-v1",
			Labels: map[string]string{"app": "recommendation", "version": "v1"},
		},
		{
			Name:   "recommendation-v2",
			Labels: map[string]string{"app": "recommendation", "version": "v2"},
		},
	}
}

func (a *IdleNodeAppender) oneNodeTraffic() map[string]*graph.Node {
	trafficMap := make(map[string]*graph.Node)

	unknown := graph.NewNode(graph.Unknown, graph.Unknown, "", graph.Unknown, graph.Unknown, graph.Unknown, graph.Unknown, a.GraphType)
	customer := graph.NewNode(graph.Unknown, "testNamespace", "customer", "testNamespace", "customer-v1", "customer", "v1", a.GraphType)
	trafficMap[unknown.ID] = &unknown
	trafficMap[customer.ID] = &customer
	edge := unknown.AddEdge(&customer)
	edge.Metadata["httpIn"] = 0.8
	unknown.Metadata["httpOut"] = 0.8
	customer.Metadata["httpIn"] = 0.8

	return trafficMap
}

func (a *IdleNodeAppender) v1Traffic(cluster string) map[string]*graph.Node {
	trafficMap := make(map[string]*graph.Node)

	unknown := graph.NewNode(cluster, graph.Unknown, "", graph.Unknown, graph.Unknown, graph.Unknown, graph.Unknown, a.GraphType)
	customer := graph.NewNode(cluster, "testNamespace", "customer", "testNamespace", "customer-v1", "customer", "v1", a.GraphType)
	preference := graph.NewNode(cluster, "testNamespace", "preference", "testNamespace", "preference-v1", "preference", "v1", a.GraphType)
	recommendation := graph.NewNode(cluster, "testNamespace", "recommendation", "testNamespace", "recommendation-v1", "recommendation", "v1", a.GraphType)
	trafficMap[unknown.ID] = &unknown
	trafficMap[customer.ID] = &customer
	trafficMap[preference.ID] = &preference
	trafficMap[recommendation.ID] = &recommendation

	// unknown --> customer --> preference --> recommendation

	edge := unknown.AddEdge(&customer)
	edge.Metadata["http"] = 0.8
	unknown.Metadata["httpOut"] = 0.8
	customer.Metadata["httpIn"] = 0.8

	edge = customer.AddEdge(&preference)
	edge.Metadata["http"] = 0.8
	customer.Metadata["httpOut"] = 0.8
	preference.Metadata["httpIn"] = 0.8

	edge = preference.AddEdge(&recommendation)
	edge.Metadata["http"] = 0.8
	preference.Metadata["httpOut"] = 0.8
	recommendation.Metadata["httpIn"] = 0.8

	return trafficMap
}
