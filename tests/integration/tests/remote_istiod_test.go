package tests

import (
	"context"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	kubeyaml "k8s.io/apimachinery/pkg/util/yaml"

	"github.com/kiali/kiali/config"
	kialiKube "github.com/kiali/kiali/kubernetes"
	"github.com/kiali/kiali/log"
	"github.com/kiali/kiali/tests/integration/utils"
	"github.com/kiali/kiali/tests/integration/utils/kiali"
	"github.com/kiali/kiali/tests/integration/utils/kube"
	"github.com/kiali/kiali/tools/cmd"
)

var assetsFolder = path.Join(cmd.KialiProjectRoot, kiali.ASSETS)

// Testing a remote istiod by exposing /debug endpoints through a proxy.
// It assumes you've deployed Kiali through the operator.
func TestRemoteIstiod(t *testing.T) {
	require := require.New(t)

	proxyPatchPath := path.Join(assetsFolder + "/remote-istiod/proxy-patch.yaml")
	proxyPatch, err := kubeyaml.ToJSON(kialiKube.ReadFile(t, proxyPatchPath))
	require.NoError(err)

	kubeClient := kube.NewKubeClient(t)
	dynamicClient := kube.NewDynamicClient(t)
	kialiGVR := schema.GroupVersionResource{Group: "kiali.io", Version: "v1alpha1", Resource: "kialis"}

	deadline, _ := t.Deadline()
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	// This is used by cleanup so needs to be added to cleanup instead of deferred.
	t.Cleanup(cancel)

	var kialiCRDExists bool
	_, err = kubeClient.Discovery().RESTClient().Get().AbsPath("/apis/kiali.io").DoRaw(ctx)
	if !kubeerrors.IsNotFound(err) {
		require.NoError(err)
		kialiCRDExists = true
	}

	if kialiCRDExists {
		log.Debug("Kiali CRD found. Assuming Kiali is deployed through the operator.")
	} else {
		log.Debug("Kiali CRD not found. Assuming Kiali is deployed through helm.")
	}

	kialiName := "kiali"
	kialiNamespace := "istio-system"
	kialiDeploymentNamespace := kialiNamespace
	ipods, err := kubeClient.AppsV1().Deployments(kialiNamespace).List(ctx, metav1.ListOptions{LabelSelector: "app=istiod"})
	istioDeploymentName := ipods.Items[0].Name

	if kialiCRDExists {
		// Find the Kiali CR and override some settings if they're set on the CR.
		kialiCRs, err := dynamicClient.Resource(kialiGVR).List(ctx, metav1.ListOptions{})
		require.NoError(err)

		kialiCR := kialiCRs.Items[0]

		kialiName = kialiCR.GetName()
		kialiNamespace = kialiCR.GetNamespace()
		if spec, ok := kialiCR.Object["spec"].(map[string]interface{}); ok {
			if deployment, ok := spec["deployment"].(map[string]interface{}); ok {
				if namespace, ok := deployment["namespace"].(string); ok {
					kialiDeploymentNamespace = namespace
				}
			}
		}
	}

	// Register clean up before creating resources in case of failure.
	t.Cleanup(func() {
		log.Debug("Cleaning up resources from RemoteIstiod test")

		currentKialiPod := kube.GetKialiPodName(ctx, kubeClient, kialiDeploymentNamespace, t)

		if kialiCRDExists {
			undoRegistryPatch := []byte(`[{"op": "remove", "path": "/spec/external_services/istio/registry"}]`)
			_, err2 := dynamicClient.Resource(kialiGVR).Namespace(kialiNamespace).Patch(ctx, kialiName, types.JSONPatchType, undoRegistryPatch, metav1.PatchOptions{})
			require.NoError(err2)
		} else {
			// Update the configmap directly by getting the configmap and patching it.
			cm, err := kubeClient.CoreV1().ConfigMaps(kialiDeploymentNamespace).Get(ctx, kialiName, metav1.GetOptions{})
			require.NoError(err)

			var currentConfig config.Config
			require.NoError(yaml.Unmarshal([]byte(cm.Data["config.yaml"]), &currentConfig))
			currentConfig.ExternalServices.Istio.Registry = nil

			newConfig, err := yaml.Marshal(currentConfig)
			require.NoError(err)
			cm.Data["config.yaml"] = string(newConfig)

			log.Debugf("Kiali namespace: %s ", kialiNamespace)
			log.Debugf("Kiali deployment namespace: %s ", kialiDeploymentNamespace)

			_, err = kubeClient.CoreV1().ConfigMaps(kialiDeploymentNamespace).Update(ctx, cm, metav1.UpdateOptions{})
			require.NoError(err)

			require.NoError(kube.DeleteKialiPod(ctx, kubeClient, kialiDeploymentNamespace, currentKialiPod))
			// Restart Kiali pod to pick up the new config.
			require.NoError(kube.RestartKialiPod(ctx, kubeClient, kialiDeploymentNamespace, currentKialiPod))
		}

		// Remove service:
		err = kubeClient.CoreV1().Services("istio-system").Delete(ctx, "istiod-debug", metav1.DeleteOptions{})
		if kubeerrors.IsNotFound(err) {
			err = nil
		}
		require.NoError(err)

		log.Debugf("Remove nginx container from istio deployment %s", istioDeploymentName)
		// Remove nginx container
		istiod, err := kubeClient.AppsV1().Deployments("istio-system").Get(ctx, istioDeploymentName, metav1.GetOptions{})
		require.NoError(err)

		for i, container := range istiod.Spec.Template.Spec.Containers {
			if container.Name == "nginx" {
				istiod.Spec.Template.Spec.Containers = append(istiod.Spec.Template.Spec.Containers[:i], istiod.Spec.Template.Spec.Containers[i+1:]...)
				break
			}
		}
		_, err = kubeClient.AppsV1().Deployments("istio-system").Update(ctx, istiod, metav1.UpdateOptions{})
		require.NoError(err)

		currentKialiPod = kube.GetKialiPodName(ctx, kubeClient, kialiDeploymentNamespace, t)

		// Wait for the configmap to be updated again before exiting.
		require.NoError(wait.PollImmediate(time.Second*5, time.Minute*2, func() (bool, error) {
			cm, err := kubeClient.CoreV1().ConfigMaps(kialiDeploymentNamespace).Get(ctx, kialiName, metav1.GetOptions{})
			if err != nil {
				return false, err
			}
			return !strings.Contains(cm.Data["config.yaml"], "http://istiod-debug.istio-system:9240"), nil
		}), "Error waiting for kiali configmap to update")

		if !kialiCRDExists {
			require.NoError(kube.DeleteKialiPod(ctx, kubeClient, kialiDeploymentNamespace, currentKialiPod))
		}
		require.NoError(kube.RestartKialiPod(ctx, kubeClient, kialiDeploymentNamespace, currentKialiPod))
	})

	// Expose the istiod /debug endpoints by adding a proxy to the pod.
	log.Debugf("Patching istiod %s deployment with proxy", istioDeploymentName)
	_, err = kubeClient.AppsV1().Deployments("istio-system").Patch(ctx, istioDeploymentName, types.StrategicMergePatchType, proxyPatch, metav1.PatchOptions{})
	require.NoError(err)
	log.Debug("Successfully patched istiod deployment with proxy")

	// Then create a service for the proxy/debug endpoint.
	require.True(utils.ApplyFile(assetsFolder+"/remote-istiod/istiod-debug-service.yaml", "istio-system"), "Could not create istiod debug service")

	// Now patch kiali to use that remote endpoint.
	log.Debug("Patching kiali to use remote istiod")
	var currentKialiPod string

	pods, err := kubeClient.CoreV1().Pods(kialiDeploymentNamespace).List(ctx, metav1.ListOptions{LabelSelector: "app=kiali"})
	if err != nil {
		log.Error("Error getting the pods list %s", err)
	} else {
		currentKialiPod = pods.Items[0].Name
	}

	if kialiCRDExists {
		registryPatch := []byte(`{"spec": {"external_services": {"istio": {"registry": {"istiod_url": "http://istiod-debug.istio-system:9240"}}}}}`)
		kube.UpdateKialiCR(ctx, dynamicClient, kubeClient, kialiDeploymentNamespace, "http://istiod-debug.istio-system:9240", registryPatch, t)
	} else {
		// Update the configmap directly.
		currentConfig, cm := kube.GetKialiConfigMap(ctx, kubeClient, kialiDeploymentNamespace, kialiName, t)

		currentConfig.ExternalServices.Istio.Registry = &config.RegistryConfig{
			IstiodURL: "http://istiod-debug.istio-system:9240",
		}

		kube.UpdateKialiConfigMap(ctx, kubeClient, kialiNamespace, currentConfig, cm, t)
	}
	log.Debugf("Successfully patched kiali to use remote istiod")

	// Restart Kiali pod to pick up the new config.
	if !kialiCRDExists {
		require.NoError(kube.DeleteKialiPod(ctx, kubeClient, kialiDeploymentNamespace, currentKialiPod))
	}
	require.NoError(kube.RestartKialiPod(ctx, kubeClient, kialiDeploymentNamespace, currentKialiPod), "Error waiting for kiali deployment to update")

	configs, err := kiali.IstioConfigs()
	require.NoError(err)
	require.NotEmpty(configs)
}
