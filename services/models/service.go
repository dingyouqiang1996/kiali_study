package models

import (
	"k8s.io/api/core/v1"

	"github.com/kiali/kiali/kubernetes"
	"github.com/kiali/kiali/prometheus"
)

type ServiceOverview struct {
	// Name of the Service
	// required: true
	// example: reviews-v1
	Name string `json:"name"`
	// Define if Pods related to this Service has an IstioSidecar deployed
	// required: true
	// example: true
	IstioSidecar bool `json:"istioSidecar"`
	// Has label app
	// required: true
	// example: true
	AppLabel bool `json:"appLabel"`
}

type ServiceList struct {
	Namespace Namespace         `json:"namespace"`
	Services  []ServiceOverview `json:"services"`
}

type ServiceDetails struct {
	Service          Service                     `json:"service"`
	IstioSidecar     bool                        `json:"istioSidecar"`
	Endpoints        Endpoints                   `json:"endpoints"`
	VirtualServices  VirtualServices             `json:"virtualServices"`
	DestinationRules DestinationRules            `json:"destinationRules"`
	Dependencies     map[string][]SourceWorkload `json:"dependencies"`
	Workloads        WorkloadOverviews           `json:"workloads"`
	Health           ServiceHealth               `json:"health"`
}

type Services []*Service
type Service struct {
	Name            string            `json:"name"`
	CreatedAt       string            `json:"createdAt"`
	ResourceVersion string            `json:"resourceVersion"`
	Namespace       Namespace         `json:"namespace"`
	Labels          map[string]string `json:"labels"`
	Type            string            `json:"type"`
	Ip              string            `json:"ip"`
	Ports           Ports             `json:"ports"`
}

// SourceWorkload holds workload identifiers used for service dependencies
type SourceWorkload struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

func (ss *Services) Parse(services []v1.Service) {
	if ss == nil {
		return
	}

	for _, item := range services {
		service := &Service{}
		service.Parse(&item)
		*ss = append(*ss, service)
	}
}

func (s *Service) Parse(service *v1.Service) {
	if service != nil {
		s.Name = service.Name
		s.Namespace = Namespace{Name: service.Namespace}
		s.Labels = service.Labels
		s.Type = string(service.Spec.Type)
		s.Ip = service.Spec.ClusterIP
		s.CreatedAt = formatTime(service.CreationTimestamp.Time)
		s.ResourceVersion = service.ResourceVersion
		(&s.Ports).Parse(service.Spec.Ports)
	}
}

func (s *ServiceDetails) SetService(svc *v1.Service) {
	s.Service.Parse(svc)
}

func (s *ServiceDetails) SetEndpoints(eps *v1.Endpoints) {
	(&s.Endpoints).Parse(eps)
}

func (s *ServiceDetails) SetPods(pods []v1.Pod) {
	mPods := Pods{}
	mPods.Parse(pods)
	s.IstioSidecar = mPods.HasIstioSideCar()
}

func (s *ServiceDetails) SetVirtualServices(vs []kubernetes.IstioObject) {
	(&s.VirtualServices).Parse(vs)
}

func (s *ServiceDetails) SetDestinationRules(dr []kubernetes.IstioObject) {
	(&s.DestinationRules).Parse(dr)
}

func (s *ServiceDetails) SetSourceWorkloads(sw map[string][]prometheus.Workload) {
	// Transform dependencies for UI
	s.Dependencies = make(map[string][]SourceWorkload)
	for version, workloads := range sw {
		for _, workload := range workloads {
			s.Dependencies[version] = append(s.Dependencies[version], SourceWorkload{
				Name:      workload.Workload,
				Namespace: workload.Namespace,
			})
		}
	}
}
