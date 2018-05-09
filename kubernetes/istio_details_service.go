package kubernetes

import (
	"fmt"
	"sync"
	"github.com/kiali/kiali/config"
)

// GetIstioDetails returns Istio details for a given namespace,
// on this version it collects the RouterRules, DestinationPolicies, VirtualService and DestinationRules defined for a namespace.
// If serviceName param is provided, it filters all the Istio objects pointing to a particular service.
// It returns an error on any problem.
func (in *IstioClient) GetIstioDetails(namespace string, serviceName string) (*IstioDetails, error) {
	var routeRules, destinationPolicies, virtualServices, destinationRules []IstioObject
	var routeRulesErr, destinationPoliciesErr, virtualServicesErr, destinationRulesErr error
	var wg sync.WaitGroup
	wg.Add(4)

	go func() {
		defer wg.Done()
		routeRules, routeRulesErr = in.GetRouteRules(namespace, serviceName)
	}()

	go func() {
		defer wg.Done()
		destinationPolicies, destinationPoliciesErr = in.GetDestinationPolicies(namespace, serviceName)
	}()

	go func() {
		defer wg.Done()
		virtualServices, virtualServicesErr = in.GetVirtualServices(namespace, serviceName)
	}()

	go func() {
		defer wg.Done()
		destinationRules, destinationRulesErr = in.GetDestinationRules(namespace, serviceName)
	}()

	wg.Wait()

	var istioDetails = IstioDetails{}

	istioDetails.RouteRules = routeRules
	if routeRulesErr != nil {
		return nil, routeRulesErr
	}

	istioDetails.DestinationPolicies = destinationPolicies
	if destinationPoliciesErr != nil {
		return nil, destinationPoliciesErr
	}

	istioDetails.VirtualServices = virtualServices
	if virtualServicesErr != nil {
		return nil, virtualServicesErr
	}

	istioDetails.DestinationRules = destinationRules
	if destinationRulesErr != nil {
		return nil, destinationRulesErr
	}

	return &istioDetails, nil
}

// GetRouteRules returns all RouteRules for a given namespace.
// If serviceName param is provided it will filter all RouteRules having a destination pointing to particular service.
// It returns an error on any problem.
func (in *IstioClient) GetRouteRules(namespace string, serviceName string) ([]IstioObject, error) {
	result, err := in.istioConfigApi.Get().Namespace(namespace).Resource(routeRules).Do().Get()
	if err != nil {
		return nil, err
	}
	rulesList, ok := result.(*RouteRuleList)
	if !ok {
		return nil, fmt.Errorf("%s/%s doesn't return a RouteRule list", namespace, serviceName)
	}

	routerRules := make([]IstioObject, 0)
	for _, rule := range rulesList.GetItems() {
		appendRule := serviceName == ""
		if !appendRule && filterByDestination(rule.GetSpec(), namespace, serviceName, "") {
			appendRule = true
		}
		if appendRule {
			routerRules = append(routerRules, rule.DeepCopyIstioObject())
		}
	}
	return routerRules, nil
}

// GetDestinationPolicies returns all DestinationPolicies for a given namespace.
// If serviceName param is provided it will filter all DestinationPolicies having a destination pointing to a particular service.
// It returns an error on any problem.
func (in *IstioClient) GetDestinationPolicies(namespace string, serviceName string) ([]IstioObject, error) {
	result, err := in.istioConfigApi.Get().Namespace(namespace).Resource(destinationPolicies).Do().Get()
	if err != nil {
		return nil, err
	}
	destinationPolicyList, ok := result.(*DestinationPolicyList)
	if !ok {
		return nil, fmt.Errorf("%s/%s doesn't return a DestinationPolicy list", namespace, serviceName)
	}

	destinationPolicies := make([]IstioObject, 0)
	for _, policy := range destinationPolicyList.Items {
		appendPolicy := serviceName == ""
		if !appendPolicy && filterByDestination(policy.GetSpec(), namespace, serviceName, "") {
			appendPolicy = true
		}
		if appendPolicy {
			destinationPolicies = append(destinationPolicies, policy.DeepCopyIstioObject())
		}
	}
	return destinationPolicies, nil
}

// GetVirtualServices return all VirtualServices for a given namespace.
// If serviceName param is provided it will filter all VirtualServices having a host defined on a particular service.
// It returns an error on any problem.
func (in *IstioClient) GetVirtualServices(namespace string, serviceName string) ([]IstioObject, error) {
	result, err := in.istioNetworkingApi.Get().Namespace(namespace).Resource(virtualServices).Do().Get()
	if err != nil {
		return nil, err
	}
	virtualServiceList, ok := result.(*VirtualServiceList)
	if !ok {
		return nil, fmt.Errorf("%s/%s doesn't return a VirtualService list", namespace, serviceName)
	}

	virtualServices := make([]IstioObject, 0)
	for _, virtualService := range virtualServiceList.GetItems() {
		appendVirtualService := serviceName == ""
		if !appendVirtualService && filterByHost(virtualService.GetSpec(), serviceName) {
			appendVirtualService = true
		}
		if appendVirtualService {
			virtualServices = append(virtualServices, virtualService.DeepCopyIstioObject())
		}
	}
	return virtualServices, nil
}

// GetDestinationRules returns all DestinationRules for a given namespace.
// If serviceName param is provided it will filter all DestinationRules having a host defined on a particular service.
// It returns an error on any problem.
func (in *IstioClient) GetDestinationRules(namespace string, serviceName string) ([]IstioObject, error) {
	result, err := in.istioNetworkingApi.Get().Namespace(namespace).Resource(destinationRules).Do().Get()
	if err != nil {
		return nil, err
	}
	destinationRuleList, ok := result.(*DestinationRuleList)
	if !ok {
		return nil, fmt.Errorf("%s/%s doesn't return a DestinationRule list", namespace, serviceName)
	}

	destinationRules := make([]IstioObject, 0)
	for _, destinationRule := range destinationRuleList.Items {
		appendDestinationRule := serviceName != ""
		if name, ok := destinationRule.Spec["name"]; ok {
			if name == serviceName {
				appendDestinationRule = true
			}
		}
		if appendDestinationRule {
			destinationRules = append(destinationRules, destinationRule.DeepCopyIstioObject())
		}
	}
	return destinationRules, nil
}

// CheckRouteRule returns true if the routeRule object includes a destination defined by namespace, serviceName and version parameters.
// It returns false otherwise.
func CheckRouteRule(routeRule IstioObject, namespace string, serviceName string, version string) bool {
	if routeRule == nil || routeRule.GetSpec() == nil {
		return false
	}
	return filterByDestination(routeRule.GetSpec(), namespace, serviceName, version)
}

// CheckVirtualService returns true if virtualService object has defined a route on a service for any subset passed as parameter.
// It returns false otherwise.
func CheckVirtualService(virtualService IstioObject, namespace string, serviceName string, subsets []string) bool {
	if virtualService == nil || virtualService.GetSpec() == nil || subsets == nil {
		return false
	}
	if len(subsets) > 0 && filterByHost(virtualService.GetSpec(), serviceName) {
		if http, ok := virtualService.GetSpec()["http"]; ok && checkSubsetRoute(http, serviceName, subsets) {
			return true
		}
		if tcp, ok := virtualService.GetSpec()["tcp"]; ok && checkSubsetRoute(tcp, serviceName, subsets) {
			return true
		}
	}
	return false
}

// CheckDestinationPolicyCircuitBreaker returns true if the destinationPolicy object includes a circuitBreaker defined
// on a destination defined by namespace, serviceName and version parameters.
// It returns false otherwise.
func CheckDestinationPolicyCircuitBreaker(destinationPolicy IstioObject, namespace string, serviceName string, version string) bool {
	if destinationPolicy == nil || destinationPolicy.GetSpec() == nil {
		return false
	}
	_, hasCircuitBreaker := destinationPolicy.GetSpec()["circuitBreaker"]
	if !hasCircuitBreaker {
		return false
	}
	return filterByDestination(destinationPolicy.GetSpec(), namespace, serviceName, version)
}

// GetDestinationRulesSubsets returns an array of subset names where a specific version is defined for a given service
func GetDestinationRulesSubsets(destinationRules []IstioObject, serviceName, version string) []string {
	cfg := config.Get()
	foundSubsets := make([]string, 0)
	for _, destinationRule := range destinationRules {
		if dName, ok := destinationRule.GetSpec()["name"]; ok && dName == serviceName {
			if subsets, ok := destinationRule.GetSpec()["subsets"]; ok {
				if dSubsets, ok := subsets.([]interface{}); ok {
					for _, subset := range dSubsets {
						if innerSubset, ok := subset.(map[string]interface{}); ok {
							subsetName := innerSubset["name"]
							if labels, ok := innerSubset["labels"]; ok {
								if dLabels, ok := labels.(map[string]interface{}); ok {
									if versionValue, ok := dLabels[cfg.VersionFilterLabelName]; ok && versionValue == version {
										foundSubsets = append(foundSubsets, subsetName.(string))
									}
								}
							}
						}
					}
				}
			}
		}
	}
	return foundSubsets
}

// CheckDestinationRuleCircuitBreaker returns true if the destinationRule object includes a trafficPolicy configuration
// on connectionPool or outlierDetection.
// TrafficPolicy configuration can be defined at service level or per subset defined by a version.
// It returns false otherwise.
func CheckDestinationRuleCircuitBreaker(destinationRule IstioObject, namespace string, serviceName string, version string) bool {
	if destinationRule == nil || destinationRule.GetSpec() == nil {
		return false
	}
	cfg := config.Get()
	if dName, ok := destinationRule.GetSpec()["name"]; ok && dName == serviceName {
		if trafficPolicy, ok := destinationRule.GetSpec()["trafficPolicy"]; ok && checkTrafficPolicy(trafficPolicy) {
			return true
		}
		if subsets, ok := destinationRule.GetSpec()["subsets"]; ok {
			if dSubsets, ok := subsets.([]interface{}); ok {
				for _, subset := range dSubsets {
					if innerSubset, ok := subset.(map[string]interface{}); ok {
						if trafficPolicy, ok := innerSubset["trafficPolicy"]; ok && checkTrafficPolicy(trafficPolicy) {
							if labels, ok := innerSubset["labels"]; ok {
								if dLabels, ok := labels.(map[string]interface{}); ok && version != "" {
									if dLabels[cfg.VersionFilterLabelName] == version {
										return true
									}
								}
							}
						}
					}
				}
			}
		}
	}
	return false
}

// Helper method to check if exist a route with a destination and a subset defined for a httpRoute or tcpRoute in a VirtualService
func checkSubsetRoute(routes interface{}, serviceName string, subsets []string) bool {
	if httpTcpRoutes, ok := routes.([]interface{}); ok {
		for _, httpTcpRoute := range httpTcpRoutes {
			if dHttpTcpRoute, ok := httpTcpRoute.(map[string]interface{}); ok {
				if route, ok := dHttpTcpRoute["route"]; ok {
					if dRoutes, ok := route.([]interface{}); ok {
						for _, dRoute := range dRoutes {
							if innerRoute, ok := dRoute.(map[string]interface{}); ok {
								if destination, ok := innerRoute["destination"]; ok {
									if dDestination, ok := destination.(map[string]interface{}); ok {
										if dName, ok := dDestination["name"]; ok && dName == serviceName {
											if dSubset, ok := dDestination["subset"]; ok {
												for _, subsetName := range subsets {
													if dSubset == subsetName {
														return true
													}
												}
											}
										}
									}
								}
							}
						}
					}
				}
			}
		}
	}
	return false
}

// Helper method to check if a trafficPolicy has defined a connetionPool or outlierDetection element
func checkTrafficPolicy(trafficPolicy interface{}) bool {
	if trafficPolicy == nil {
		return false
	}
	if dTrafficPolicy, ok := trafficPolicy.(map[string]interface{}); ok {
		if _, ok := dTrafficPolicy["connectionPool"]; ok {
			return true
		}
		if _, ok := dTrafficPolicy["outlierDetection"]; ok {
			return true
		}
	}
	return false
}

func filterByDestination(spec map[string]interface{}, namespace string, serviceName string, version string) bool {
	if spec == nil {
		return false
	}
	cfg := config.Get()
	if destination, ok := spec["destination"]; ok {
		dest, ok := destination.(map[string]interface{})
		if !ok {
			return false
		}
		if dNamespace, ok := dest["namespace"]; ok && dNamespace != namespace {
			return false
		}
		if dName, ok := dest["name"]; ok && dName != serviceName {
			return false
		}
		if dLabels, ok := dest["labels"]; ok && version != "" {
			if labels, ok := dLabels.(map[string]interface{}); ok {
				if labels[cfg.VersionFilterLabelName] == version {
					return true
				}
				return false
			}
		} else {
			// It has not labels defined, but destination is defined on whole service
			return true
		}
	}
	return false
}

func filterByHost(spec map[string]interface{}, hostName string) bool {
	if hosts, ok := spec["hosts"]; ok {
		if hostsArray, ok := hosts.([]interface{}); ok {
			for _, host := range hostsArray {
				if host == hostName {
					return true
				}
			}
		}
	}
	return false
}
