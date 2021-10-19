package destinationrules

import (
	"strconv"
	"strings"

	networking_v1alpha3 "istio.io/client-go/pkg/apis/networking/v1alpha3"

	core_v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/kiali/kiali/kubernetes"
	"github.com/kiali/kiali/models"
)

type NoDestinationChecker struct {
	Namespace       string
	Namespaces      models.Namespaces
	WorkloadList    models.WorkloadList
	DestinationRule networking_v1alpha3.DestinationRule
	VirtualServices []networking_v1alpha3.VirtualService
	ServiceEntries  map[string][]string
	Services        []core_v1.Service
	RegistryStatus  []*kubernetes.RegistryStatus
}

// Check parses the DestinationRule definitions and verifies that they point to an existing service, including any subset definitions
func (n NoDestinationChecker) Check() ([]*models.IstioCheck, bool) {
	valid := true
	validations := make([]*models.IstioCheck, 0)

	fqdn := kubernetes.GetHost(n.DestinationRule.Spec.Host, n.DestinationRule.Namespace, n.DestinationRule.ClusterName, n.Namespaces.GetNames())
	// Testing Kubernetes Services + Istio ServiceEntries + Istio Runtime Registry (cross namespace)
	if !n.hasMatchingService(fqdn, n.DestinationRule.Namespace) {
		validation := models.Build("destinationrules.nodest.matchingregistry", "spec/host")
		valid = false
		validations = append(validations, &validation)
	} else if len(n.DestinationRule.Spec.Subsets) > 0 {
		// Check that each subset has a matching workload somewhere..
		for i, subset := range n.DestinationRule.Spec.Subsets {
			if len(subset.Labels) > 0 {
				if !n.hasMatchingWorkload(fqdn.Service, subset.Labels) {
					validation := models.Build("destinationrules.nodest.subsetlabels",
						"spec/subsets["+strconv.Itoa(i)+"]")
					if n.isSubsetReferenced(n.DestinationRule.Spec.Host, subset.Name) {
						valid = false
					} else {
						validation.Severity = models.Unknown
					}
					validations = append(validations, &validation)
				}
			} else {
				validation := models.Build("destinationrules.nodest.subsetnolabels",
					"spec/subsets["+strconv.Itoa(i)+"]")
				validations = append(validations, &validation)
				// Not changing valid value, if other subset is on error, a valid = false has priority
			}
		}
	}
	return validations, valid
}

func (n NoDestinationChecker) hasMatchingWorkload(service string, subsetLabels map[string]string) bool {
	// Check wildcard hosts - needs to match "*" and "*.suffix" also..
	if strings.HasPrefix(service, "*") {
		return true
	}

	// Covering 'servicename.namespace' host format scenario
	svc := service
	svcParts := strings.Split(service, ".")
	if len(svcParts) > 1 {
		svc = svcParts[0]
	}

	var selectors map[string]string

	// Find the correct service
	for _, s := range n.Services {
		if s.Name == svc {
			selectors = s.Spec.Selector
		}
	}

	// Check workloads
	if len(selectors) == 0 {
		return false
	}

	selector := labels.SelectorFromSet(labels.Set(selectors))

	subsetLabelSet := labels.Set(subsetLabels)
	subsetSelector := labels.SelectorFromSet(subsetLabelSet)

	for _, wl := range n.WorkloadList.Workloads {
		wlLabelSet := labels.Set(wl.Labels)
		if selector.Matches(wlLabelSet) {
			if subsetSelector.Matches(wlLabelSet) {
				return true
			}
		}
	}
	return false
}

func (n NoDestinationChecker) hasMatchingService(host kubernetes.Host, itemNamespace string) bool {
	// Check wildcard hosts - needs to match "*" and "*.suffix" also..
	if host.IsWildcard() {
		return true
	}

	// Covering 'servicename.namespace' host format scenario
	localSvc, localNs := kubernetes.ParseTwoPartHost(host)

	if localNs == itemNamespace {
		// Check Workloads
		if matches := kubernetes.HasMatchingWorkloads(localSvc, n.WorkloadList.GetLabels()); matches {
			return matches
		}

		// Check ServiceNames
		if matches := kubernetes.HasMatchingServices(localSvc, n.Services); matches {
			return matches
		}
	}

	// Check ServiceEntries
	if kubernetes.HasMatchingServiceEntries(host.String(), n.ServiceEntries) {
		return true
	}

	// Use RegistryStatus to check destinations that may not be covered with previous check
	// i.e. Multi-cluster or Federation validations
	if kubernetes.HasMatchingRegistryStatus(host.String(), n.RegistryStatus) {
		return true
	}
	return false
}

func (n NoDestinationChecker) isSubsetReferenced(host string, subset string) bool {
	virtualServices, ok := n.getVirtualServices(host, subset)
	if ok && len(virtualServices) > 0 {
		return true
	}

	return false
}

func (n NoDestinationChecker) getVirtualServices(virtualServiceHost string, virtualServiceSubset string) ([]networking_v1alpha3.VirtualService, bool) {
	vss := make([]networking_v1alpha3.VirtualService, 0, len(n.VirtualServices))

	for _, virtualService := range n.VirtualServices {

		if len(virtualService.Spec.Http) > 0 {
			for _, httpRoute := range virtualService.Spec.Http {
				if httpRoute == nil {
					continue
				}
				if len(httpRoute.Route) > 0 {
					for _, dest := range httpRoute.Route {
						if dest == nil || dest.Destination == nil {
							continue
						}
						host := dest.Destination.Host
						subset := dest.Destination.Subset
						drHost := kubernetes.GetHost(host, n.DestinationRule.Namespace, n.DestinationRule.ClusterName, n.Namespaces.GetNames())
						vsHost := kubernetes.GetHost(virtualServiceHost, virtualService.Namespace, virtualService.ClusterName, n.Namespaces.GetNames())
						// TODO Host could be in another namespace (FQDN)
						if kubernetes.FilterByHost(vsHost.String(), drHost.Service, drHost.Namespace) && subset == virtualServiceSubset {
							vss = append(vss, virtualService)
						}
					}
				}
			}
		}

		if len(virtualService.Spec.Tcp) > 0 {
			for _, tcpRoute := range virtualService.Spec.Tcp {
				if tcpRoute == nil {
					continue
				}
				if len(tcpRoute.Route) > 0 {
					for _, dest := range tcpRoute.Route {
						if dest == nil || dest.Destination == nil {
							continue
						}
						host := dest.Destination.Host
						subset := dest.Destination.Subset
						drHost := kubernetes.GetHost(host, n.DestinationRule.Namespace, n.DestinationRule.ClusterName, n.Namespaces.GetNames())
						vsHost := kubernetes.GetHost(virtualServiceHost, virtualService.Namespace, virtualService.ClusterName, n.Namespaces.GetNames())
						// TODO Host could be in another namespace (FQDN)
						if kubernetes.FilterByHost(vsHost.String(), drHost.Service, drHost.Namespace) && subset == virtualServiceSubset {
							vss = append(vss, virtualService)
						}
					}
				}
			}
		}

		if len(virtualService.Spec.Tls) > 0 {
			for _, tlsRoute := range virtualService.Spec.Tls {
				if tlsRoute == nil {
					continue
				}
				if len(tlsRoute.Route) > 0 {
					for _, dest := range tlsRoute.Route {
						if dest == nil || dest.Destination == nil {
							continue
						}
						host := dest.Destination.Host
						subset := dest.Destination.Subset
						drHost := kubernetes.GetHost(host, n.DestinationRule.Namespace, n.DestinationRule.ClusterName, n.Namespaces.GetNames())
						vsHost := kubernetes.GetHost(virtualServiceHost, virtualService.Namespace, virtualService.ClusterName, n.Namespaces.GetNames())
						// TODO Host could be in another namespace (FQDN)
						if kubernetes.FilterByHost(vsHost.String(), drHost.Service, drHost.Namespace) && subset == virtualServiceSubset {
							vss = append(vss, virtualService)
						}
					}
				}
			}
		}
	}

	return vss, len(vss) > 0
}
