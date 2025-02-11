// Copyright (c) Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package imageregistry

import (
	"encoding/json"
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fakekube "k8s.io/client-go/kubernetes/fake"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
)

func newCluster(name, imageRegistryAnnotation string) *clusterv1.ManagedCluster {
	cluster := &clusterv1.ManagedCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	if imageRegistryAnnotation != "" {
		cluster.SetAnnotations(map[string]string{ClusterImageRegistriesAnnotation: imageRegistryAnnotation})
	}
	return cluster
}

func newAnnotationRegistries(registries []Registry, namespacePullSecret string) string {
	registriesData := ImageRegistries{
		PullSecret: namespacePullSecret,
		Registries: registries,
	}

	registriesDataStr, _ := json.Marshal(registriesData)
	return string(registriesDataStr)
}

func newPullSecret(namespace, name string) *corev1.Secret {
	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			corev1.DockerConfigJsonKey: []byte("data"),
		},
		StringData: nil,
		Type:       corev1.SecretTypeDockerConfigJson,
	}
}

func fakeClient(secret *corev1.Secret) Interface {
	fakeKubeClient := fakekube.NewSimpleClientset(secret)

	return NewClient(fakeKubeClient)
}

func Test_ClientPullSecret(t *testing.T) {
	testCases := []struct {
		name               string
		client             Interface
		cluster            *clusterv1.ManagedCluster
		expectedErr        error
		expectedPullSecret *corev1.Secret
	}{
		{
			name:               "get correct pullSecret",
			client:             fakeClient(newPullSecret("ns1", "pullSecret")),
			cluster:            newCluster("cluster1", newAnnotationRegistries(nil, "ns1.pullSecret")),
			expectedErr:        nil,
			expectedPullSecret: newPullSecret("ns1", "pullSecret"),
		},
		{
			name:               "failed to get pullSecret pullSecret without annotation",
			client:             fakeClient(newPullSecret("ns1", "pullSecret")),
			cluster:            newCluster("cluster1", ""),
			expectedErr:        nil,
			expectedPullSecret: nil,
		},
		{
			name:               "failed to get pullSecret pullSecret with wrong annotation",
			client:             fakeClient(newPullSecret("ns1", "pullSecret")),
			cluster:            newCluster("cluster1", "abc"),
			expectedErr:        fmt.Errorf("invalid character 'a' looking for beginning of value"),
			expectedPullSecret: nil,
		},
		{
			name:               "failed to get pullSecret without pullSecret",
			client:             fakeClient(newPullSecret("ns1", "pullSecret")),
			cluster:            newCluster("cluster1", newAnnotationRegistries(nil, "ns.test")),
			expectedErr:        fmt.Errorf("secrets \"test\" not found"),
			expectedPullSecret: nil,
		},
	}

	for _, c := range testCases {
		t.Run(c.name, func(t *testing.T) {
			pullSecret, err := c.client.Cluster(c.cluster).PullSecret()
			if (err != nil && c.expectedErr == nil) || (err == nil && c.expectedErr != nil) {
				t.Errorf("expected error %v, but got %v", c.expectedErr, err)
			}

			if err != nil && c.expectedErr != nil && err.Error() != c.expectedErr.Error() {
				t.Errorf("expected err %v, but got %v", c.expectedErr, err)
			}
			if pullSecret != nil && c.expectedPullSecret != nil {
				if pullSecret.Name != c.expectedPullSecret.Name {
					t.Errorf("expected secret name %v, but got %v", c.expectedPullSecret.Name, pullSecret.Name)
				}
			}
		})
	}
}

func Test_ClientImageOverride(t *testing.T) {
	testCases := []struct {
		name          string
		image         string
		cluster       *clusterv1.ManagedCluster
		expectedImage string
		expectedErr   error
	}{
		{
			name: "override rhacm2 image ",
			cluster: newCluster("c1", newAnnotationRegistries([]Registry{
				{Source: "registry.redhat.io/rhacm2", Mirror: "quay.io/rhacm2"},
				{Source: "registry.redhat.io/multicluster-engine", Mirror: "quay.io/multicluster-engine"}}, "")),
			image:         "registry.redhat.io/rhacm2/registration@SHA256abc",
			expectedImage: "quay.io/rhacm2/registration@SHA256abc",
			expectedErr:   nil,
		},
		{
			name: "override acm-d image",
			cluster: newCluster("c1", newAnnotationRegistries([]Registry{
				{Source: "registry.redhat.io/rhacm2", Mirror: "quay.io/rhacm2"},
				{Source: "registry.redhat.io/multicluster-engine", Mirror: "quay.io/multicluster-engine"}}, "")),
			image:         "registry.redhat.io/acm-d/registration@SHA256abc",
			expectedImage: "registry.redhat.io/acm-d/registration@SHA256abc",
			expectedErr:   nil,
		},
		{
			name: "override multicluster-engine image",
			cluster: newCluster("c1", newAnnotationRegistries([]Registry{
				{Source: "registry.redhat.io/rhacm2", Mirror: "quay.io/rhacm2"},
				{Source: "registry.redhat.io/multicluster-engine", Mirror: "quay.io/multicluster-engine"}}, "")),
			image:         "registry.redhat.io/multicluster-engine/registration@SHA256abc",
			expectedImage: "quay.io/multicluster-engine/registration@SHA256abc",
			expectedErr:   nil,
		},
		{
			name: "override image without source ",
			cluster: newCluster("c1", newAnnotationRegistries([]Registry{
				{Source: "", Mirror: "quay.io/rhacm2"}}, "")),
			image:         "registry.redhat.io/multicluster-engine/registration@SHA256abc",
			expectedImage: "quay.io/rhacm2/registration@SHA256abc",
			expectedErr:   nil,
		},
		{
			name: "override image",
			cluster: newCluster("c1", newAnnotationRegistries([]Registry{
				{Source: "registry.redhat.io/rhacm2", Mirror: "quay.io/rhacm2"},
				{Source: "registry.redhat.io/rhacm2/registration@SHA256abc",
					Mirror: "quay.io/acm-d/registration:latest"}}, "")),
			image:         "registry.redhat.io/rhacm2/registration@SHA256abc",
			expectedImage: "quay.io/acm-d/registration:latest",
			expectedErr:   nil,
		},
		{
			name:          "return image without annotation",
			cluster:       newCluster("c1", ""),
			image:         "registry.redhat.io/rhacm2/registration@SHA256abc",
			expectedImage: "registry.redhat.io/rhacm2/registration@SHA256abc",
			expectedErr:   nil,
		},
		{
			name:          "return image with wrong annotation",
			cluster:       newCluster("c1", "abc"),
			image:         "registry.redhat.io/rhacm2/registration@SHA256abc",
			expectedImage: "registry.redhat.io/rhacm2/registration@SHA256abc",
			expectedErr:   fmt.Errorf("invalid character 'a' looking for beginning of value"),
		},
	}
	client := fakeClient(newPullSecret("n1", "s1"))
	for _, c := range testCases {
		t.Run(c.name, func(t *testing.T) {
			image, err := client.Cluster(c.cluster).ImageOverride(c.image)
			if image != c.expectedImage {
				t.Errorf("expected image %v but got %v", c.expectedImage, image)
			}

			if (err != nil && c.expectedErr == nil) || (err == nil && c.expectedErr != nil) {
				t.Errorf("expected error %v, but got %v", c.expectedErr, err)
			}

			if err != nil && c.expectedErr != nil && err.Error() != c.expectedErr.Error() {
				t.Errorf("expected error %v, but got %v", c.expectedErr, err)
			}

			image, err = OverrideImageByAnnotation(c.cluster.GetAnnotations(), c.image)
			if image != c.expectedImage {
				t.Errorf("expected image %v but got %v", c.expectedImage, image)
			}
			if (err != nil && c.expectedErr == nil) || (err == nil && c.expectedErr != nil) {
				t.Errorf("expected error %v, but got %v", c.expectedErr, err)
			}
			if err != nil && c.expectedErr != nil && err.Error() != c.expectedErr.Error() {
				t.Errorf("expected error %v, but got %v", c.expectedErr, err)
			}
		})
	}
}
