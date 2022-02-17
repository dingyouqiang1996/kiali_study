package references

import (
	networking_v1alpha3 "istio.io/client-go/pkg/apis/networking/v1alpha3"

	"github.com/kiali/kiali/kubernetes"
	"github.com/kiali/kiali/models"
)

type VirtualServiceReferences struct {
	Namespace        string
	Namespaces       models.Namespaces
	VirtualServices  []networking_v1alpha3.VirtualService
	DestinationRules []networking_v1alpha3.DestinationRule
}

func (n VirtualServiceReferences) References() models.IstioReferencesMap {
	result := models.IstioReferencesMap{}

	for _, vs := range n.VirtualServices {
		key := models.IstioReferenceKey{Namespace: vs.Namespace, Name: vs.Name, ObjectType: models.ObjectTypeSingular[kubernetes.VirtualServices]}
		references := &models.IstioReferences{}
		references.ServiceReferences = n.getServiceReferences(vs)
		references.ObjectReferences = n.getConfigReferences(vs)
		result.MergeReferencesMap(models.IstioReferencesMap{key: references})
	}

	return result
}

func (n VirtualServiceReferences) getServiceReferences(vs networking_v1alpha3.VirtualService) []models.ServiceReference {
	keys := make(map[string]bool)
	allServices := make([]models.ServiceReference, 0)
	result := make([]models.ServiceReference, 0)
	namespace, clusterName := vs.Namespace, vs.ClusterName

	for _, httpRoute := range vs.Spec.Http {
		if httpRoute != nil {
			for _, dest := range httpRoute.Route {
				if dest != nil {
					host := dest.Destination.Host
					if host == "" {
						continue
					}
					fqdn := kubernetes.GetHost(host, namespace, clusterName, n.Namespaces.GetNames())
					if !fqdn.IsWildcard() {
						allServices = append(allServices, models.ServiceReference{Name: fqdn.Service, Namespace: fqdn.Namespace})
					}
				}
			}
		}
	}

	for _, tcpRoute := range vs.Spec.Tcp {
		if tcpRoute != nil {
			for _, dest := range tcpRoute.Route {
				if dest != nil {
					host := dest.Destination.Host
					if host == "" {
						continue
					}
					fqdn := kubernetes.GetHost(host, namespace, clusterName, n.Namespaces.GetNames())
					if !fqdn.IsWildcard() {
						allServices = append(allServices, models.ServiceReference{Name: fqdn.Service, Namespace: fqdn.Namespace})
					}
				}
			}
		}
	}

	for _, tlsRoute := range vs.Spec.Tls {
		if tlsRoute != nil {
			for _, dest := range tlsRoute.Route {
				if dest != nil {
					host := dest.Destination.Host
					if host == "" {
						continue
					}
					fqdn := kubernetes.GetHost(host, namespace, clusterName, n.Namespaces.GetNames())
					if !fqdn.IsWildcard() {
						allServices = append(allServices, models.ServiceReference{Name: fqdn.Service, Namespace: fqdn.Namespace})
					}
				}
			}
		}
	}
	// filter unique references
	for _, sv := range allServices {
		if !keys[sv.Name+"."+sv.Namespace] {
			result = append(result, sv)
			keys[sv.Name+"."+sv.Namespace] = true
		}
	}
	return result
}

func (n VirtualServiceReferences) getConfigReferences(vs networking_v1alpha3.VirtualService) []models.IstioReference {
	keys := make(map[string]bool)
	result := make([]models.IstioReference, 0)
	allGateways := getAllGateways(vs)
	// filter unique references
	for _, gw := range allGateways {
		if !keys[gw.Name+"."+gw.Namespace+"/"+gw.ObjectType] {
			result = append(result, gw)
			keys[gw.Name+"."+gw.Namespace+"/"+gw.ObjectType] = true
		}
	}
	allDRs := n.getAllDestinationRules(vs)
	// filter unique references
	for _, dr := range allDRs {
		if !keys[dr.Name+"."+dr.Namespace+"/"+dr.ObjectType] {
			result = append(result, dr)
			keys[dr.Name+"."+dr.Namespace+"/"+dr.ObjectType] = true
		}
	}
	return result
}

func (n VirtualServiceReferences) getAllDestinationRules(virtualService networking_v1alpha3.VirtualService) []models.IstioReference {
	allDRs := make([]models.IstioReference, 0)
	for _, dr := range n.DestinationRules {
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
						drHost := kubernetes.GetHost(host, dr.Namespace, dr.ClusterName, n.Namespaces.GetNames())
						vsHost := kubernetes.GetHost(dr.Spec.Host, virtualService.Namespace, virtualService.ClusterName, n.Namespaces.GetNames())
						if kubernetes.FilterByHost(vsHost.String(), vsHost.Namespace, drHost.Service, drHost.Namespace) {
							allDRs = append(allDRs, models.IstioReference{Name: dr.Name, Namespace: dr.Namespace, ObjectType: models.ObjectTypeSingular[kubernetes.DestinationRules]})
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
						drHost := kubernetes.GetHost(host, dr.Namespace, dr.ClusterName, n.Namespaces.GetNames())
						vsHost := kubernetes.GetHost(dr.Spec.Host, virtualService.Namespace, virtualService.ClusterName, n.Namespaces.GetNames())
						if kubernetes.FilterByHost(vsHost.String(), vsHost.Namespace, drHost.Service, drHost.Namespace) {
							allDRs = append(allDRs, models.IstioReference{Name: dr.Name, Namespace: dr.Namespace, ObjectType: models.ObjectTypeSingular[kubernetes.DestinationRules]})
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
						drHost := kubernetes.GetHost(host, dr.Namespace, dr.ClusterName, n.Namespaces.GetNames())
						vsHost := kubernetes.GetHost(dr.Spec.Host, virtualService.Namespace, virtualService.ClusterName, n.Namespaces.GetNames())
						if kubernetes.FilterByHost(vsHost.String(), vsHost.Namespace, drHost.Service, drHost.Namespace) {
							allDRs = append(allDRs, models.IstioReference{Name: dr.Name, Namespace: dr.Namespace, ObjectType: models.ObjectTypeSingular[kubernetes.DestinationRules]})
						}
					}
				}
			}
		}
	}
	return allDRs
}

func getAllGateways(vs networking_v1alpha3.VirtualService) []models.IstioReference {
	allGateways := make([]models.IstioReference, 0)
	namespace, clusterName := vs.Namespace, vs.ClusterName
	if len(vs.Spec.Gateways) > 0 {
		allGateways = append(allGateways, getGatewayReferences(vs.Spec.Gateways, namespace, clusterName)...)
	}
	if len(vs.Spec.Http) > 0 {
		for _, httpRoute := range vs.Spec.Http {
			if httpRoute != nil {
				for _, match := range httpRoute.Match {
					if match != nil {
						allGateways = append(allGateways, getGatewayReferences(match.Gateways, namespace, clusterName)...)
					}
				}
			}
		}
	}
	// TODO TCPMatch is not completely supported in Istio yet
	if len(vs.Spec.Tls) > 0 {
		for _, tlsRoute := range vs.Spec.Tls {
			if tlsRoute != nil {
				for _, match := range tlsRoute.Match {
					if match != nil {
						allGateways = append(allGateways, getGatewayReferences(match.Gateways, namespace, clusterName)...)
					}
				}
			}
		}
	}
	return allGateways
}

func getGatewayReferences(gateways []string, namespace string, clusterName string) []models.IstioReference {
	result := make([]models.IstioReference, 0)
	for _, gate := range gateways {
		gw := kubernetes.ParseGatewayAsHost(gate, namespace, clusterName)
		if !gw.IsWildcard() {
			if gate == "mesh" {
				result = append(result, models.IstioReference{Name: gw.Service, ObjectType: models.ObjectTypeSingular[kubernetes.Gateways]})
			} else {
				result = append(result, models.IstioReference{Name: gw.Service, Namespace: gw.Namespace, ObjectType: models.ObjectTypeSingular[kubernetes.Gateways]})
			}
		}
	}
	return result
}
