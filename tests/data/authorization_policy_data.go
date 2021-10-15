package data

import (
	api_security_v1beta1 "istio.io/api/security/v1beta1"
	api_v1beta1 "istio.io/api/type/v1beta1"
	security_v1beta1 "istio.io/client-go/pkg/apis/security/v1beta1"
)

func CreateAuthorizationPolicy(sourceNamespaces, operationMethods, operationHosts []string, selector map[string]string) *security_v1beta1.AuthorizationPolicy {
	ap := security_v1beta1.AuthorizationPolicy{}
	ap.Name = "auth-policy"
	ap.Namespace = "bookinfo"
	ap.ClusterName = "svc.cluster.local"
	ap.Spec.Selector = &api_v1beta1.WorkloadSelector{
		MatchLabels: selector,
	}
	ap.Spec.Rules = []*api_security_v1beta1.Rule{
		{
			From: []*api_security_v1beta1.Rule_From{
				{
					Source: &api_security_v1beta1.Source{
						Namespaces: sourceNamespaces,
					},
				},
			},
			To: []*api_security_v1beta1.Rule_To{
				{
					Operation: &api_security_v1beta1.Operation{
						Methods: operationMethods,
						Hosts:   operationHosts,
					},
				},
			},
			When: []*api_security_v1beta1.Condition{
				{
					Key:    "request.headers",
					Values: []string{"HTTP"},
				},
			},
		},
	}
	return &ap
}
