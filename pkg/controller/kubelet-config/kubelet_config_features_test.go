package kubeletconfig

import (
	"reflect"
	"testing"

	ign3types "github.com/coreos/ignition/v2/config/v3_2/types"
	configv1 "github.com/openshift/api/config/v1"
	osev1 "github.com/openshift/api/config/v1"
	"github.com/vincent-petithory/dataurl"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeletconfigv1beta1 "k8s.io/kubelet/config/v1beta1"

	ctrlcommon "github.com/openshift/machine-config-operator/pkg/controller/common"
	"github.com/openshift/machine-config-operator/test/helpers"
)

func TestFeatureGateDrift(t *testing.T) {
	for _, platform := range []configv1.PlatformType{configv1.AWSPlatformType, configv1.NonePlatformType, "unrecognized"} {
		t.Run(string(platform), func(t *testing.T) {
			f := newFixture(t)
			cc := newControllerConfig(ctrlcommon.ControllerConfigName, platform)
			f.ccLister = append(f.ccLister, cc)

			ctrl := f.newController()
			kubeletConfig, err := ctrl.generateOriginalKubeletConfig("master", nil)
			if err != nil {
				t.Errorf("could not generate kubelet config from templates %v", err)
			}
			dataURL, _ := dataurl.DecodeString(*kubeletConfig.Contents.Source)
			originalKubeConfig, _ := decodeKubeletConfig(dataURL.Data)
			defaultFeatureGates, err := generateFeatureMap(createNewDefaultFeatureGate())
			if err != nil {
				t.Errorf("could not generate defaultFeatureGates: %v", err)
			}
			if !reflect.DeepEqual(originalKubeConfig.FeatureGates, *defaultFeatureGates) {
				t.Errorf("template FeatureGates do not match openshift/api FeatureGates: (tmpl=[%v], api=[%v]", originalKubeConfig.FeatureGates, defaultFeatureGates)
			}
		})
	}
}

func TestFeaturesDefault(t *testing.T) {
	for _, platform := range []configv1.PlatformType{configv1.AWSPlatformType, configv1.NonePlatformType, "unrecognized"} {
		t.Run(string(platform), func(t *testing.T) {
			f := newFixture(t)
			f.newController()

			cc := newControllerConfig(ctrlcommon.ControllerConfigName, platform)
			mcp := helpers.NewMachineConfigPool("master", nil, helpers.MasterSelector, "v0")
			mcp2 := helpers.NewMachineConfigPool("worker", nil, helpers.WorkerSelector, "v0")
			kc1 := newKubeletConfig("smaller-max-pods", &kubeletconfigv1beta1.KubeletConfiguration{MaxPods: 100}, metav1.AddLabelToSelector(&metav1.LabelSelector{}, "pools.operator.machineconfiguration.openshift.io/master", ""))
			kc2 := newKubeletConfig("bigger-max-pods", &kubeletconfigv1beta1.KubeletConfiguration{MaxPods: 250}, metav1.AddLabelToSelector(&metav1.LabelSelector{}, "pools.operator.machineconfiguration.openshift.io/master", ""))
			kubeletConfigKey1, _ := getManagedKubeletConfigKey(mcp, f.client, kc1)
			kubeletConfigKey2, _ := getManagedKubeletConfigKey(mcp2, f.client, kc2)
			mcs := helpers.NewMachineConfig(kubeletConfigKey1, map[string]string{"node-role/master": ""}, "dummy://", []ign3types.File{{}})
			mcs2 := helpers.NewMachineConfig(kubeletConfigKey2, map[string]string{"node-role/worker": ""}, "dummy://", []ign3types.File{{}})
			mcsDeprecated := mcs.DeepCopy()
			mcsDeprecated.Name = getManagedFeaturesKeyDeprecated(mcp)
			mcs2Deprecated := mcs2.DeepCopy()
			mcs2Deprecated.Name = getManagedFeaturesKeyDeprecated(mcp2)

			f.ccLister = append(f.ccLister, cc)
			f.mcpLister = append(f.mcpLister, mcp)
			f.mcpLister = append(f.mcpLister, mcp2)

			features := createNewDefaultFeatureGate()
			f.featLister = append(f.featLister, features)

			f.expectGetMachineConfigAction(mcs)
			f.expectGetMachineConfigAction(mcsDeprecated)
			f.expectGetMachineConfigAction(mcs)
			f.expectGetMachineConfigAction(mcs2)
			f.expectGetMachineConfigAction(mcs2Deprecated)
			f.expectGetMachineConfigAction(mcs2)

			f.runFeature(getKeyFromFeatureGate(features, t))
		})
	}
}

func TestFeaturesCustomNoUpgrade(t *testing.T) {
	for _, platform := range []configv1.PlatformType{configv1.AWSPlatformType, configv1.NonePlatformType, "unrecognized"} {
		t.Run(string(platform), func(t *testing.T) {
			f := newFixture(t)
			f.newController()

			cc := newControllerConfig(ctrlcommon.ControllerConfigName, platform)
			mcp := helpers.NewMachineConfigPool("master", nil, helpers.MasterSelector, "v0")
			mcp2 := helpers.NewMachineConfigPool("worker", nil, helpers.WorkerSelector, "v0")
			kc1 := newKubeletConfig("smaller-max-pods", &kubeletconfigv1beta1.KubeletConfiguration{MaxPods: 100}, metav1.AddLabelToSelector(&metav1.LabelSelector{}, "pools.operator.machineconfiguration.openshift.io/master", ""))
			kc2 := newKubeletConfig("bigger-max-pods", &kubeletconfigv1beta1.KubeletConfiguration{MaxPods: 250}, metav1.AddLabelToSelector(&metav1.LabelSelector{}, "pools.operator.machineconfiguration.openshift.io/master", ""))
			kubeletConfigKey1, _ := getManagedKubeletConfigKey(mcp, f.client, kc1)
			kubeletConfigKey2, _ := getManagedKubeletConfigKey(mcp2, f.client, kc2)
			mcs := helpers.NewMachineConfig(kubeletConfigKey1, map[string]string{"node-role/master": ""}, "dummy://", []ign3types.File{{}})
			mcs2 := helpers.NewMachineConfig(kubeletConfigKey2, map[string]string{"node-role/worker": ""}, "dummy://", []ign3types.File{{}})
			mcsDeprecated := mcs.DeepCopy()
			mcsDeprecated.Name = getManagedFeaturesKeyDeprecated(mcp)
			mcs2Deprecated := mcs2.DeepCopy()
			mcs2Deprecated.Name = getManagedFeaturesKeyDeprecated(mcp2)

			f.ccLister = append(f.ccLister, cc)
			f.mcpLister = append(f.mcpLister, mcp)
			f.mcpLister = append(f.mcpLister, mcp2)

			features := &osev1.FeatureGate{
				ObjectMeta: metav1.ObjectMeta{
					Name: ctrlcommon.ClusterFeatureInstanceName,
				},
				Spec: osev1.FeatureGateSpec{
					FeatureGateSelection: osev1.FeatureGateSelection{
						FeatureSet: osev1.CustomNoUpgrade,
						CustomNoUpgrade: &osev1.CustomFeatureGates{
							Enabled: []string{"CSIMigration"},
						},
					},
				},
			}

			f.featLister = append(f.featLister, features)

			f.expectGetMachineConfigAction(mcs)
			f.expectGetMachineConfigAction(mcsDeprecated)
			f.expectGetMachineConfigAction(mcs)
			f.expectCreateMachineConfigAction(mcs)
			f.expectGetMachineConfigAction(mcs2)
			f.expectGetMachineConfigAction(mcs2Deprecated)
			f.expectGetMachineConfigAction(mcs2)
			f.expectCreateMachineConfigAction(mcs2)
			f.runFeature(getKeyFromFeatureGate(features, t))
		})
	}
}
