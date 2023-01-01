/*
Copyright 2021 The Kubernetes Authors.

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

package ipvsfullsate

import (
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/kpng/api/localv1"
	"sigs.k8s.io/kpng/client/serviceevents"
)

// Operation which can be done on ServicePortInfo and EndpointInfo
type Operation int32

// 4 types of Operation(s) can be done on Services and Endpoints
const (
	NoOp = iota
	Create
	Update
	Delete
)

// Handler contains the networking logic, calls proxier to implement the changes in network layer
type Handler interface {
	createService(*ServicePortInfo)
	createEndpoint(*EndpointInfo, *ServicePortInfo)

	updateService(*ServicePortInfo)
	updateEndpoint(*EndpointInfo, *ServicePortInfo)

	deleteService(*ServicePortInfo)
	deleteEndpoint(*EndpointInfo, *ServicePortInfo)

	getServiceHandlers() map[Operation]func(*ServicePortInfo)
	getEndpointHandlers() map[Operation]func(*EndpointInfo, *ServicePortInfo)
}

type ServiceType string

const (
	ClusterIPService    ServiceType = "ClusterIP"
	NodePortService     ServiceType = "NodePort"
	LoadBalancerService ServiceType = "LoadBalancer"
)

// String returns ServiceType as string
func (st ServiceType) String() string {
	return string(st)
}

// TODO - move these to BindFlags
const (
	IPVSSchedulingMethod = "rr"
	IPVSWeight           = 1
)

// ResourceInfo interface for ServicePortInfo and EndpointInfo
type ResourceInfo interface {
	ToBytes() []byte
}

// EndpointInfo contains base information of an endpoint in a structure that can be directly consumed by the proxier
type EndpointInfo struct {
	svcKey  string
	ip      string
	isNew   bool
	isLocal bool
	portMap map[string]int32
}

// ServicePortInfo contains base information of a service in a structure that can be directly consumed by the proxier
type ServicePortInfo struct {
	name                  string
	namespace             string
	isNew                 bool
	ip                    string
	lbIP                  string
	serviceType           ServiceType
	port                  int32
	targetPort            int32
	targetPortName        string
	nodePort              int32
	protocol              localv1.Protocol
	schedulingMethod      string
	weight                int32
	sessionAffinity       serviceevents.SessionAffinity
	stickyMaxAgeSeconds   int
	healthCheckNodePort   int
	nodeLocalExternal     bool
	nodeLocalInternal     bool
	internalTrafficPolicy *v1.ServiceInternalTrafficPolicyType
	hintsAnnotation       string
	ipFilterTargetIps     []string
	ipFilterSourceRanges  []string
}
