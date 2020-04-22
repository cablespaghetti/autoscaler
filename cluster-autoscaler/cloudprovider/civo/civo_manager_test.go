/*
Copyright 2019 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package civo

import (
	"bytes"
	"errors"
	"github.com/civo/civogo"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewManager(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		cfg := `{"cluster_id": "123456", "api_key": "123-123-123"}`
		nodeGroupSpecs := []string {"1:10:workers"}
		nodeGroupDiscoveryOptions := cloudprovider.NodeGroupDiscoveryOptions{NodeGroupSpecs: nodeGroupSpecs}
		manager, err := newManager(bytes.NewBufferString(cfg), nodeGroupDiscoveryOptions)
		assert.NoError(t, err)
		assert.Equal(t, "123456", manager.clusterID, "cluster ID does not match")
		assert.Equal(t, nodeGroupDiscoveryOptions, manager.discoveryOpts, "node group discovery options do not match")
	})

	t.Run("empty api_key", func(t *testing.T) {
		cfg := `{"cluster_id": "123456", "api_key": ""}`
		nodeGroupSpecs := []string {"1:10:workers"}
		nodeGroupDiscoveryOptions := cloudprovider.NodeGroupDiscoveryOptions{NodeGroupSpecs: nodeGroupSpecs}
		_, err := newManager(bytes.NewBufferString(cfg), nodeGroupDiscoveryOptions)
		assert.EqualError(t, err, errors.New("Civo API Key was not provided").Error())
	})

	t.Run("empty cluster ID", func(t *testing.T) {
		cfg := `{"cluster_id": "", "api_key": "123-123-123"}`
		nodeGroupSpecs := []string {"1:10:workers"}
		nodeGroupDiscoveryOptions := cloudprovider.NodeGroupDiscoveryOptions{NodeGroupSpecs: nodeGroupSpecs}
		_, err := newManager(bytes.NewBufferString(cfg), nodeGroupDiscoveryOptions)
		assert.EqualError(t, err, errors.New("cluster ID was not provided").Error())
	})
}

func TestCivoManager_Refresh(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		cfg := `{"cluster_id": "123456", "api_key": "123-123-123"}`
		nodeGroupSpecs := []string {"1:10:workers"}
		nodeGroupDiscoveryOptions := cloudprovider.NodeGroupDiscoveryOptions{NodeGroupSpecs: nodeGroupSpecs}
		manager, err := newManager(bytes.NewBufferString(cfg), nodeGroupDiscoveryOptions)
		assert.NoError(t, err)

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

		manager.client = client
		err = manager.Refresh()
		assert.NoError(t, err)
		assert.Equal(t, 1, len(manager.nodeGroups), "number of node groups do not match")
	})
}

func TestCivoManager_RefreshWithNodeSpec(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		cfg := `{"cluster_id": "123456", "api_key": "123-123-123"}`
		nodeGroupSpecs := []string {"1:10:workers"}
		nodeGroupDiscoveryOptions := cloudprovider.NodeGroupDiscoveryOptions{NodeGroupSpecs: nodeGroupSpecs}
		manager, err := newManager(bytes.NewBufferString(cfg), nodeGroupDiscoveryOptions)
		assert.NoError(t, err)

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

		manager.client = client
		err = manager.Refresh()
		assert.NoError(t, err)
		assert.Equal(t, 1, len(manager.nodeGroups), "number of node groups do not match")
		assert.Equal(t, 1, manager.nodeGroups[0].minSize,  "minimum node for node group does not match")
		assert.Equal(t, 10, manager.nodeGroups[0].maxSize, "maximum node for node group does not match")
	})
}
