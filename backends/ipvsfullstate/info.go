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
	serviceType := ServiceType(service.GetType())
	ipFilterTargetIps, ipFilterSourceRanges := getIPFilterTargetIpsAndSourceRanges(v1.IPv4Protocol, service)
	return &ServicePortInfo{
		name:                 service.Name,
		namespace:            service.Namespace,
		isNew:                true,
		ip:                   clusterIP,
		lbIP:                 lbIP,
		port:                 port.Port,
		targetPort:           port.TargetPort,
		targetPortName:       port.Name,
		nodePort:             port.NodePort,
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
		ip:                   b.ip,
		lbIP:                 b.lbIP,
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

// GetIP returns service ip
func (b *ServicePortInfo) GetIP() string {
	return b.ip
}

// LoadBalancerIP returns service LB ip
func (b *ServicePortInfo) LoadBalancerIP() string {
	return b.lbIP
}

// Port return service port
func (b *ServicePortInfo) Port() int32 {
	return b.port
}

// NodePort return service node port
func (b *ServicePortInfo) NodePort() int32 {
	return b.nodePort
}

// TargetPort return service target port
func (b *ServicePortInfo) TargetPort() int32 {
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
func (b *ServicePortInfo) GetVirtualServer() ipvsLB {
	vs := ipvsLB{
		IP:               b.ip,
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

// SetIP update IP of ServicePortInfo
func (b *ServicePortInfo) SetIP(IP string) {
	b.ip = IP
}

// ToBytes returns ServicePortInfo as []byte
func (b *ServicePortInfo) ToBytes() []byte {
	data := ""
	// name and namespace
	data += b.name + Delimiter + b.namespace + Delimiter

	// ips
	data += b.ip + Delimiter + b.lbIP + Delimiter

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
	return &EndpointInfo{
		svcKey:  svcKey,
		isNew:   true,
		ip:      endpointIP,
		isLocal: endpoint.GetLocal(),
	}
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
