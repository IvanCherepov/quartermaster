// Copyright 2017 The quartermaster Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package glusterfs

import (
	"sort"
	"strings"

	qmclient "github.com/coreos-inc/quartermaster/pkg/client"
	"github.com/coreos-inc/quartermaster/pkg/operator"
	"github.com/coreos-inc/quartermaster/pkg/spec"
	qmstorage "github.com/coreos-inc/quartermaster/pkg/storage"
	"github.com/coreos-inc/quartermaster/pkg/utils"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/extensions"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/client/restclient"

	heketiclient "github.com/heketi/heketi/client/api/go-client"
	heketiapi "github.com/heketi/heketi/pkg/glusterfs/api"
)

var (
	logger          = utils.NewLogger("glusterfs", utils.LEVEL_DEBUG)
	heketiAddressFn = func(namespace string) (string, error) {
		return "http://localhost:8080", nil
	}
	waitForDeploymentFn = func(client clientset.Interface, namespace, name string, available int32) error {
		return operator.WaitForDeploymentReady(client, namespace, name, available)
	}
)

func New(client clientset.Interface, qm restclient.Interface) (qmstorage.StorageType, error) {
	s := &GlusterStorage{
		client: client,
		qm:     qm,
	}

	return &qmstorage.StorageHandlerFuncs{
		StorageHandler:     s,
		TypeFunc:           s.Type,
		InitFunc:           s.Init,
		AddClusterFunc:     s.AddCluster,
		UpdateClusterFunc:  s.UpdateCluster,
		DeleteClusterFunc:  s.DeleteCluster,
		MakeDeploymentFunc: s.MakeDeployment,
		AddNodeFunc:        s.AddNode,
		UpdateNodeFunc:     s.UpdateNode,
		DeleteNodeFunc:     s.DeleteNode,
		GetStatusFunc:      s.GetStatus,
	}, nil
}

type GlusterStorage struct {
	client clientset.Interface
	qm     restclient.Interface
}

func (st *GlusterStorage) Init() error {
	logger.Debug("msg init")
	return nil
}

func (st *GlusterStorage) AddCluster(c *spec.StorageCluster) (*spec.StorageCluster, error) {
	logger.Debug("msg add cluster cluster %v", c.Name)

	// Make sure Heketi is up and running
	err := st.deployHeketi(c.Namespace)
	if err != nil {
		return nil, err
	}

	// Get a client
	if c.Spec.GlusterFS == nil || len(c.Spec.GlusterFS.Cluster) == 0 {
		httpAddress, err := heketiAddressFn(c.GetNamespace())
		if err != nil {
			return nil, err
		}

		// Create a new cluster
		// TODO(lpabon): Need to set user and secret
		h := heketiclient.NewClientNoAuth(httpAddress)
		hcluster, err := h.ClusterCreate()
		if err != nil {
			return nil, logger.LogError("err: unable to create cluster in Heketi: %v", err)
		} else {
			logger.Debug("Created cluster %v cluster id %v", c.GetName(), hcluster.Id)
		}

		// Save cluster id in the spec
		c.Spec.GlusterFS = &spec.GlusterStorageCluster{
			Cluster: hcluster.Id,
		}

		return c, nil
	}

	// No changes needed
	logger.Info("cluster already registered: cluster[%v] id[%v]",
		c.GetName(),
		c.Spec.GlusterFS.Cluster)
	return nil, nil
}

func (st *GlusterStorage) UpdateCluster(old *spec.StorageCluster,
	new *spec.StorageCluster) error {
	logger.Debug("update cluster %v", old.Name)
	return nil
}

func (st *GlusterStorage) DeleteCluster(c *spec.StorageCluster) error {
	logger.Debug("delete cluster %v", c.Name)
	return nil
}

func (st *GlusterStorage) MakeDeployment(c *spec.StorageCluster,
	s *spec.StorageNode,
	old *extensions.Deployment) (*extensions.Deployment, error) {

	// TODO(lpabon): Make this required
	if s.Spec.Image == "" {
		s.Spec.Image = "gluster/gluster-centos:latest"
	}

	spec, err := st.makeGlusterFSDeploymentSpec(s)
	if err != nil {
		return nil, err
	}
	lmap := make(map[string]string)
	for k, v := range s.Labels {
		lmap[k] = v
	}
	lmap["quartermaster"] = s.Name
	deployment := &extensions.Deployment{
		ObjectMeta: api.ObjectMeta{
			Name:        s.Name,
			Namespace:   s.Namespace,
			Annotations: s.Annotations,
			Labels:      lmap,
		},
		Spec: *spec,
	}
	if old != nil {
		deployment.Annotations = old.Annotations
	}
	return deployment, nil
}

func (st *GlusterStorage) AddNode(c *spec.StorageCluster, s *spec.StorageNode) (*spec.StorageNode, error) {

	// Update cluster
	clusters := qmclient.NewStorageClusters(st.qm, s.GetNamespace())
	cluster, err := clusters.Get(s.Spec.ClusterRef.Name)
	if err != nil {
		return nil, logger.Err(err)
	}

	// Check GlusterFS information was added
	err = IsGlusterFSStorageClusterUsable(cluster)
	if err != nil {
		return nil, err
	}
	err = IsGlusterFSStorageNodeUsable(s)
	if err != nil {
		return nil, err
	}

	// Get a client to Heketi
	h, err := st.heketiClient(s.GetNamespace())
	if err != nil {
		return nil, err
	}

	// Get storage ip
	if s.Spec.StorageNetwork == nil {
		return nil, logger.Warning("StorageNetwork has not been defined for %v/%v",
			s.GetNamespace(), s.GetName())
	}
	storageip := s.Spec.StorageNetwork.IPs[0]

	// Check if the node has already been added
	if len(s.Spec.GlusterFS.Node) != 0 {
		// Check node ID
		_, err := h.NodeInfo(s.Spec.GlusterFS.Node)
		if err != nil {
			return nil, logger.Critical("Storage node %v/%v has a glusterfs id of %v but "+
				"the GlusterFS cluster does not recognize it.",
				s.GetNamespace(), s.GetName(), s.Spec.GlusterFS.Node)
		}
	} else {
		// Add node to Heketi
		nodereq := &heketiapi.NodeAddRequest{
			Zone:      s.Spec.GlusterFS.Zone,
			ClusterId: cluster.Spec.GlusterFS.Cluster,
			Hostnames: heketiapi.HostAddresses{
				Manage:  sort.StringSlice{s.Spec.NodeName},
				Storage: sort.StringSlice{storageip},
			},
		}

		// Add node to Heketi
		node, err := h.NodeAdd(nodereq)
		if err != nil {
			return nil, logger.LogError("unable to add node %v: %v", s.GetName(), err)
		}
		logger.Info("Added node %v/%v with id %v", s.GetNamespace(), s.GetName(), node.Id)

		// Update node with new information
		s.Spec.GlusterFS.Cluster = cluster.Spec.GlusterFS.Cluster
		s.Spec.GlusterFS.Node = node.Id
		s.Status.Ready = true
	}

	// Check if there are any devices to add
	if len(s.Spec.Devices) == 0 {
		logger.Warning("No devices defined for node %v/%v", s.GetNamespace(), s.GetName())
		return s, nil
	}

	// Get full node information
	nodeInfo, err := h.NodeInfo(s.Spec.GlusterFS.Node)
	if err != nil {
		return nil, logger.LogError("Unable to get node %v/%v information from Heketi using id %v",
			s.GetNamespace(), s.GetName(), s.Spec.GlusterFS.Node)
	}

	// Add devices
	for _, device := range s.Spec.Devices {
		// Check device to see if it is setup alreaedy
		_, err := st.heketiIdForDevice(device, nodeInfo)
		if err != nil {
			// Add device
			err := h.DeviceAdd(&heketiapi.DeviceAddRequest{
				Device: heketiapi.Device{
					Name: device,
				},
				NodeId: s.Spec.GlusterFS.Node,
			})
			if err != nil {
				logger.LogError("Unable to add device %v/%v %v: %v",
					s.GetNamespace(), s.GetName(), device, err)
			} else {
				logger.Info("Registered %v/%v %v", s.GetNamespace(), s.GetName(), device)
			}

		} else {
			logger.Debug("Already registered %v/%v %v", s.GetNamespace(), s.GetName(), device)
		}
	}

	return s, nil
}

func (st *GlusterStorage) UpdateNode(c *spec.StorageCluster, s *spec.StorageNode) (*spec.StorageNode, error) {
	logger.Debug("update node storagenode %v", s.Name)
	return nil, nil
}

func (st *GlusterStorage) DeleteNode(s *spec.StorageNode) error {
	logger.Debug("delete node storagenode %v", s.Name)

	if len(s.Spec.GlusterFS.Node) == 0 {
		return nil
	}

	// Get a client to Heketi
	h, err := st.heketiClient(s.GetNamespace())
	if err != nil {
		return logger.Err(err)
	}

	// Get device information
	node, err := h.NodeInfo(s.Spec.GlusterFS.Node)
	if err != nil {
		return logger.Err(err)
	}

	// Delete all devices
	for _, d := range node.DevicesInfo {
		err := h.DeviceDelete(d.Id)
		if err != nil {
			logger.Err(err)
		}
	}

	// Delete node
	return h.NodeDelete(s.Spec.GlusterFS.Node)
}

func (st *GlusterStorage) GetStatus(c *spec.StorageCluster) (*spec.StorageStatus, error) {
	logger.Debug("status")
	status := &spec.StorageStatus{}
	return status, nil
}

func (st *GlusterStorage) Type() spec.StorageTypeIdentifier {
	return spec.StorageTypeIdentifierGlusterFS
}

func dashifyPath(s string) string {
	s = strings.TrimLeft(s, "/")
	return strings.Replace(s, "/", "-", -1)
}
