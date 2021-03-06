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

package storage

import (
	"github.com/coreos/quartermaster/pkg/spec"
	"github.com/lpabon/godbc"
	"k8s.io/kubernetes/pkg/apis/extensions"
)

// StorageHandlerFuncs insulates the implementation from unsupported
// interface functions.  Please see StorageType for documentation
type StorageHandlerFuncs struct {
	StorageHandler interface{}

	AddClusterFunc    func(c *spec.StorageCluster) (*spec.StorageCluster, error)
	UpdateClusterFunc func(old *spec.StorageCluster, new *spec.StorageCluster) error
	DeleteClusterFunc func(c *spec.StorageCluster) error

	MakeDeploymentFunc func(n *spec.StorageNode, old *extensions.Deployment) (*extensions.Deployment, error)
	AddNodeFunc        func(n *spec.StorageNode) (*spec.StorageNode, error)
	UpdateNodeFunc     func(n *spec.StorageNode) (*spec.StorageNode, error)
	DeleteNodeFunc     func(n *spec.StorageNode) error

	InitFunc func() error
	TypeFunc func() spec.StorageTypeIdentifier
}

func (s StorageHandlerFuncs) AddCluster(c *spec.StorageCluster) (*spec.StorageCluster, error) {
	if s.AddClusterFunc != nil {
		return s.AddClusterFunc(c)
	}
	return nil, nil
}

func (s StorageHandlerFuncs) UpdateCluster(old *spec.StorageCluster,
	new *spec.StorageCluster) error {
	if s.UpdateClusterFunc != nil {
		return s.UpdateClusterFunc(old, new)
	}
	return nil
}

func (s StorageHandlerFuncs) DeleteCluster(c *spec.StorageCluster) error {
	if s.DeleteClusterFunc != nil {
		return s.DeleteClusterFunc(c)
	}
	return nil
}

func (s StorageHandlerFuncs) MakeDeployment(n *spec.StorageNode,
	old *extensions.Deployment) (*extensions.Deployment, error) {
	if s.MakeDeploymentFunc != nil {
		return s.MakeDeploymentFunc(n, old)
	}
	return nil, nil
}

func (s StorageHandlerFuncs) AddNode(n *spec.StorageNode) (*spec.StorageNode, error) {
	if s.AddNodeFunc != nil {
		return s.AddNodeFunc(n)
	}
	return nil, nil
}

func (s StorageHandlerFuncs) UpdateNode(n *spec.StorageNode) (*spec.StorageNode, error) {
	if s.UpdateNodeFunc != nil {
		return s.UpdateNodeFunc(n)
	}
	return nil, nil
}

func (s StorageHandlerFuncs) DeleteNode(n *spec.StorageNode) error {
	if s.DeleteNodeFunc != nil {
		return s.DeleteNodeFunc(n)
	}
	return nil
}

func (s StorageHandlerFuncs) Init() error {
	if s.InitFunc != nil {
		return s.InitFunc()
	}
	return nil
}

func (s StorageHandlerFuncs) Type() spec.StorageTypeIdentifier {
	godbc.Require(s.TypeFunc != nil, "Type() must be defined")
	return s.TypeFunc()
}
