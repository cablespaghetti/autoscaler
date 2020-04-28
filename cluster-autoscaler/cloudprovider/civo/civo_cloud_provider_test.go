package civo

import (
	"bytes"
	"github.com/civo/civogo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	apiv1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func TestCivoCloudProvider_NodeGroups(t *testing.T) {
	provider := testCloudProvider(t, nil)

	t.Run("success", func(t *testing.T) {
		nodegroups := provider.NodeGroups()
		assert.Equal(t, len(nodegroups), 1, "number of node groups does not match")
		nodes, _ := nodegroups[0].Nodes()
		assert.Equal(t, len(nodes), 3, "number of nodes in workers node group does not match")

	})

	t.Run("zero groups", func(t *testing.T) {
		provider.manager.nodeGroups = []*NodeGroup{}
		nodes := provider.NodeGroups()
		assert.Equal(t, len(nodes), 0, "number of nodes do not match")
	})
}

func TestCivoCloudProvider_NodeGroupForNode(t *testing.T) {
	cfg := `{"cluster_id": "123456", "api_key": "123-123-123"}`
	nodeGroupSpecs := []string {"1:10:workers"}
	nodeGroupDiscoveryOptions := cloudprovider.NodeGroupDiscoveryOptions{NodeGroupSpecs: nodeGroupSpecs}
	manager, err := newManager(bytes.NewBufferString(cfg), nodeGroupDiscoveryOptions)
	assert.NoError(t, err)

	t.Run("success", func(t *testing.T) {
		client := &civoClientMock{}
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
						Status: "BUILD_PENDING",
					},
					{
						Hostname: "kube-node-3",
						Status: "BUILD",
					},
				},
			},
			nil,
		).Once()

		provider := testCloudProvider(t, client)

		// let's get the nodeGroup for the node with ID 4
		node := &apiv1.Node{
			ObjectMeta: v1.ObjectMeta{
				Name: "kube-node-3",
			},
		}

		nodeGroup, err := provider.NodeGroupForNode(node)
		require.NoError(t, err)
		require.NotNil(t, nodeGroup)
		require.Equal(t, nodeGroup.Id(), "workers", "node group ID does not match")
	})

	t.Run("node is master", func(t *testing.T) {
		client := &civoClientMock{}
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
						Status: "BUILD_PENDING",
						Tags: []string{"banana", "civo-kubernetes:node", "apple"},
					},
					{
						Hostname: "kube-node-3",
						Status: "BUILD",
						Tags: []string{"banana", "civo-kubernetes:master", "apple"},
					},
				},
			},
			nil,
		).Once()

		provider := testCloudProvider(t, client)

		node := &apiv1.Node{
			ObjectMeta: v1.ObjectMeta{Name: "kube-node-3", Labels: map[string]string{"node-role.kubernetes.io/master": "true"}},
		}

		nodeGroup, err := provider.NodeGroupForNode(node)
		assert.NoError(t, err)
		assert.Nil(t, nodeGroup)
	})
}
