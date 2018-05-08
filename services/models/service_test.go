package models

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"k8s.io/api/apps/v1beta1"
	autoscalingV1 "k8s.io/api/autoscaling/v1"
	"k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kiali/kiali/kubernetes"
)

func TestServiceDetailParsing(t *testing.T) {
	assert := assert.New(t)

	service := Service{}
	service.Name = "service"
	service.Namespace = Namespace{"namespace"}
	service.SetServiceDetails(fakeServiceDetails(), fakeIstioDetails(), fakePrometheusDetails())

	// Kubernetes Details
	assert.Equal(service.Name, "service")
	assert.Equal(service.Namespace.Name, "namespace")
	assert.Equal(service.CreatedAt, "2018-03-08T17:44:00+03:00")
	assert.Equal(service.ResourceVersion, "1234")
	assert.Equal(service.Type, "ClusterIP")
	assert.Equal(service.Ip, "fromservice")
	assert.Equal(service.Labels, map[string]string{"label1": "labelName1", "label2": "labelName2"})
	assert.Equal(service.Ports, Ports{
		Port{Name: "http", Protocol: "TCP", Port: 3001},
		Port{Name: "http", Protocol: "TCP", Port: 3000}})
	assert.Equal(service.Endpoints, Endpoints{
		Endpoint{
			Addresses: Addresses{
				Address{Kind: "Pod", Name: "recommendation-v1", IP: "172.17.0.9"},
				Address{Kind: "Pod", Name: "recommendation-v2", IP: "172.17.0.8"},
			},
			Ports: Ports{
				Port{Name: "http", Protocol: "TCP", Port: 3001},
				Port{Name: "http", Protocol: "TCP", Port: 3000},
			}}})

	assert.Equal(*service.Deployments[0], Deployment{
		Name:                "reviews-v1",
		Labels:              map[string]string{"apps": "reviews", "version": "v1"},
		CreatedAt:           "2018-03-08T17:44:00+03:00",
		ResourceVersion:     "1234",
		Replicas:            3,
		AvailableReplicas:   1,
		UnavailableReplicas: 2,
		Autoscaler: Autoscaler{
			Name:                            "reviews-v1",
			Labels:                          map[string]string{"apps": "reviews", "version": "v1"},
			CreatedAt:                       "2018-03-08T17:44:00+03:00",
			MinReplicas:                     1,
			MaxReplicas:                     10,
			TargetCPUUtilizationPercentage:  50,
			CurrentReplicas:                 3,
			DesiredReplicas:                 4,
			ObservedGeneration:              50,
			CurrentCPUUtilizationPercentage: 70}})

	assert.Equal(*service.Deployments[1], Deployment{
		Name:                "reviews-v2",
		Labels:              map[string]string{"apps": "reviews", "version": "v2"},
		CreatedAt:           "2018-03-08T17:45:00+03:00",
		ResourceVersion:     "1234",
		Replicas:            3,
		AvailableReplicas:   3,
		UnavailableReplicas: 0,
		Autoscaler: Autoscaler{
			Name:                            "reviews-v2",
			Labels:                          map[string]string{"apps": "reviews", "version": "v2"},
			CreatedAt:                       "2018-03-08T17:45:00+03:00",
			MinReplicas:                     1,
			MaxReplicas:                     10,
			TargetCPUUtilizationPercentage:  50,
			CurrentReplicas:                 3,
			DesiredReplicas:                 2,
			ObservedGeneration:              50,
			CurrentCPUUtilizationPercentage: 30}})

	// Istio Details
	assert.Equal(service.RouteRules, RouteRules{
		RouteRule{
			CreatedAt:       "2018-03-08T17:46:00+03:00",
			ResourceVersion: "1234",
			Destination: map[string]string{
				"name":      "reviews",
				"namespace": "tutorial"},
			Precedence: 1,
			Route: map[string]map[string]string{
				"labels": {
					"name":      "version",
					"namespace": "v1"}},
			HttpFault: map[string]map[string]string{
				"abort": {
					"percent":    "50",
					"httpStatus": "503",
				},
			}},
		RouteRule{
			CreatedAt:       "2018-03-08T17:46:00+03:00",
			ResourceVersion: "1234",
			Destination: map[string]string{
				"name":      "reviews",
				"namespace": "tutorial"},
			Precedence: 1,
			Route: map[string]map[string]string{
				"labels": {
					"name":      "version",
					"namespace": "v3"}}}})

	assert.Equal(service.DestinationPolicies, DestinationPolicies{
		DestinationPolicy{
			CreatedAt:       "2018-03-08T17:47:00+03:00",
			ResourceVersion: "1234",
			Source: map[string]string{
				"name": "recommendation"},
			Destination: map[string]string{
				"name":      "reviews",
				"namespace": "tutorial"},
			LoadBalancing: map[string]string{
				"name": "RANDOM"},
		},
		DestinationPolicy{
			CreatedAt:       "2018-03-08T17:47:00+03:00",
			ResourceVersion: "1234",
			Destination: map[string]interface{}{
				"name":      "reviews",
				"namespace": "tutorial",
				"labels": map[string]string{
					"version": "v2"}},
			CircuitBreaker: map[string]interface{}{
				"simpleCb": map[string]interface{}{
					"maxConnections":               1,
					"httpMaxPendingRequests":       1,
					"sleepWindow":                  "2m",
					"httpDetectionInterval":        "1s",
					"httpMaxEjectionPercent":       100,
					"httpConsecutiveErrors":        1,
					"httpMaxRequestsPerConnection": 1,
				}},
		}})

	assert.Equal(service.VirtualServices, VirtualServices{
		VirtualService{
			Name:            "reviews",
			CreatedAt:       "2018-03-08T17:47:00+03:00",
			ResourceVersion: "1234",
			Hosts: []interface{}{
				"reviews",
			},
			Http: []interface{}{
				map[string]interface{}{
					"route": []interface{}{
						map[string]interface{}{
							"destination": map[string]interface{}{
								"name":   "reviews",
								"subset": "v2",
							},
							"weight": 50,
						},
						map[string]interface{}{
							"destination": map[string]interface{}{
								"name":   "reviews",
								"subset": "v3",
							},
							"weight": 50,
						},
					},
				},
			},
		},
		VirtualService{
			Name:            "ratings",
			CreatedAt:       "2018-03-08T17:47:00+03:00",
			ResourceVersion: "1234",
			Hosts: []interface{}{
				"reviews",
			},
			Http: []interface{}{
				map[string]interface{}{
					"match": []interface{}{
						map[string]interface{}{
							"headers": map[string]interface{}{
								"cookie": map[string]interface{}{
									"regex": "^(.*?;)?(user=jason)(;.*)?$",
								},
							},
						},
					},
					"fault": map[string]interface{}{
						"delay": map[string]interface{}{
							"percent":    100,
							"fixedDelay": "7s",
						},
					},
					"route": []interface{}{
						map[string]interface{}{
							"destination": map[string]interface{}{
								"name":   "ratings",
								"subset": "v1",
							},
						},
					},
				},
				map[string]interface{}{
					"route": []interface{}{
						map[string]interface{}{
							"destination": map[string]interface{}{
								"name":   "ratings",
								"subset": "v1",
							},
						},
					},
				},
			},
		},
	})

	assert.Equal(service.DestinationRules, DestinationRules{
		DestinationRule{
			Name:            "reviews-destination",
			CreatedAt:       "2018-03-08T17:47:00+03:00",
			ResourceVersion: "1234",
			DestinationName: "reviews",
			Subsets: []interface{}{
				map[string]interface{}{
					"name": "v1",
					"labels": map[string]interface{}{
						"version": "v1",
					},
				},
				map[string]interface{}{
					"name": "v2",
					"labels": map[string]interface{}{
						"version": "v2",
					},
				},
			},
		},
		DestinationRule{
			Name:            "bookinfo-ratings",
			CreatedAt:       "2018-03-08T17:47:00+03:00",
			ResourceVersion: "1234",
			DestinationName: "ratings",
			TrafficPolicy: map[string]interface{}{
				"loadBalancer": map[string]interface{}{
					"simple": "LEAST_CONN",
				},
			},
			Subsets: []interface{}{
				map[string]interface{}{
					"name": "testversion",
					"labels": map[string]interface{}{
						"version": "v3",
					},
					"trafficPolicy": map[string]interface{}{
						"loadBalancer": map[string]interface{}{
							"simple": "ROUND_ROBIN",
						},
					},
				},
			},
		},
	})

	// Prometheus Client
	assert.Equal(service.Dependencies, map[string][]string{
		"v1": {"unknown", "/products", "/reviews"},
		"v2": {"/catalog", "/shares"}})
}

func fakeServiceDetails() *kubernetes.ServiceDetails {
	t1, _ := time.Parse(time.RFC822Z, "08 Mar 18 17:44 +0300")
	t2, _ := time.Parse(time.RFC822Z, "08 Mar 18 17:45 +0300")

	service := &v1.Service{
		ObjectMeta: meta_v1.ObjectMeta{
			Name:              "Name",
			Namespace:         "Namespace",
			CreationTimestamp: meta_v1.NewTime(t1),
			ResourceVersion:   "1234",
			Labels: map[string]string{
				"label1": "labelName1",
				"label2": "labelName2"}},
		Spec: v1.ServiceSpec{
			ClusterIP: "fromservice",
			Type:      "ClusterIP",
			Ports: []v1.ServicePort{
				{
					Name:     "http",
					Protocol: "TCP",
					Port:     3001},
				{
					Name:     "http",
					Protocol: "TCP",
					Port:     3000}}}}

	endpoints := &v1.Endpoints{
		Subsets: []v1.EndpointSubset{
			{
				Addresses: []v1.EndpointAddress{
					{
						IP: "172.17.0.9",
						TargetRef: &v1.ObjectReference{
							Kind: "Pod",
							Name: "recommendation-v1"}},
					{
						IP: "172.17.0.8",
						TargetRef: &v1.ObjectReference{
							Kind: "Pod",
							Name: "recommendation-v2"}},
				},
				Ports: []v1.EndpointPort{
					{Name: "http", Protocol: "TCP", Port: 3001},
					{Name: "http", Protocol: "TCP", Port: 3000},
				}}}}

	deployments := &v1beta1.DeploymentList{
		Items: []v1beta1.Deployment{
			v1beta1.Deployment{
				ObjectMeta: meta_v1.ObjectMeta{
					Name:              "reviews-v1",
					CreationTimestamp: meta_v1.NewTime(t1),
					ResourceVersion:   "1234",
					Labels:            map[string]string{"apps": "reviews", "version": "v1"}},
				Status: v1beta1.DeploymentStatus{
					Replicas:            3,
					AvailableReplicas:   1,
					UnavailableReplicas: 2}},
			v1beta1.Deployment{
				ObjectMeta: meta_v1.ObjectMeta{
					Name:              "reviews-v2",
					CreationTimestamp: meta_v1.NewTime(t2),
					ResourceVersion:   "1234",
					Labels:            map[string]string{"apps": "reviews", "version": "v2"}},
				Status: v1beta1.DeploymentStatus{
					Replicas:            3,
					AvailableReplicas:   3,
					UnavailableReplicas: 0}}}}

	autoscalers := &autoscalingV1.HorizontalPodAutoscalerList{
		Items: []autoscalingV1.HorizontalPodAutoscaler{
			autoscalingV1.HorizontalPodAutoscaler{
				ObjectMeta: meta_v1.ObjectMeta{
					Name:              "reviews-v1",
					Labels:            map[string]string{"apps": "reviews", "version": "v1"},
					CreationTimestamp: meta_v1.NewTime(t1)},
				Spec: autoscalingV1.HorizontalPodAutoscalerSpec{
					ScaleTargetRef: autoscalingV1.CrossVersionObjectReference{
						Name: "reviews-v1"},
					MinReplicas:                    &[]int32{1}[0],
					MaxReplicas:                    10,
					TargetCPUUtilizationPercentage: &[]int32{50}[0]},
				Status: autoscalingV1.HorizontalPodAutoscalerStatus{
					ObservedGeneration:              &[]int64{50}[0],
					CurrentReplicas:                 3,
					DesiredReplicas:                 4,
					CurrentCPUUtilizationPercentage: &[]int32{70}[0]}},
			autoscalingV1.HorizontalPodAutoscaler{
				ObjectMeta: meta_v1.ObjectMeta{
					Name:              "reviews-v2",
					Labels:            map[string]string{"apps": "reviews", "version": "v2"},
					CreationTimestamp: meta_v1.NewTime(t2)},
				Spec: autoscalingV1.HorizontalPodAutoscalerSpec{
					ScaleTargetRef: autoscalingV1.CrossVersionObjectReference{
						Name: "reviews-v2"},
					MinReplicas:                    &[]int32{1}[0],
					MaxReplicas:                    10,
					TargetCPUUtilizationPercentage: &[]int32{50}[0]},
				Status: autoscalingV1.HorizontalPodAutoscalerStatus{
					ObservedGeneration:              &[]int64{50}[0],
					CurrentReplicas:                 3,
					DesiredReplicas:                 2,
					CurrentCPUUtilizationPercentage: &[]int32{30}[0]}}}}

	return &kubernetes.ServiceDetails{service, endpoints, deployments, autoscalers, nil}
}

func fakeIstioDetails() *kubernetes.IstioDetails {
	t1, _ := time.Parse(time.RFC822Z, "08 Mar 18 17:46 +0300")
	t2, _ := time.Parse(time.RFC822Z, "08 Mar 18 17:47 +0300")

	route1 := kubernetes.MockIstioObject{
		ObjectMeta: meta_v1.ObjectMeta{
			CreationTimestamp: meta_v1.NewTime(t1),
			ResourceVersion:   "1234",
		},
		Spec: map[string]interface{}{
			"destination": map[string]string{
				"name":      "reviews",
				"namespace": "tutorial"},
			"precedence": 1,
			"route": map[string]map[string]string{
				"labels": map[string]string{
					"name":      "version",
					"namespace": "v1"}},
			"httpFault": map[string]map[string]string{
				"abort": map[string]string{
					"percent":    "50",
					"httpStatus": "503",
				}}},
	}
	route2 := kubernetes.MockIstioObject{
		ObjectMeta: meta_v1.ObjectMeta{
			CreationTimestamp: meta_v1.NewTime(t1),
			ResourceVersion:   "1234",
		},
		Spec: map[string]interface{}{
			"destination": map[string]string{
				"name":      "reviews",
				"namespace": "tutorial"},
			"precedence": 1,
			"route": map[string]map[string]string{
				"labels": map[string]string{
					"name":      "version",
					"namespace": "v3"}}},
	}
	routes := []kubernetes.IstioObject{&route1, &route2}
	policy1 := kubernetes.MockIstioObject{
		ObjectMeta: meta_v1.ObjectMeta{
			CreationTimestamp: meta_v1.NewTime(t2),
			ResourceVersion:   "1234",
		},
		Spec: map[string]interface{}{
			"source": map[string]string{
				"name": "recommendation",
			},
			"destination": map[string]string{
				"name":      "reviews",
				"namespace": "tutorial",
			},
			"loadBalancing": map[string]string{
				"name": "RANDOM",
			},
		},
	}
	policy2 := kubernetes.MockIstioObject{
		ObjectMeta: meta_v1.ObjectMeta{
			CreationTimestamp: meta_v1.NewTime(t2),
			ResourceVersion:   "1234",
		},
		Spec: map[string]interface{}{
			"destination": map[string]interface{}{
				"name":      "reviews",
				"namespace": "tutorial",
				"labels": map[string]string{
					"version": "v2",
				},
			},
			"circuitBreaker": map[string]interface{}{
				"simpleCb": map[string]interface{}{
					"maxConnections":               1,
					"httpMaxPendingRequests":       1,
					"sleepWindow":                  "2m",
					"httpDetectionInterval":        "1s",
					"httpMaxEjectionPercent":       100,
					"httpConsecutiveErrors":        1,
					"httpMaxRequestsPerConnection": 1,
				},
			},
		},
	}

	policies := []kubernetes.IstioObject{&policy1, &policy2}

	virtualService1 := kubernetes.MockIstioObject{
		ObjectMeta: meta_v1.ObjectMeta{
			Name:              "reviews",
			CreationTimestamp: meta_v1.NewTime(t2),
			ResourceVersion:   "1234",
		},
		Spec: map[string]interface{}{
			"hosts": []interface{}{
				"reviews",
			},
			"http": []interface{}{
				map[string]interface{}{
					"route": []interface{}{
						map[string]interface{}{
							"destination": map[string]interface{}{
								"name":   "reviews",
								"subset": "v2",
							},
							"weight": 50,
						},
						map[string]interface{}{
							"destination": map[string]interface{}{
								"name":   "reviews",
								"subset": "v3",
							},
							"weight": 50,
						},
					},
				},
			},
		},
	}
	virtualService2 := kubernetes.MockIstioObject{
		ObjectMeta: meta_v1.ObjectMeta{
			Name:              "ratings",
			CreationTimestamp: meta_v1.NewTime(t2),
			ResourceVersion:   "1234",
		},
		Spec: map[string]interface{}{
			"hosts": []interface{}{
				"reviews",
			},
			"http": []interface{}{
				map[string]interface{}{
					"match": []interface{}{
						map[string]interface{}{
							"headers": map[string]interface{}{
								"cookie": map[string]interface{}{
									"regex": "^(.*?;)?(user=jason)(;.*)?$",
								},
							},
						},
					},
					"fault": map[string]interface{}{
						"delay": map[string]interface{}{
							"percent":    100,
							"fixedDelay": "7s",
						},
					},
					"route": []interface{}{
						map[string]interface{}{
							"destination": map[string]interface{}{
								"name":   "ratings",
								"subset": "v1",
							},
						},
					},
				},
				map[string]interface{}{
					"route": []interface{}{
						map[string]interface{}{
							"destination": map[string]interface{}{
								"name":   "ratings",
								"subset": "v1",
							},
						},
					},
				},
			},
		},
	}
	virtualServices := []kubernetes.IstioObject{&virtualService1, &virtualService2}

	destinationRule1 := kubernetes.MockIstioObject{
		ObjectMeta: meta_v1.ObjectMeta{
			Name:              "reviews-destination",
			CreationTimestamp: meta_v1.NewTime(t2),
			ResourceVersion:   "1234",
		},
		Spec: map[string]interface{}{
			"name": "reviews",
			"subsets": []interface{}{
				map[string]interface{}{
					"name": "v1",
					"labels": map[string]interface{}{
						"version": "v1",
					},
				},
				map[string]interface{}{
					"name": "v2",
					"labels": map[string]interface{}{
						"version": "v2",
					},
				},
			},
		},
	}
	destinationRule2 := kubernetes.MockIstioObject{
		ObjectMeta: meta_v1.ObjectMeta{
			Name:              "bookinfo-ratings",
			CreationTimestamp: meta_v1.NewTime(t2),
			ResourceVersion:   "1234",
		},
		Spec: map[string]interface{}{
			"name": "ratings",
			"trafficPolicy": map[string]interface{}{
				"loadBalancer": map[string]interface{}{
					"simple": "LEAST_CONN",
				},
			},
			"subsets": []interface{}{
				map[string]interface{}{
					"name": "testversion",
					"labels": map[string]interface{}{
						"version": "v3",
					},
					"trafficPolicy": map[string]interface{}{
						"loadBalancer": map[string]interface{}{
							"simple": "ROUND_ROBIN",
						},
					},
				},
			},
		},
	}
	destinationRules := []kubernetes.IstioObject{&destinationRule1, &destinationRule2}

	return &kubernetes.IstioDetails{routes, policies, virtualServices, destinationRules}
}

func fakePrometheusDetails() map[string][]string {
	return map[string][]string{
		"v1": []string{"unknown", "/products", "/reviews"},
		"v2": []string{"/catalog", "/shares"}}
}
