package business

import (
	"errors"
	"fmt"
	"sync"

	"github.com/kiali/kiali/kubernetes"
	"github.com/kiali/kiali/log"
	"github.com/kiali/kiali/models"
	"github.com/kiali/kiali/prometheus/internalmetrics"
)

type IstioConfigService struct {
	k8s kubernetes.IstioClientInterface
}

type IstioConfigCriteria struct {
	Namespace                string
	IncludeGateways          bool
	IncludeVirtualServices   bool
	IncludeDestinationRules  bool
	IncludeServiceEntries    bool
	IncludeRules             bool
	IncludeAdapters          bool
	IncludeTemplates         bool
	IncludeQuotaSpecs        bool
	IncludeQuotaSpecBindings bool
	IncludePolicies          bool
}

const (
	VirtualServices   = "virtualservices"
	DestinationRules  = "destinationrules"
	ServiceEntries    = "serviceentries"
	Gateways          = "gateways"
	Rules             = "rules"
	Adapters          = "adapters"
	Templates         = "templates"
	QuotaSpecs        = "quotaspecs"
	QuotaSpecBindings = "quotaspecbindings"
	Policies          = "policies"
)

var resourceTypesToAPI = map[string]string{
	DestinationRules:  "networking.istio.io",
	VirtualServices:   "networking.istio.io",
	ServiceEntries:    "networking.istio.io",
	Gateways:          "networking.istio.io",
	Adapters:          "config.istio.io",
	Templates:         "config.istio.io",
	Rules:             "config.istio.io",
	QuotaSpecs:        "config.istio.io",
	QuotaSpecBindings: "config.istio.io",
	Policies:          "authentication.istio.io",
}

// GetIstioConfigList returns a list of Istio routing objects, Mixer Rules, (etc.)
// per a given Namespace.
func (in *IstioConfigService) GetIstioConfigList(criteria IstioConfigCriteria) (models.IstioConfigList, error) {
	var err error
	promtimer := internalmetrics.GetGoFunctionMetric("business", "IstioConfigService", "GetIstioConfigList")
	defer promtimer.ObserveNow(&err)

	if criteria.Namespace == "" {
		return models.IstioConfigList{}, errors.New("GetIstioConfigList needs a non empty Namespace")
	}
	istioConfigList := models.IstioConfigList{
		Namespace:         models.Namespace{Name: criteria.Namespace},
		Gateways:          models.Gateways{},
		VirtualServices:   models.VirtualServices{Items: []models.VirtualService{}},
		DestinationRules:  models.DestinationRules{Items: []models.DestinationRule{}},
		ServiceEntries:    models.ServiceEntries{},
		Rules:             models.IstioRules{},
		Adapters:          models.IstioAdapters{},
		Templates:         models.IstioTemplates{},
		QuotaSpecs:        models.QuotaSpecs{},
		QuotaSpecBindings: models.QuotaSpecBindings{},
		Policies:          models.Policies{},
	}
	var gg, vs, dr, se, qs, qb, aa, tt, mr, pc []kubernetes.IstioObject
	var ggErr, vsErr, drErr, seErr, mrErr, qsErr, qbErr, aaErr, ttErr, pcErr error
	var wg sync.WaitGroup
	wg.Add(10)

	go func() {
		defer wg.Done()
		if criteria.IncludeGateways {
			if gg, ggErr = in.k8s.GetGateways(criteria.Namespace); ggErr == nil {
				(&istioConfigList.Gateways).Parse(gg)
			}
		}
	}()

	go func() {
		defer wg.Done()
		if criteria.IncludeVirtualServices {
			if vs, vsErr = in.k8s.GetVirtualServices(criteria.Namespace, ""); vsErr == nil {
				(&istioConfigList.VirtualServices).Parse(vs)
			}
		}
	}()

	go func() {
		defer wg.Done()
		if criteria.IncludeDestinationRules {
			if dr, drErr = in.k8s.GetDestinationRules(criteria.Namespace, ""); drErr == nil {
				(&istioConfigList.DestinationRules).Parse(dr)
			}
		}
	}()

	go func() {
		defer wg.Done()
		if criteria.IncludeServiceEntries {
			if se, seErr = in.k8s.GetServiceEntries(criteria.Namespace); seErr == nil {
				(&istioConfigList.ServiceEntries).Parse(se)
			}
		}
	}()

	go func() {
		defer wg.Done()
		if criteria.IncludeRules {
			if mr, mrErr = in.k8s.GetIstioRules(criteria.Namespace); mrErr == nil {
				istioConfigList.Rules = models.CastIstioRulesCollection(mr)
			}
		}
	}()

	go func() {
		defer wg.Done()
		if criteria.IncludeAdapters {
			if aa, aaErr = in.k8s.GetAdapters(criteria.Namespace); aaErr == nil {
				istioConfigList.Adapters = models.CastIstioAdaptersCollection(aa)
			}
		}
	}()

	go func() {
		defer wg.Done()
		if criteria.IncludeTemplates {
			if tt, ttErr = in.k8s.GetTemplates(criteria.Namespace); ttErr == nil {
				istioConfigList.Templates = models.CastIstioTemplatesCollection(tt)
			}
		}
	}()

	go func() {
		defer wg.Done()
		if criteria.IncludeQuotaSpecs {
			if qs, qsErr = in.k8s.GetQuotaSpecs(criteria.Namespace); qsErr == nil {
				(&istioConfigList.QuotaSpecs).Parse(qs)
			}
		}
	}()

	go func() {
		defer wg.Done()
		if criteria.IncludeQuotaSpecBindings {
			if qb, qbErr = in.k8s.GetQuotaSpecBindings(criteria.Namespace); qbErr == nil {
				(&istioConfigList.QuotaSpecBindings).Parse(qb)
			}
		}
	}()

	go func() {
		defer wg.Done()
		if criteria.IncludePolicies {
			if pc, pcErr = in.k8s.GetPolicies(criteria.Namespace); pcErr == nil {
				(&istioConfigList.Policies).Parse(pc)
			}
		}
	}()

	wg.Wait()

	for _, genErr := range []error{ggErr, vsErr, drErr, seErr, mrErr, qsErr, qbErr, aaErr, ttErr} {
		if genErr != nil {
			err = genErr
			return models.IstioConfigList{}, err
		}
	}

	return istioConfigList, nil
}

// GetIstioConfigDetails returns a specific Istio configuration object.
// It uses following parameters:
// - "namespace": 		namespace where configuration is stored
// - "objectType":		type of the configuration
// - "objectSubtype":	subtype of the configuration, used when objectType == "adapters" or "templates", empty/not used otherwise
// - "object":			name of the configuration
func (in *IstioConfigService) GetIstioConfigDetails(namespace, objectType, objectSubtype, object string) (models.IstioConfigDetails, error) {
	var err error
	promtimer := internalmetrics.GetGoFunctionMetric("business", "IstioConfigService", "GetIstioConfigDetails")
	defer promtimer.ObserveNow(&err)

	istioConfigDetail := models.IstioConfigDetails{}
	istioConfigDetail.Namespace = models.Namespace{Name: namespace}
	istioConfigDetail.ObjectType = objectType
	var gw, vs, dr, se, qs, qb, r, a, t kubernetes.IstioObject
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		canUpdate, canDelete := getUpdateDeletePermissions(in.k8s, namespace, objectType, objectSubtype)
		istioConfigDetail.Permissions = models.ResourcePermissions{
			Update: canUpdate,
			Delete: canDelete,
		}
	}()

	switch objectType {
	case Gateways:
		if gw, err = in.k8s.GetGateway(namespace, object); err == nil {
			istioConfigDetail.Gateway = &models.Gateway{}
			istioConfigDetail.Gateway.Parse(gw)
		}
	case VirtualServices:
		if vs, err = in.k8s.GetVirtualService(namespace, object); err == nil {
			istioConfigDetail.VirtualService = &models.VirtualService{}
			istioConfigDetail.VirtualService.Parse(vs)
		}
	case DestinationRules:
		if dr, err = in.k8s.GetDestinationRule(namespace, object); err == nil {
			istioConfigDetail.DestinationRule = &models.DestinationRule{}
			istioConfigDetail.DestinationRule.Parse(dr)
		}
	case ServiceEntries:
		if se, err = in.k8s.GetServiceEntry(namespace, object); err == nil {
			istioConfigDetail.ServiceEntry = &models.ServiceEntry{}
			istioConfigDetail.ServiceEntry.Parse(se)
		}
	case Rules:
		if r, err = in.k8s.GetIstioRule(namespace, object); err == nil {
			istioRule := models.CastIstioRule(r)
			istioConfigDetail.Rule = &istioRule
		}
	case Adapters:
		if a, err = in.k8s.GetAdapter(namespace, objectSubtype, object); err == nil {
			adapter := models.CastIstioAdapter(a)
			istioConfigDetail.Adapter = &adapter
		}
	case Templates:
		if t, err = in.k8s.GetTemplate(namespace, objectSubtype, object); err == nil {
			template := models.CastIstioTemplate(t)
			istioConfigDetail.Template = &template
		}
	case QuotaSpecs:
		if qs, err = in.k8s.GetQuotaSpec(namespace, object); err == nil {
			istioConfigDetail.QuotaSpec = &models.QuotaSpec{}
			istioConfigDetail.QuotaSpec.Parse(qs)
		}
	case QuotaSpecBindings:
		if qb, err = in.k8s.GetQuotaSpecBinding(namespace, object); err == nil {
			istioConfigDetail.QuotaSpecBinding = &models.QuotaSpecBinding{}
			istioConfigDetail.QuotaSpecBinding.Parse(qb)
		}
	default:
		err = fmt.Errorf("Object type not found: %v", objectType)
	}

	wg.Wait()

	return istioConfigDetail, err
}

// GetIstioAPI provides the Kubernetes API that manages this Istio resource type
// or empty string if it's not managed
func GetIstioAPI(resourceType string) string {
	return resourceTypesToAPI[resourceType]
}

// DeleteIstioConfigDetail deletes the given Istio resource
func (in *IstioConfigService) DeleteIstioConfigDetail(api, namespace, resourceType, resourceSubtype, name string) (err error) {
	promtimer := internalmetrics.GetGoFunctionMetric("business", "IstioConfigService", "DeleteIstioConfigDetail")
	defer promtimer.ObserveNow(&err)

	if resourceType == Adapters || resourceType == Templates {
		err = in.k8s.DeleteIstioObject(api, namespace, resourceSubtype, name)
	} else {
		err = in.k8s.DeleteIstioObject(api, namespace, resourceType, name)
	}
	return err
}

func (in *IstioConfigService) UpdateIstioConfigDetail(api, namespace, resourceType, resourceSubtype, name, jsonPatch string) (models.IstioConfigDetails, error) {
	var err error
	promtimer := internalmetrics.GetGoFunctionMetric("business", "IstioConfigService", "UpdateIstioConfigDetail")
	defer promtimer.ObserveNow(&err)

	updatedType := resourceType
	if resourceType == Adapters || resourceType == Templates {
		updatedType = resourceSubtype
	}

	var result kubernetes.IstioObject
	istioConfigDetail := models.IstioConfigDetails{}
	istioConfigDetail.Namespace = models.Namespace{Name: namespace}
	istioConfigDetail.ObjectType = resourceType

	result, err = in.k8s.UpdateIstioObject(api, namespace, updatedType, name, jsonPatch)
	if err != nil {
		return istioConfigDetail, err
	}

	switch resourceType {
	case Gateways:
		istioConfigDetail.Gateway = &models.Gateway{}
		istioConfigDetail.Gateway.Parse(result)
	case VirtualServices:
		istioConfigDetail.VirtualService = &models.VirtualService{}
		istioConfigDetail.VirtualService.Parse(result)
	case DestinationRules:
		istioConfigDetail.DestinationRule = &models.DestinationRule{}
		istioConfigDetail.DestinationRule.Parse(result)
	case ServiceEntries:
		istioConfigDetail.ServiceEntry = &models.ServiceEntry{}
		istioConfigDetail.ServiceEntry.Parse(result)
	case Rules:
		istioRule := models.CastIstioRule(result)
		istioConfigDetail.Rule = &istioRule
	case Adapters:
		adapter := models.CastIstioAdapter(result)
		istioConfigDetail.Adapter = &adapter
	case Templates:
		template := models.CastIstioTemplate(result)
		istioConfigDetail.Template = &template
	case QuotaSpecs:
		istioConfigDetail.QuotaSpec = &models.QuotaSpec{}
		istioConfigDetail.QuotaSpec.Parse(result)
	case QuotaSpecBindings:
		istioConfigDetail.QuotaSpecBinding = &models.QuotaSpecBinding{}
		istioConfigDetail.QuotaSpecBinding.Parse(result)
	default:
		err = fmt.Errorf("Object type not found: %v", resourceType)
	}
	return istioConfigDetail, err
}

func getUpdateDeletePermissions(k8s kubernetes.IstioClientInterface, namespace, objectType, objectSubtype string) (bool, bool) {
	var canPatch, canUpdate, canDelete bool
	if api, ok := resourceTypesToAPI[objectType]; ok {
		// objectType will always match the api used in adapters/templates
		// but if objectSubtype is present it should be used as resourceType
		resourceType := objectType
		if objectSubtype != "" {
			resourceType = objectSubtype
		}
		ssars, permErr := k8s.GetSelfSubjectAccessReview(namespace, api, resourceType, []string{"patch", "update", "delete"})
		if permErr == nil {
			for _, ssar := range ssars {
				if ssar.Spec.ResourceAttributes != nil {
					switch ssar.Spec.ResourceAttributes.Verb {
					case "patch":
						canPatch = ssar.Status.Allowed
					case "update":
						canUpdate = ssar.Status.Allowed
					case "delete":
						canDelete = ssar.Status.Allowed
					}
				}
			}
		} else {
			log.Errorf("Error getting permissions [namespace: %s, api: %s, resourceType: %s]: %v", namespace, api, resourceType, permErr)
		}
	}
	return canUpdate || canPatch, canDelete
}
