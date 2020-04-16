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

	"github.com/civo/civogo"
	"k8s.io/klog"
)

var (
	version = "dev"
)

type nodeGroupClient interface {
	// ListNodePools lists all the node pools found in a Kubernetes cluster.
	GetKubernetesClusters(clusterID string) (*civogo.KubernetesCluster, error)

	// UpdateNodePool updates the details of an existing node pool.
	UpdateKubernetesCluster(clusterID, config civogo.KubernetesClusterConfig) (*civogo.KubernetesCluster, error)
}

// Manager handles Civo communication and data caching of
// node groups
type Manager struct {
	client     *civogo.Client
	clusterID  string
	minNodes int
	maxNodes int
	nodeGroups []*NodeGroup
}

// Config is the configuration of the Civo cloud provider
type Config struct {
	// ClusterID is the id associated with the cluster where Civo
	// Cluster Autoscaler is running.
	ClusterID string `json:"cluster_id"`

	// ApiKey is the Civo User's API Key associated with the cluster where
	// Civo Cluster Autoscaler is running.
	ApiKey string `json:"api_key"`

	// MinNodes is the minimum number of nodes for the cluster to have
	MinNodes int `json:"min_nodes"`

	// MinNodes is the minimum number of nodes for the cluster to have
	MaxNodes int `json:"max_nodes"`
}

func newManager(configReader io.Reader) (*Manager, error) {
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
	}

	if cfg.ApiKey == "" {
		return nil, errors.New("access token is not provided")
	}
	if cfg.ClusterID == "" {
		return nil, errors.New("cluster ID is not provided")
	}
	if cfg.MinNodes == 0 {
		return nil, errors.New("cluster minimum nodes is not provided")
	}
	if cfg.MaxNodes == 0 {
		return nil, errors.New("cluster maximum nodes is not provided")
	}

	civoClient, err := civogo.NewClient(cfg.ApiKey)

	if err != nil {
		return nil, fmt.Errorf("couldn't initialize Civo client: %s", err)
	}

	m := &Manager{
		client:    civoClient,
		clusterID: cfg.ClusterID,
		minNodes: cfg.MinNodes,
		maxNodes: cfg.MaxNodes,
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

	klog.V(4).Infof("adding node pool: %q name: %s min: %d max: %d", kubernetesCluster.ID, kubernetesCluster.Name, 1, 10)
	var group []*NodeGroup
	group = append(group, &NodeGroup{
			id:        kubernetesCluster.ID,
			clusterID: kubernetesCluster.ID,
			client:    m.client,
			kubernetesCluster:  kubernetesCluster,
			minSize:   m.minNodes,
			maxSize:   m.maxNodes,
		})

	if len(group) == 0 {
		klog.V(4).Info("cluster-autoscaler is disabled. no node pools are configured")
	}

	m.nodeGroups = group
	return nil
}
