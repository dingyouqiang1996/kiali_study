package kubernetes

import (
	"sync"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"k8s.io/api/apps/v1beta1"
	"k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/kiali/kiali/config"
)

type servicesResponse struct {
	services *v1.ServiceList
	err      error
}

type deploymentsResponse struct {
	deployments *v1beta1.DeploymentList
	err         error
}

type podsResponse struct {
	pods *v1.PodList
	err  error
}

// GetNamespaces returns a list of all namespaces of the cluster.
// It returns a list of all namespaces of the cluster.
// It returns an error on any problem.
func (in *IstioClient) GetNamespaces() (*v1.NamespaceList, error) {
	namespaces, err := in.k8s.CoreV1().Namespaces().List(emptyListOptions)
	if err != nil {
		return nil, err
	}

	return namespaces, nil
}

// GetNamespaceAppsDetails returns a map of apps details (services, deployments and pods) in the given namespace.
// The map key is the app label.
// Entities without app label are gathered under empty-string-key
// It returns an error on any problem.
func (in *IstioClient) GetNamespaceAppsDetails(namespace string) (NamespaceApps, error) {
	allEntities := make(NamespaceApps)
	var err error
	servicesChan, podsChan, deploymentsChan := make(chan servicesResponse), make(chan podsResponse), make(chan deploymentsResponse)
	appLabel := config.Get().IstioLabels.AppLabelName

	go in.getServiceList(namespace, servicesChan)
	go in.getPodsList(namespace, podsChan)
	go in.getDeployments(namespace, deploymentsChan)

	servicesResponse := <-servicesChan
	podsResponse := <-podsChan
	deploymentsResponse := <-deploymentsChan
	for _, service := range servicesResponse.services.Items {
		app := service.Labels[appLabel]
		if appEntities, ok := allEntities[app]; ok {
			appEntities.Services = append(appEntities.Services, service)
		} else {
			allEntities[app] = &AppDetails{
				app:      app,
				Services: []v1.Service{service},
			}
		}
	}
	for _, pod := range podsResponse.pods.Items {
		app := pod.Labels[appLabel]
		if appEntities, ok := allEntities[app]; ok {
			appEntities.Pods = append(appEntities.Pods, pod)
		} else {
			allEntities[app] = &AppDetails{
				app:  app,
				Pods: []v1.Pod{pod},
			}
		}
	}
	for _, depl := range deploymentsResponse.deployments.Items {
		app := depl.Labels[appLabel]
		if appEntities, ok := allEntities[app]; ok {
			appEntities.Deployments = append(appEntities.Deployments, depl)
		} else {
			allEntities[app] = &AppDetails{
				app:         app,
				Deployments: []v1beta1.Deployment{depl},
			}
		}
	}

	if servicesResponse.err != nil {
		err = servicesResponse.err
	} else if podsResponse.err != nil {
		err = podsResponse.err
	} else {
		err = deploymentsResponse.err
	}

	return allEntities, err
}

// TODO I think this method could fail as it is filtering services by app label
// TODO it should use getServicesByDeployment() instead
// GetAppDetails returns the app details (services, deployments and pods) for the input app label.
// It returns an error on any problem.
func (in *IstioClient) GetAppDetails(namespace, app string) (AppDetails, error) {
	var errSvc, errPods, errDepls error
	var wg sync.WaitGroup
	var services *v1.ServiceList
	var pods *v1.PodList
	var depls *v1beta1.DeploymentList
	appLabel := config.Get().IstioLabels.AppLabelName
	opts := meta_v1.ListOptions{LabelSelector: appLabel + "=" + app}
	wg.Add(3)

	go func() {
		defer wg.Done()
		services, errSvc = in.k8s.CoreV1().Services(namespace).List(opts)
	}()
	go func() {
		defer wg.Done()
		pods, errPods = in.k8s.CoreV1().Pods(namespace).List(opts)
	}()
	go func() {
		defer wg.Done()
		depls, errDepls = in.k8s.AppsV1beta1().Deployments(namespace).List(opts)
	}()

	wg.Wait()

	details := AppDetails{
		Services:    []v1.Service{},
		Pods:        []v1.Pod{},
		Deployments: []v1beta1.Deployment{},
	}
	if services != nil {
		details.Services = services.Items
	}
	if pods != nil {
		details.Pods = pods.Items
	}
	if depls != nil {
		details.Deployments = depls.Items
	}
	if errSvc != nil {
		return details, errSvc
	} else if errPods != nil {
		return details, errPods
	}
	return details, errDepls
}

// GetServices returns a list of services for a given namespace.
// If selectorLabels is defined the list of services is filtered for those that matches Services selector labels.
// It returns an error on any problem.
func (in *IstioClient) GetServices(namespace string, selectorLabels map[string]string) (*v1.ServiceList, error) {
	allServices, err := in.k8s.CoreV1().Services(namespace).List(emptyListOptions)
	if selectorLabels == nil {
		return allServices, err
	}
	var services []v1.Service
	for _, svc := range allServices.Items {
		svcSelector := labels.Set(svc.Spec.Selector).AsSelector()
		if svcSelector.Matches(labels.Set(selectorLabels)) {
			services = append(services, svc)
		}
	}
	return &v1.ServiceList{Items: services}, err
}

// GetDeploymentsBySelector returns a list of deployments for a given namespace and a set of labels.
// It returns an error on any problem.
func (in *IstioClient) GetDeployments(namespace string, labelSelector string) (*v1beta1.DeploymentList, error) {
	return in.k8s.AppsV1beta1().Deployments(namespace).List(meta_v1.ListOptions{LabelSelector: labelSelector})
}

// GetService returns the definition of a specific service.
// It returns an error on any problem.
func (in *IstioClient) GetService(namespace, serviceName string) (*v1.Service, error) {
	return in.k8s.CoreV1().Services(namespace).Get(serviceName, emptyGetOptions)
}

// GetEndpoints return the list of endpoint of a specific service.
// It returns an error on any problem.
func (in *IstioClient) GetEndpoints(namespace, serviceName string) (*v1.Endpoints, error) {
	return in.k8s.CoreV1().Endpoints(namespace).Get(serviceName, emptyGetOptions)
}

// GetDeployment returns the definition of a specific deployment.
// It returns an error on any problem.
func (in *IstioClient) GetDeployment(namespace, deploymentName string) (*v1beta1.Deployment, error) {
	return in.k8s.AppsV1beta1().Deployments(namespace).Get(deploymentName, emptyGetOptions)
}

// GetPods returns the pods definitions for a given set of labels.
// An empty labelSelector will fetch all pods found per a namespace.
// It returns an error on any problem.
func (in *IstioClient) GetPods(namespace, labelSelector string) (*v1.PodList, error) {
	// An empty selector is ambiguous in the go client, could mean either "select all" or "select none"
	// Here we assume empty == select all
	// (see also https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#label-selectors)
	return in.k8s.CoreV1().Pods(namespace).List(meta_v1.ListOptions{LabelSelector: labelSelector})
}

func (in *IstioClient) getServiceList(namespace string, servicesChan chan servicesResponse) {
	services, err := in.k8s.CoreV1().Services(namespace).List(emptyListOptions)
	servicesChan <- servicesResponse{services: services, err: err}
}

func (in *IstioClient) getPodsList(namespace string, podsChan chan podsResponse) {
	pods, err := in.GetPods(namespace, "")
	podsChan <- podsResponse{pods: pods, err: err}
}

func (in *IstioClient) getDeployments(namespace string, deploymentsChan chan deploymentsResponse) {
	deployments, err := in.k8s.AppsV1beta1().Deployments(namespace).List(emptyListOptions)
	deploymentsChan <- deploymentsResponse{deployments: deployments, err: err}
}

// GetSelectorAsString extracts the Selector used by a Deployment
// Returns a selector represented as a plain string
func GetSelectorAsString(deployment *v1beta1.Deployment) (string, error) {
	selector, err := meta_v1.LabelSelectorAsMap(deployment.Spec.Selector)
	if err != nil {
		return "", err
	}
	return labels.FormatLabels(selector), nil
}

// NewNotFound is a helper method to create a NotFound error similar as used by the kubernetes client.
// This method helps upper layers to send a explicit NotFound error without querying the backend.
func NewNotFound(name, group, resource string) error {
	return errors.NewNotFound(schema.GroupResource{Group: group, Resource: resource}, name)
}
