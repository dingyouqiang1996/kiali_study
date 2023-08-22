package tests

import (
	"path"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/stretchr/testify/require"

	"github.com/kiali/kiali/kubernetes"
	"github.com/kiali/kiali/models"
	"github.com/kiali/kiali/tests/integration/utils"
	"github.com/kiali/kiali/tools/cmd"
)

func TestAuthPolicyPrincipalsError(t *testing.T) {
	name := "ratings-policy"
	require := require.New(t)
	filePath := path.Join(cmd.KialiProjectRoot, utils.ASSETS+"/bookinfo-auth-policy-principals.yaml")
	defer utils.DeleteFile(filePath, utils.BOOKINFO)
	require.True(utils.ApplyFile(filePath, utils.BOOKINFO))

	config, err := getConfigDetails(utils.BOOKINFO, name, kubernetes.AuthorizationPolicies, false, require)

	require.Nil(err)
	require.NotNil(config)
	require.Equal(kubernetes.AuthorizationPolicies, config.ObjectType)
	require.Equal(utils.BOOKINFO, config.Namespace.Name)
	require.NotNil(config.AuthorizationPolicy)
	require.Equal(name, config.AuthorizationPolicy.Name)
	require.Equal(utils.BOOKINFO, config.AuthorizationPolicy.Namespace)
	require.NotNil(config.IstioReferences)
	require.NotNil(config.IstioValidation)
	require.Equal(name, config.IstioValidation.Name)
	require.Equal("authorizationpolicy", config.IstioValidation.ObjectType)
	require.False(config.IstioValidation.Valid)
	require.Empty(config.IstioValidation.References)
	require.NotEmpty(config.IstioValidation.Checks)
	require.Len(config.IstioValidation.Checks, 1)
	require.Equal(models.ErrorSeverity, config.IstioValidation.Checks[0].Severity)
	require.Equal("Service Account not found for this principal", config.IstioValidation.Checks[0].Message)
}

func TestServiceEntryLabels(t *testing.T) {
	require := require.New(t)
	filePath := path.Join(cmd.KialiProjectRoot, utils.ASSETS+"/bookinfo-service-entry-labels.yaml")
	defer utils.DeleteFile(filePath, utils.BOOKINFO)
	require.True(utils.ApplyFile(filePath, utils.BOOKINFO))

	// the DR with matching labels with SE
	name := "dest-rule-labels"
	config, err := getConfigDetails(utils.BOOKINFO, name, kubernetes.DestinationRules, false, require)
	require.Nil(err)
	require.NotNil(config)
	require.True(config.IstioValidation.Valid)
	require.Empty(config.IstioValidation.Checks)
}

func TestServiceEntryLabelsNotMatch(t *testing.T) {
	require := require.New(t)
	filePath := path.Join(cmd.KialiProjectRoot, utils.ASSETS+"/bookinfo-service-entry-wrong-labels.yaml")
	defer utils.DeleteFile(filePath, utils.BOOKINFO)
	require.True(utils.ApplyFile(filePath, utils.BOOKINFO))

	// the DR with error, labels not match with SE
	name := "dest-rule-labels-wrong"
	config, err := getConfigDetails(utils.BOOKINFO, name, kubernetes.DestinationRules, false, require)
	require.Nil(err)
	require.NotNil(config)
	require.False(config.IstioValidation.Valid)
	require.NotEmpty(config.IstioValidation.Checks)
	require.Len(config.IstioValidation.Checks, 1)
	require.Equal("This subset's labels are not found in any matching host", config.IstioValidation.Checks[0].Message)
}

func TestK8sGatewaysAddressesError(t *testing.T) {
	name := "gatewayapi"
	require := require.New(t)
	filePath := path.Join(cmd.KialiProjectRoot, utils.ASSETS+"/bookinfo-k8sgateways-addresses.yaml")
	defer utils.DeleteFile(filePath, utils.BOOKINFO)
	require.True(utils.ApplyFile(filePath, utils.BOOKINFO))

	config, err := getConfigDetails(utils.BOOKINFO, name, kubernetes.K8sGateways, true, require)

	require.Nil(err)
	require.NotNil(config)
	require.Equal(kubernetes.K8sGateways, config.ObjectType)
	require.Equal(utils.BOOKINFO, config.Namespace.Name)
	require.NotNil(config.K8sGateway)
	require.Equal(name, config.K8sGateway.Name)
	require.Equal(utils.BOOKINFO, config.K8sGateway.Namespace)
	require.NotNil(config.IstioValidation)
	require.Equal(name, config.IstioValidation.Name)
	require.Equal("k8sgateway", config.IstioValidation.ObjectType)
	require.NotEmpty(config.IstioValidation.Checks)
	require.Equal(models.WarningSeverity, config.IstioValidation.Checks[0].Severity)
	require.Equal("More than one K8s Gateway for the same address and type combination", config.IstioValidation.Checks[0].Message)
}

func TestK8sGatewaysListenersError(t *testing.T) {
	name := "gatewayapi"
	require := require.New(t)
	filePath := path.Join(cmd.KialiProjectRoot, utils.ASSETS+"/bookinfo-k8sgateways-listeners.yaml")
	defer utils.DeleteFile(filePath, utils.BOOKINFO)
	require.True(utils.ApplyFile(filePath, utils.BOOKINFO))

	config, err := getConfigDetails(utils.BOOKINFO, name, kubernetes.K8sGateways, true, require)

	require.Nil(err)
	require.NotNil(config)
	require.Equal(kubernetes.K8sGateways, config.ObjectType)
	require.Equal(utils.BOOKINFO, config.Namespace.Name)
	require.NotNil(config.K8sGateway)
	require.Equal(name, config.K8sGateway.Name)
	require.Equal(utils.BOOKINFO, config.K8sGateway.Namespace)
	require.NotNil(config.IstioValidation)
	require.Equal(name, config.IstioValidation.Name)
	require.Equal("k8sgateway", config.IstioValidation.ObjectType)
	require.NotEmpty(config.IstioValidation.Checks)
	require.Equal(models.WarningSeverity, config.IstioValidation.Checks[0].Severity)
	require.Equal("More than one K8s Gateway for the same host port combination", config.IstioValidation.Checks[0].Message)
}

func TestK8sHTTPRoutesGatewaysError(t *testing.T) {
	name := "httproute"
	require := require.New(t)
	filePath := path.Join(cmd.KialiProjectRoot, utils.ASSETS+"/bookinfo-k8shttproutes-gateways.yaml")
	defer utils.DeleteFile(filePath, utils.BOOKINFO)
	require.True(utils.ApplyFile(filePath, utils.BOOKINFO))

	config, err := getConfigDetails(utils.BOOKINFO, name, kubernetes.K8sHTTPRoutes, true, require)

	require.Nil(err)
	require.NotNil(config)
	require.Equal(kubernetes.K8sHTTPRoutes, config.ObjectType)
	require.Equal(utils.BOOKINFO, config.Namespace.Name)
	require.NotNil(config.K8sHTTPRoute)
	require.Equal(name, config.K8sHTTPRoute.Name)
	require.Equal(utils.BOOKINFO, config.K8sHTTPRoute.Namespace)
	require.NotNil(config.IstioValidation)
	require.False(config.IstioValidation.Valid)
	require.Equal(name, config.IstioValidation.Name)
	require.Equal("k8shttproute", config.IstioValidation.ObjectType)
	require.NotEmpty(config.IstioValidation.Checks)
	require.Equal(models.ErrorSeverity, config.IstioValidation.Checks[0].Severity)
	require.Equal("HTTPRoute is pointing to a non-existent K8s gateway", config.IstioValidation.Checks[0].Message)
}

func TestK8sHTTPRoutesServicesError(t *testing.T) {
	name := "httprouteservices"
	require := require.New(t)
	filePath := path.Join(cmd.KialiProjectRoot, utils.ASSETS+"/bookinfo-k8shttproutes-services.yaml")
	defer utils.DeleteFile(filePath, utils.BOOKINFO)
	require.True(utils.ApplyFile(filePath, utils.BOOKINFO))

	config, err := getConfigDetails(utils.BOOKINFO, name, kubernetes.K8sHTTPRoutes, true, require)

	require.Nil(err)
	require.NotNil(config)
	require.Equal(kubernetes.K8sHTTPRoutes, config.ObjectType)
	require.Equal(utils.BOOKINFO, config.Namespace.Name)
	require.NotNil(config.K8sHTTPRoute)
	require.Equal(name, config.K8sHTTPRoute.Name)
	require.Equal(utils.BOOKINFO, config.K8sHTTPRoute.Namespace)
	require.NotNil(config.IstioValidation)
	require.False(config.IstioValidation.Valid)
	require.Equal(name, config.IstioValidation.Name)
	require.Equal("k8shttproute", config.IstioValidation.ObjectType)
	require.NotEmpty(config.IstioValidation.Checks)
	require.Equal(models.ErrorSeverity, config.IstioValidation.Checks[0].Severity)
	require.Equal("BackendRef on rule doesn't have a valid service (host not found)", config.IstioValidation.Checks[0].Message)
}

func getConfigDetails(namespace, name, configType string, skipReferences bool, require *require.Assertions) (*models.IstioConfigDetails, error) {
	config, _, err := utils.IstioConfigDetails(namespace, name, configType)
	if err == nil && config != nil && config.IstioValidation != nil && config.IstioReferences != nil {
		return config, nil
	}
	pollErr := wait.Poll(time.Second, time.Minute*5, func() (bool, error) {
		config, _, err = utils.IstioConfigDetails(namespace, name, configType)
		if err == nil && config != nil && config.IstioValidation != nil {
			if !skipReferences && config.IstioReferences != nil {
				return true, nil
			} else if skipReferences {
				return true, nil
			}
		}
		return false, nil
	})
	require.Nil(pollErr)
	return config, nil
}
