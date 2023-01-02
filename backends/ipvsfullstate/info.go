package ipvsfullsate

import (
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/kpng/api/localv1"
	"sigs.k8s.io/kpng/client/serviceevents"
	"strconv"
)

func NewServicePortInfo(service *localv1.Service, port *localv1.PortMapping, schedulingMethod string, weight int32) *ServicePortInfo {

	clusterIP := getClusterIPByFamily(v1.IPv4Protocol, service)
	lbIP := getLoadBalancerIPByFamily(v1.IPv4Protocol, service)
	externalIp := getLoadBalancerIPByFamily(v1.IPv4Protocol, service)

	serviceType := ServiceType(service.GetType())
	ipFilterTargetIps, ipFilterSourceRanges := getIPFilterTargetIpsAndSourceRanges(v1.IPv4Protocol, service)
	return &ServicePortInfo{
		name:                 service.Name,
		namespace:            service.Namespace,
		isNew:                true,
		clusterIP:            clusterIP,
		loadbalancerIP:       lbIP,
		externalIP:           externalIp,
		port:                 uint16(port.Port),
		targetPort:           uint16(port.TargetPort),
		targetPortName:       port.Name,
		nodePort:             uint16(port.NodePort),
		protocol:             port.Protocol,
		schedulingMethod:     schedulingMethod,
		weight:               weight,
		serviceType:          serviceType,
		sessionAffinity:      serviceevents.GetSessionAffinity(service.SessionAffinity),
		nodeLocalExternal:    service.GetExternalTrafficToLocal(),
		nodeLocalInternal:    service.GetInternalTrafficToLocal(),
		ipFilterTargetIps:    ipFilterTargetIps,
		ipFilterSourceRanges: ipFilterSourceRanges,
	}
}

// Clone creates a deep-copy of ServicePortInfo object
func (b *ServicePortInfo) Clone() *ServicePortInfo {
	return &ServicePortInfo{
		name:                 b.name,
		namespace:            b.namespace,
		clusterIP:            b.clusterIP,
		loadbalancerIP:       b.loadbalancerIP,
		externalIP:           b.externalIP,
		port:                 b.port,
		targetPort:           b.targetPort,
		targetPortName:       b.targetPortName,
		nodePort:             b.nodePort,
		protocol:             b.protocol,
		schedulingMethod:     b.schedulingMethod,
		weight:               b.weight,
		serviceType:          b.serviceType,
		sessionAffinity:      b.sessionAffinity,
		nodeLocalExternal:    b.nodeLocalExternal,
		nodeLocalInternal:    b.nodeLocalInternal,
		ipFilterTargetIps:    b.ipFilterTargetIps,
		ipFilterSourceRanges: b.ipFilterSourceRanges,
	}
}

// NamespacedName returns namespace + name
func (b *ServicePortInfo) NamespacedName() string {
	return b.namespace + "/" + b.name
}

// GetClusterIP returns service ip
func (b *ServicePortInfo) GetClusterIP() string {
	return b.clusterIP
}

// GetLoadBalancerIP returns service LB ip
func (b *ServicePortInfo) GetLoadBalancerIP() string {
	return b.loadbalancerIP
}

// GetExternalIP returns service LB ip
func (b *ServicePortInfo) GetExternalIP() string {
	return b.externalIP
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
		Port:             uint16(b.port),
		Protocol:         b.protocol,
		NodePort:         uint16(b.nodePort),
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
	data += b.clusterIP + Delimiter + b.loadbalancerIP + Delimiter + b.externalIP + Delimiter

	// ports
	data += string(b.port) + Delimiter + string(b.targetPort) + Delimiter + string(b.nodePort) + Delimiter

	// protocol + type
	data += string(b.protocol) + Delimiter + b.serviceType.String() + Delimiter

	// schedulingMethod method + weigh
	data += b.schedulingMethod + Delimiter + string(b.weight) + Delimiter

	if b.sessionAffinity.ClientIP != nil && b.sessionAffinity.ClientIP.ClientIP != nil {
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
		isNew:   true,
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
