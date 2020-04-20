package civo

import (
	"bytes"
	"github.com/civo/civogo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider"
	"testing"
)

type civoClientMock struct {
	mock.Mock
}

func (m *civoClientMock) GetKubernetesClusters(id string) (*civogo.KubernetesCluster, error) {
	args := m.Called(id)
	return args.Get(0).(*civogo.KubernetesCluster), args.Error(1)
}

func (m *civoClientMock) UpdateKubernetesCluster(id string, i *civogo.KubernetesClusterConfig) (*civogo.KubernetesCluster, error) {
	args := m.Called(id, i)
	return args.Get(0).(*civogo.KubernetesCluster), args.Error(1)
}

func testCloudProvider(t *testing.T, client *civoClientMock) *civoCloudProvider {
	cfg := `{"cluster_id": "123456", "api_key": "123-123-123"}`
	nodeGroupSpecs := []string {"1:10:workers"}
	nodeGroupDiscoveryOptions := cloudprovider.NodeGroupDiscoveryOptions{NodeGroupSpecs: nodeGroupSpecs}
	manager, err := newManager(bytes.NewBufferString(cfg), nodeGroupDiscoveryOptions)
	assert.NoError(t, err)
	rl := &cloudprovider.ResourceLimiter{}

	// fill the test provider with some example
	if client == nil {
		client = &civoClientMock{}
		client.On("GetKubernetesClusters", manager.clusterID).Return(
			&civogo.KubernetesCluster{
				ID: manager.clusterID,
				Name: "dave",
				NumTargetNode: 3,
				Instances: []civogo.KubernetesInstance{
					{
						Hostname: "kube-node-1",
						Status: "ACTIVE",
					},
					{
						Hostname: "kube-node-2",
						Status: "ACTIVE",
					},
					{
						Hostname: "kube-node-3",
						Status: "ACTIVE",
					},
				},
			},
			nil,
		).Once()
	}

	manager.client = client

	provider, err := newCivoCloudProvider(manager, rl)
	assert.NoError(t, err)
	return provider
}

func TestNewCivoCloudProvider(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		_ = testCloudProvider(t, nil)
	})
}

func TestCivoCloudProvider_Name(t *testing.T) {
	provider := testCloudProvider(t, nil)

	t.Run("success", func(t *testing.T) {
		name := provider.Name()
		assert.Equal(t, cloudprovider.CivoProviderName, name, "provider name doesn't match")
	})
}
