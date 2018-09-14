package status

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"regexp"
	"strings"

	"github.com/hashicorp/go-version"
	"k8s.io/api/core/v1"
	kube "k8s.io/client-go/kubernetes"

	"github.com/kiali/kiali/config"
	"github.com/kiali/kiali/kubernetes"
	"github.com/kiali/kiali/log"
)

type externalService func() (*ExternalServiceInfo, error)

var (
	// Example Maistra version is:
	//   redhat@redhat-docker.io/maistra-0.1.0-1-3a136c90ec5e308f236e0d7ebb5c4c5e405217f4-unknown
	maistraVersionExpr = regexp.MustCompile(".*maistra-([0-9]+\\.[0-9]+\\.[0-9]+).*")
	istioVersionExpr   = regexp.MustCompile(".*([0-9]+\\.[0-9]+\\.[0-9]+).*")
)

func getVersions() {
	components := []externalService{
		istioVersion,
		prometheusVersion,
		kubernetesVersion,
	}
	for _, comp := range components {
		getVersionComponent(comp)
	}
}

func getVersionComponent(serviceComponent externalService) {
	componentInfo, err := serviceComponent()
	if err == nil {
		info.ExternalServices = append(info.ExternalServices, *componentInfo)
	}
}

func validateVersion(istioReq string, installedVersion string) bool {
	reqWords := strings.Split(istioReq, " ")
	requirementV, errReqV := version.NewVersion(reqWords[1])
	installedV, errInsV := version.NewVersion(installedVersion)
	if errReqV != nil || errInsV != nil {
		return false
	}
	switch operator := reqWords[0]; operator {
	case "==":
		return installedV.Equal(requirementV)
	case ">=":
		return installedV.GreaterThan(requirementV) || installedV.Equal(requirementV)
	case ">":
		return installedV.GreaterThan(requirementV)
	case "<=":
		return installedV.LessThan(requirementV) || installedV.Equal(requirementV)
	case "<":
		return installedV.LessThan(requirementV)
	}
	return false
}

func istioVersion() (*ExternalServiceInfo, error) {
	product := ExternalServiceInfo{Name: "Unknown", Version: "Unknown"}
	istioConfig := config.Get().ExternalServices.Istio
	resp, err := http.Get(istioConfig.UrlServiceVersion)
	if err == nil {
		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)
		if err == nil {
			rawVersion := string(body)

			// First see if we detect Maistra. If it is not Maistra, see if it is upstream Istio.
			// If it is neither then it is some unknown Istio implementation that we do not support.

			maistraVersionStringArr := maistraVersionExpr.FindStringSubmatch(rawVersion)
			if maistraVersionStringArr != nil {
				log.Debugf("Detected Maistra version [%v]", rawVersion)
				if len(maistraVersionStringArr) > 1 {
					product.Name = "Maistra"
					product.Version = maistraVersionStringArr[1] // get regex group #1 ,which is the "#.#.#" version string
					if !validateVersion(config.MaistraVersionSupported, product.Version) {
						info.WarningMessages = append(info.WarningMessages, "Maistra version "+product.Version+" is not supported, the version should be "+config.MaistraVersionSupported)
					}

					// we know this is Maistra - either a supported or unsupported version - return now
					return &product, nil
				}
			}

			istioVersionStringArr := istioVersionExpr.FindStringSubmatch(rawVersion)
			if istioVersionStringArr != nil {
				log.Debugf("Detected Istio version [%v]", rawVersion)
				if len(istioVersionStringArr) > 1 {
					product.Name = "Istio"
					product.Version = istioVersionStringArr[1] // get regex group #1 ,which is the "#.#.#" version string
					if !validateVersion(config.IstioVersionSupported, product.Version) {
						info.WarningMessages = append(info.WarningMessages, "Istio version "+product.Version+" is not supported, the version should be "+config.IstioVersionSupported)
					}
					// we know this is Istio upstream - either a supported or unsupported version - return now
					return &product, nil
				}
			}

			log.Debugf("Detected unknown Istio implementation version [%v]", rawVersion)
			product.Name = "Unknown Istio Implementation"
			product.Version = rawVersion
			info.WarningMessages = append(info.WarningMessages, "Unknown Istio implementation version "+product.Version+" is not recognized, thus not supported.")
			return &product, nil
		}
	}
	return nil, err
}

type p8sResponseVersion struct {
	Version  string `json:"version"`
	Revision string `json:"revision"`
}

func prometheusVersion() (*ExternalServiceInfo, error) {
	product := ExternalServiceInfo{}
	prometheusV := new(p8sResponseVersion)
	prometheusUrl := config.Get().ExternalServices.PrometheusServiceURL
	resp, err := http.Get(prometheusUrl + "/version")
	if err == nil {
		defer resp.Body.Close()
		err = json.NewDecoder(resp.Body).Decode(&prometheusV)
		if err == nil {
			product.Name = "Prometheus"
			product.Version = prometheusV.Version
			return &product, nil
		}
	}
	return nil, err
}

const (
	// These constants are tweaks to the k8s client I think once are set up they won't change so no need to put them on the config
	// Default QPS and Burst are quite low and those are not designed for a backend that should perform several
	// queries to build an inventory of entities from a k8s backend.
	// Other k8s clients have increased these values to a similar values.
	k8sQPS   = 100
	k8sBurst = 200
)

func kubernetesVersion() (*ExternalServiceInfo, error) {
	product := ExternalServiceInfo{}
	config, err := kubernetes.ConfigClient()
	if err == nil {
		config.QPS = k8sQPS
		config.Burst = k8sBurst
		k8s, err := kube.NewForConfig(config)
		if err == nil {
			serverVersion, err := k8s.Discovery().ServerVersion()
			if err == nil {
				product.Name = "Kubernetes"
				product.Version = serverVersion.GitVersion
				return &product, nil
			}
		}
	}
	return nil, err
}
func getService(namespace string, service string) (*v1.ServiceSpec, error) {
	client, err := kubernetes.NewClient()
	if err != nil {
		return nil, err
	}
	details, err := client.GetServiceDetails(namespace, service)
	if err != nil {
		return nil, err
	}
	return &details.Service.Spec, nil
}
