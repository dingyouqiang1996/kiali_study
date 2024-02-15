package generator

import (
	"context"
	"crypto/md5"
	"fmt"

	"github.com/kiali/kiali/config"
	"github.com/kiali/kiali/graph"
	"github.com/kiali/kiali/log"
	"github.com/kiali/kiali/mesh"
	"github.com/kiali/kiali/mesh/appender"
	"github.com/kiali/kiali/observability"
)

// BuildMeshMap is required by the graph/TelemetryVendor interface
func BuildMeshMap(ctx context.Context, o mesh.Options, gi *mesh.AppenderGlobalInfo) mesh.MeshMap {
	var end observability.EndFunc
	ctx, end = observability.StartSpan(ctx, "BuildMeshMap",
		observability.Attribute("package", "generator"),
	)
	defer end()

	_, finalizers := appender.ParseAppenders(o)
	meshMap := mesh.NewMeshMap()

	// start by creating infra nodes for each accessible namespace
	// note - namespace infra nodes will be converted, as needed, by namespace boxes at the config-gen stage (e.g. in Cytoscape.go)
	clusterMap := make(map[string]bool)
	for _, ns := range o.AccessibleNamespaces {
		clusterMap[ns.Cluster] = true

		var err error
		_, _, err = addInfra(meshMap, mesh.InfraTypeNamespace, ns.Cluster, ns.Name, ns.Name, nil)
		mesh.CheckError(err)
	}

	meshDef, err := gi.Business.Mesh.GetMesh(ctx)
	graph.CheckError(err)

	for _, cp := range meshDef.ControlPlanes {
		// add any cluster that is configured but somehow has no accessible namespace
		if _, ok := clusterMap[cp.Cluster.Name]; !ok {
			_, _, err := addInfra(meshMap, mesh.InfraTypeCluster, cp.Cluster.Name, mesh.Unknown, cp.Cluster.Name, cp.Cluster)
			mesh.CheckError(err)
		}
		for _, mc := range cp.ManagedClusters {
			if _, ok := clusterMap[mc.Name]; ok {
				_, _, err := addInfra(meshMap, mesh.InfraTypeCluster, mc.Name, mesh.Unknown, mc.Name, mc)
				mesh.CheckError(err)

				continue
			}
		}

		// add the control plane istiod
		_, _, err := addInfra(meshMap, mesh.InfraTypeIstiod, cp.Cluster.Name, cp.IstiodNamespace, cp.IstiodName, nil)
		mesh.CheckError(err)

		// add any Kiali instances (note, this will not include the home Kiali)
		for _, kiali := range cp.Cluster.KialiInstances {
			_, _, err = addInfra(meshMap, mesh.InfraTypeKiali, cp.Cluster.Name, kiali.Namespace, kiali.ServiceName, nil)
			mesh.CheckError(err)
		}

		// add the home Kiali
		var kiali *mesh.Node
		conf := config.Get()
		kiali, _, err = addInfra(meshMap, mesh.InfraTypeKiali, conf.KubernetesConfig.ClusterName, conf.Deployment.Namespace, conf.Deployment.InstanceName, nil)
		mesh.CheckError(err)

		// add the Kiali external services...  How to do this?
		var node *mesh.Node
		node, _, err = addInfra(meshMap, mesh.InfraTypeMetricStore, conf.KubernetesConfig.ClusterName, conf.Deployment.Namespace, "Prometheus", nil)
		mesh.CheckError(err)

		kiali.AddEdge(node)

		if conf.ExternalServices.Tracing.Enabled {
			node, _, err = addInfra(meshMap, mesh.InfraTypeTraceStore, conf.KubernetesConfig.ClusterName, conf.Deployment.Namespace, string(conf.ExternalServices.Tracing.Provider), nil)
			mesh.CheckError(err)

			kiali.AddEdge(node)
		}

		if conf.ExternalServices.Grafana.Enabled {
			node, _, err = addInfra(meshMap, mesh.InfraTypeGrafana, conf.KubernetesConfig.ClusterName, conf.Deployment.Namespace, "Grafana", nil)
			mesh.CheckError(err)

			kiali.AddEdge(node)
		}
	}

	// The finalizers can perform final manipulations on the complete graph
	for _, f := range finalizers {
		f.AppendGraph(meshMap, gi, nil)
	}

	return meshMap
}

func addInfra(meshMap mesh.MeshMap, infraType, cluster, namespace, name string, infraData interface{}) (*mesh.Node, bool, error) {
	log.Infof("Adding Infra [%s][%s]", infraType, name)

	id, err := mesh.Id(cluster, namespace, name)
	if err != nil {
		return nil, false, err
	}
	node, found := meshMap[id]
	if !found {
		newNode := mesh.NewNodeExplicit(id, mesh.NodeTypeInfra, infraType, cluster, namespace, name)
		node = newNode
		meshMap[id] = node
	}
	node.Metadata["tsHash"] = timeSeriesHash(cluster, namespace, name)
	if infraData != nil {
		node.Metadata[mesh.InfraData] = infraData
	}
	return node, found, nil
}

func timeSeriesHash(cluster, namespace, name string) string {
	return fmt.Sprintf("%x", md5.Sum([]byte(fmt.Sprintf("%s:%s:%s", cluster, namespace, name))))
}
