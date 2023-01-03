/*
Copyright 2023 The Kubernetes Authors.

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
	"strconv"
)

func NewServicePortInfo(service *localv1.Service, port *localv1.PortMapping, schedulingMethod string, weight int32) *ServicePortInfo {

	serviceType := ServiceType(service.GetType())
	ipFilterTargetIps, ipFilterSourceRanges := getIPFilterTargetIpsAndSourceRanges(v1.IPv4Protocol, service)

	return &ServicePortInfo{
		name:                 service.Name,
		namespace:            service.Namespace,
		clusterIPs:           getClusterIPsByFamily(v1.IPv4Protocol, service),
		loadbalancerIPs:      getLoadBalancerIPsByFamily(v1.IPv4Protocol, service),
		externalIPs:          getExternalIPsByFamily(v1.IPv4Protocol, service),
		port:                 uint16(port.Port),
		targetPort:           uint16(port.TargetPort),
		targetPortName:       port.Name,
		nodePort:             uint16(port.NodePort),
		protocol:             port.Protocol,
		schedulingMethod:     schedulingMethod,
		weight:               weight,
		serviceType:          serviceType,
		sessionAffinity:      getSessionAffinity(service.SessionAffinity),
		nodeLocalExternal:    service.GetExternalTrafficToLocal(),
		nodeLocalInternal:    service.GetInternalTrafficToLocal(),
		ipFilterTargetIps:    ipFilterTargetIps,
		ipFilterSourceRanges: ipFilterSourceRanges,
	}
}

// NamespacedName returns namespace + name
func (b *ServicePortInfo) NamespacedName() string {
	return b.namespace + "/" + b.name
}

// GetClusterIPs returns service ip
func (b *ServicePortInfo) GetClusterIPs() []string {
	return b.clusterIPs
}

// GetLoadBalancerIPs returns service LB ip
func (b *ServicePortInfo) GetLoadBalancerIP() []string {
	return b.loadbalancerIPs
}

// GetExternalIPs returns service LB ip
func (b *ServicePortInfo) GetExternalIPs() []string {
	return b.externalIPs
}

// Port return service port
func (b *ServicePortInfo) Port() uint16 {
	return b.port
}

// NodePort return service node port
func (b *ServicePortInfo) NodePort() uint16 {
	return b.nodePort
}

// TargetPort return service target port
func (b *ServicePortInfo) TargetPort() uint16 {
	return b.targetPort
}

// TargetPortName return name of the target port
func (b *ServicePortInfo) TargetPortName() string {
	return b.targetPortName
}

// Protocol return service protocol
func (b *ServicePortInfo) Protocol() localv1.Protocol {
	return b.protocol
}

// GetVirtualServer return IPVS LB object
func (b *ServicePortInfo) GetVirtualServer(IP string) ipvsLB {
	vs := ipvsLB{
		IP:               IP,
		SchedulingMethod: b.schedulingMethod,
		ServiceType:      b.serviceType,
		Port:             b.port,
		Protocol:         b.protocol,
		NodePort:         b.nodePort,
	}

	if b.sessionAffinity.ClientIP != nil {
		vs.Flags |= FlagPersistent
		vs.Timeout = uint32(b.sessionAffinity.ClientIP.ClientIP.TimeoutSeconds)
	}
	return vs
}

// ToBytes returns ServicePortInfo as []byte
func (b *ServicePortInfo) ToBytes() []byte {
	data := ""
	// name and namespace
	data += b.name + Delimiter + b.namespace + Delimiter

	// ips
	for _, ip := range b.clusterIPs {
		data += ip + Delimiter
	}
	for _, ip := range b.loadbalancerIPs {
		data += ip + Delimiter
	}
	for _, ip := range b.externalIPs {
		data += ip + Delimiter
	}

	// ports
	data += strconv.Itoa(int(b.port)) + Delimiter + strconv.Itoa(int(b.targetPort))
	data += Delimiter + strconv.Itoa(int(b.nodePort)) + Delimiter

	// protocol + type
	data += string(b.protocol) + Delimiter + b.serviceType.String() + Delimiter

	// schedulingMethod method + weigh
	data += b.schedulingMethod + Delimiter + string(b.weight) + Delimiter

	if b.sessionAffinity.ClientIP != nil {
		data += string(b.sessionAffinity.ClientIP.ClientIP.TimeoutSeconds) + Delimiter
	} else {
		data += "nil" + Delimiter
	}

	data += strconv.FormatBool(b.nodeLocalExternal) + Delimiter + strconv.FormatBool(b.nodeLocalInternal)

	for _, ip := range b.ipFilterTargetIps {
		data += ip + Delimiter
	}

	for _, source := range b.ipFilterSourceRanges {
		data += source + Delimiter
	}

	return []byte(data)
}

func NewEndpointInfo(svcKey string, endpointIP string, endpoint *localv1.Endpoint) *EndpointInfo {
	endpointInfo := &EndpointInfo{
		svcKey:  svcKey,
		ip:      endpointIP,
		isLocal: endpoint.GetLocal(),
		portMap: make(map[string]uint16),
	}
	for _, port := range endpoint.PortOverrides {
		endpointInfo.portMap[port.Name] = uint16(port.Port)
	}

	return endpointInfo
}

func (e *EndpointInfo) GetIP() string {
	return e.ip
}

func (e *EndpointInfo) ToBytes() []byte {
	data := ""
	data += e.svcKey + Delimiter
	data += e.ip + Delimiter
	data += strconv.FormatBool(e.isLocal) + Delimiter
	return []byte(data)
}
