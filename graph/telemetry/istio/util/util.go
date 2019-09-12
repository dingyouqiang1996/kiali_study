package util

import (
	"strings"

	"github.com/kiali/kiali/config"
	"github.com/kiali/kiali/graph"
)

// HandleMultiClusterRequest ensures the proper destination service workload and name
// for requests forwarded from another cluster (via a ServiceEntry).
//
// Given a request from clusterA to clusterB, clusterA will generate source telemetry
// (from the source workload to the service entry) and clusterB will generate destination
// telemetry (from unknown to the destination workload). If this is the destination
// telemetry the destination_service_name label will be set to the service entry host,
// which is required to have the form <name>.<namespace>.global where name and namespace
// correspond to the remote service’s name and namespace respectively. In this situation
// we alter the request in two ways:
//
// First, we reset destSvcName to <name> in order to unify remote and local requests to the
// service. By doing this the graph will show only one <service> node instead of having a
// node for both <service> and <name>.<namespace>.global which in practice, are the same.
//
// Second, we reset destSvcNs to <namespace>. We want destSvcNs to be set to the namespace
// of the remote service's namespace.  But in practice it will be set to the namespace
// (on clusterA) where the servieEntry is defined. This is not useful for the visualization,
// and so we replace it here. Note that <namespace> should be equivalent to the value set for
// destination_workload_namespace, we just use <namespace> for convenience, we have it here.
//
// All of this is only done if source workload is "unknown", which is what indicates that
// this represents the destination telemetry on clusterB, and if the destSvcName is in
// the MC format. When the source workload IS known the traffic it should be representing
// the clusterA traffic to the ServiceEntry and being routed out of the cluster. That use
// case is handled in the service_entry.go file.
func HandleMultiClusterRequest(sourceWlNs, sourceWl, destSvcNs, destSvcName string) (string, string) {
	if sourceWlNs == graph.Unknown && sourceWl == graph.Unknown {
		destSvcNameEntries := strings.Split(destSvcName, ".")

		if len(destSvcNameEntries) == 3 && destSvcNameEntries[2] == config.IstioMultiClusterHostSuffix {
			return destSvcNameEntries[1], destSvcNameEntries[0]
		}
	}

	return destSvcNs, destSvcName
}
