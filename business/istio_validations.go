package business

import (
	"fmt"
	"sync"

	"github.com/kiali/kiali/business/checkers"
	"github.com/kiali/kiali/kubernetes"
	"github.com/kiali/kiali/models"
	"github.com/kiali/kiali/prometheus/internalmetrics"

	"k8s.io/api/core/v1"
)

type IstioValidationsService struct {
	k8s kubernetes.IstioClientInterface
	ws  WorkloadService
}

type ObjectChecker interface {
	Check() models.IstioValidations
}

// GetServiceValidations returns an IstioValidations object with all the checks found when running
// all the enabled checkers.
func (in *IstioValidationsService) GetServiceValidations(namespace, service string) (models.IstioValidations, error) {
	var err error
	promtimer := internalmetrics.GetGoFunctionMetric("business", "IstioValidationsService", "GetServiceValidations")
	defer promtimer.ObserveNow(&err)

	// Ensure the service exists
	if _, err := in.k8s.GetService(namespace, service); err != nil {
		return nil, err
	}

	// Get Gateways and ServiceEntries to validate VirtualServices
	wg := sync.WaitGroup{}
	errChan := make(chan error, 2)

	istioDetails := kubernetes.IstioDetails{}
	vs := make([]kubernetes.IstioObject, 0)
	drs := make([]kubernetes.IstioObject, 0)
	var services []v1.Service
	var workloads models.WorkloadList

	wg.Add(4)
	go fetch(&vs, namespace, service, in.k8s.GetVirtualServices, &wg, errChan)
	go fetch(&drs, namespace, service, in.k8s.GetDestinationRules, &wg, errChan)
	go in.serviceFetcher(&services, namespace, errChan, &wg)
	go in.fetchWorkloads(&workloads, namespace, errChan, &wg)
	wg.Wait()
	if len(errChan) != 0 {
		err = <-errChan
		return nil, err
	}
	istioDetails.DestinationRules = drs
	istioDetails.VirtualServices = vs
	objectCheckers := []ObjectChecker{
		checkers.VirtualServiceChecker{namespace, drs, vs},
		checkers.DestinationRulesChecker{DestinationRules: drs},
		checkers.NoServiceChecker{Namespace: namespace, Services: services, IstioDetails: &istioDetails, WorkloadList: workloads},
	}

	// Get group validations for same kind istio objects
	return runObjectCheckers(objectCheckers), nil
}

func (in *IstioValidationsService) GetNamespaceValidations(namespace string) (models.NamespaceValidations, error) {
	var err error
	promtimer := internalmetrics.GetGoFunctionMetric("business", "IstioValidationsService", "GetNamespaceValidations")
	defer promtimer.ObserveNow(&err)

	// Ensure the Namespace exists
	if _, err := in.k8s.GetNamespace(namespace); err != nil {
		return nil, err
	}

	// Get all the Istio objects from a Namespace
	istioDetails, err := in.k8s.GetIstioDetails(namespace, "")
	if err != nil {
		return nil, err
	}

	// Get Gateways and ServiceEntries to validate VirtualServices
	wg := sync.WaitGroup{}
	errChan := make(chan error, 4)

	gws := make([]kubernetes.IstioObject, 0)
	ses := make([]kubernetes.IstioObject, 0)
	var services []v1.Service
	var workloads models.WorkloadList

	wg.Add(4)
	go fetchNoEntry(&gws, namespace, in.k8s.GetGateways, &wg, errChan)
	go fetchNoEntry(&ses, namespace, in.k8s.GetServiceEntries, &wg, errChan)
	go in.serviceFetcher(&services, namespace, errChan, &wg)
	go in.fetchWorkloads(&workloads, namespace, errChan, &wg)

	wg.Wait()
	if len(errChan) == 0 {
		istioDetails.Gateways = gws
		istioDetails.ServiceEntries = ses
	} else {
		err = <-errChan
		return nil, err
	}

	objectCheckers := []ObjectChecker{
		checkers.VirtualServiceChecker{namespace, istioDetails.DestinationRules,
			istioDetails.VirtualServices},
		checkers.NoServiceChecker{Namespace: namespace, IstioDetails: istioDetails, Services: services, WorkloadList: workloads},
		checkers.DestinationRulesChecker{DestinationRules: istioDetails.DestinationRules},
	}

	return models.NamespaceValidations{namespace: runObjectCheckers(objectCheckers)}, nil
}

func (in *IstioValidationsService) GetIstioObjectValidations(namespace string, objectType string, object string) (models.IstioValidations, error) {
	var err error
	promtimer := internalmetrics.GetGoFunctionMetric("business", "IstioValidationsService", "GetIstioObjectValidations")
	defer promtimer.ObserveNow(&err)

	// Get only the given Istio Object
	var dr kubernetes.IstioObject
	vss := make([]kubernetes.IstioObject, 0)
	ses := make([]kubernetes.IstioObject, 0)
	drs := make([]kubernetes.IstioObject, 0)
	gws := make([]kubernetes.IstioObject, 0)
	var services []v1.Service
	var workloads models.WorkloadList

	var objectCheckers []ObjectChecker
	istioDetails := kubernetes.IstioDetails{}
	wg := sync.WaitGroup{}
	errChan := make(chan error, 4)

	switch objectType {
	case Gateways:
		// Validations on Gateways are not yet in place
	case VirtualServices:
		wg.Add(5)
		go fetch(&vss, namespace, "", in.k8s.GetVirtualServices, &wg, errChan)
		go fetch(&drs, namespace, "", in.k8s.GetDestinationRules, &wg, errChan)
		go fetchNoEntry(&gws, namespace, in.k8s.GetGateways, &wg, errChan)
		go in.serviceFetcher(&services, namespace, errChan, &wg)
		go in.fetchWorkloads(&workloads, namespace, errChan, &wg)
		// We can block current goroutine for the fifth fetch
		ses, err = in.k8s.GetServiceEntries(namespace)
		if err != nil {
			errChan <- err
		}
		wg.Wait()
		if len(errChan) == 0 {
			istioDetails.ServiceEntries = ses
			istioDetails.VirtualServices = vss
			istioDetails.DestinationRules = drs
			istioDetails.Gateways = gws
			virtualServiceChecker := checkers.VirtualServiceChecker{Namespace: namespace, VirtualServices: istioDetails.VirtualServices, DestinationRules: istioDetails.DestinationRules}
			noServiceChecker := checkers.NoServiceChecker{Namespace: namespace, Services: services, IstioDetails: &istioDetails, WorkloadList: workloads}
			objectCheckers = []ObjectChecker{noServiceChecker, virtualServiceChecker}
		} else {
			err = <-errChan
			close(errChan)
		}
	case DestinationRules:
		// TODO Replicated code from the virtualservices part also.. this package needs an overhaul
		wg.Add(2)
		// TODO Get Workloads here for the no_gateway_checker..
		go in.serviceFetcher(&services, namespace, errChan, &wg)
		go in.fetchWorkloads(&workloads, namespace, errChan, &wg)
		// We can use current goroutine for the second fetch
		drs, err := in.k8s.GetDestinationRules(namespace, "")
		if err != nil {
			errChan <- err
		}
		wg.Wait()
		if len(errChan) == 0 {
			for _, o := range drs {
				meta := o.GetObjectMeta()
				if meta.Name == object {
					dr = o
					break
				}
			}
			istioDetails.DestinationRules = []kubernetes.IstioObject{dr} // Single destination rule only available here, not whole namespace
			destinationRulesChecker := checkers.DestinationRulesChecker{DestinationRules: drs}
			noServiceChecker := checkers.NoServiceChecker{Namespace: namespace, Services: services, IstioDetails: &istioDetails, WorkloadList: workloads}
			objectCheckers = []ObjectChecker{noServiceChecker, destinationRulesChecker}
		} else {
			err = <-errChan
			close(errChan)
		}
	case ServiceEntries:
		// Validations on ServiceEntries are not yet in place
	case Rules:
		// Validations on Istio Rules are not yet in place
	case QuotaSpecs:
		// Validations on QuotaSpecs are not yet in place
	case QuotaSpecBindings:
		// Validations on QuotaSpecBindings are not yet in place
	default:
		err = fmt.Errorf("Object type not found: %v", objectType)
	}

	if objectCheckers == nil || err != nil {
		return models.IstioValidations{}, err
	}

	return runObjectCheckers(objectCheckers).FilterByKey(models.ObjectTypeSingular[objectType], object), nil
}

func runObjectCheckers(objectCheckers []ObjectChecker) models.IstioValidations {
	objectTypeValidations := models.IstioValidations{}

	// Run checks for each IstioObject type
	for _, objectChecker := range objectCheckers {
		objectTypeValidations.MergeValidations(objectChecker.Check())
	}

	return objectTypeValidations
}

func fetch(rValue *[]kubernetes.IstioObject, namespace string, service string, fetcher func(string, string) ([]kubernetes.IstioObject, error), wg *sync.WaitGroup, errChan chan error) {
	defer wg.Done()
	fetched, err := fetcher(namespace, service)
	*rValue = append(*rValue, fetched...)
	if err != nil {
		errChan <- err
	}
}

// Identical to above, but since k8s layer has both (namespace, serviceentry) and (namespace) queries, we need two different functions
func fetchNoEntry(rValue *[]kubernetes.IstioObject, namespace string, fetcher func(string) ([]kubernetes.IstioObject, error), wg *sync.WaitGroup, errChan chan error) {
	defer wg.Done()
	fetched, err := fetcher(namespace)
	*rValue = append(*rValue, fetched...)
	if err != nil {
		errChan <- err
	}
}

func (in *IstioValidationsService) serviceFetcher(rValue *[]v1.Service, namespace string, errChan chan error, wg *sync.WaitGroup) {
	defer wg.Done()
	services, err := in.k8s.GetServices(namespace, nil)
	if err != nil {
		errChan <- err
	} else {
		*rValue = services
	}
}

func (in *IstioValidationsService) fetchWorkloads(rValue *models.WorkloadList, namespace string, errChan chan error, wg *sync.WaitGroup) {
	defer wg.Done()
	workloadList, err := in.ws.GetWorkloadList(namespace)
	if err != nil {
		errChan <- err
	} else {
		*rValue = workloadList
	}
}
