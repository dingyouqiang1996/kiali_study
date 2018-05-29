package appender

import (
	"strings"

	"github.com/kiali/kiali/graph/tree"
	"github.com/kiali/kiali/kubernetes"
)

// DeadServiceAppender is responsible for removing from the graph any service nodes for which
// the service is undefined (presumably removed from K8S) and for which there is
// no traffic reported. (kiali-621)
type DeadServiceAppender struct{}

// AppendGraph implements Appender
func (a DeadServiceAppender) AppendGraph(trees *[]tree.ServiceNode, _ string) {
	if len(*trees) == 0 {
		return
	}

	istioClient, err := kubernetes.NewClient()
	checkError(err)

	for _, tree := range *trees {
		applyDeadServices(&tree, istioClient)
	}
}

func applyDeadServices(n *tree.ServiceNode, istioClient kubernetes.IstioClientInterface) {
	// set children to list filtered of dead services
	filteredChildren := make([]*tree.ServiceNode, 0)
	for _, child := range n.Children {
		isDead := false
		rate, hasRate := child.Metadata["rate"]
		split := strings.Split(child.Name, ".")
		serviceName := split[0]
		namespace := split[1]
		if hasRate && rate.(float64) == 0 {
			// filter the child if it has no backing service
			service, err := istioClient.GetService(namespace, serviceName)
			if err != nil || service == nil {
				isDead = true
			} else {
				// flag the service if it has a defined service but no pods running for the service version
				servicePods, err := istioClient.GetServicePods(namespace, serviceName, child.Version)
				if err != nil || servicePods == nil || len(servicePods.Items) == 0 {
					child.Metadata["isDead"] = "true"
				}
			}
		}

		if !isDead {
			filteredChildren = append(filteredChildren, child)
		}
	}

	n.Children = filteredChildren

	// recurse on the remaining children
	for _, child := range n.Children {
		applyDeadServices(child, istioClient)
	}
}
