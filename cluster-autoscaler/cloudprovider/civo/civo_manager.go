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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider"
	"k8s.io/autoscaler/cluster-autoscaler/config/dynamic"
	"os"

	"github.com/civo/civogo"
	"k8s.io/klog"
)

// Manager handles Civo communication and data caching of
// node groups
type Manager struct {
	client     *civogo.Client
	clusterID  string
	nodeGroups []*NodeGroup
}

// Config is the configuration of the Civo cloud provider
type Config struct {
	// ClusterID is the id associated with the cluster where Civo
	// Cluster Autoscaler is running.
	ClusterID string `json:"cluster_id" yaml:"cluster_id"`

	// ApiKey is the Civo User's API Key associated with the cluster where
	// Civo Cluster Autoscaler is running.
	ApiKey string `json:"api_key" yaml:"api_key"`
}

func newManager(configReader io.Reader, discoveryOpts cloudprovider.NodeGroupDiscoveryOptions) (*Manager, error) {
	cfg := &Config{}
	if configReader != nil {
		body, err := ioutil.ReadAll(configReader)
		if err != nil {
			return nil, err
		}
		err = json.Unmarshal(body, cfg)
		if err != nil {
			return nil, err
		}
	} else {
		cfg.ApiKey = os.Getenv("CIVO_API_KEY")
		cfg.ApiKey = os.Getenv("CIVO_CLUSTER_ID")
	}

	if cfg.ApiKey == "" {
		return nil, errors.New("access token is not provided")
	}
	if cfg.ClusterID == "" {
		return nil, errors.New("cluster ID is not provided")
	}

	civoClient, err := civogo.NewClient(cfg.ApiKey)

	if err != nil {
		return nil, fmt.Errorf("couldn't initialize Civo client: %s", err)
	}

	kubernetesCluster, err := civoClient.GetKubernetesClusters(cfg.ClusterID)
	if err != nil {
		return nil, fmt.Errorf("couldn't get Kubernetes cluster from Civo API: %s", err)
	}

	var group []*NodeGroup
	for _, specString := range discoveryOpts.NodeGroupSpecs {
		spec, err := dynamic.SpecFromString(specString, true)
		if err != nil {
			return nil, fmt.Errorf("failed to parse node group spec: %v", err)
		}
		if spec.Name == "workers" {
			minNodes := spec.MinSize
			maxNodes := spec.MaxSize
			klog.V(4).Infof("found configuration for workers node group: min: %d max: %d", minNodes, maxNodes)
			group = append(group, &NodeGroup{
				id:        "workers",
				clusterID: cfg.ClusterID,
				client:    civoClient,
				kubernetesCluster:  kubernetesCluster,
				minSize:   minNodes,
				maxSize:   maxNodes,
			})
		}
	}
	if len(group) < 1 {
		return nil, fmt.Errorf("no workers node group configuration found")
	}

	m := &Manager{
		client:    civoClient,
		clusterID: cfg.ClusterID,
		nodeGroups: group,
	}

	return m, nil
}

// Refresh refreshes the cache holding the nodegroups. This is called by the CA
// based on the `--scan-interval`. By default it's 10 seconds.
func (m *Manager) Refresh() error {
	kubernetesCluster, err := m.client.GetKubernetesClusters(m.clusterID)
	if err != nil {
		return err
	}
	klog.V(4).Infof("refreshing workers node group kubernetes cluster: %q name: %s min: %d max: %d", kubernetesCluster.ID, kubernetesCluster.Name, m.nodeGroups[0].MinSize(), m.nodeGroups[0].MaxSize())
	var group []*NodeGroup
	group = append(group, &NodeGroup{
			id:        "workers",
			clusterID: kubernetesCluster.ID,
			client:    m.client,
			kubernetesCluster:  kubernetesCluster,
			minSize:   m.nodeGroups[0].MinSize(),
			maxSize:   m.nodeGroups[0].MaxSize(),
		})

	m.nodeGroups = group
	return nil
}
