package ipvsfullsate

import (
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/kpng/api/localv1"
	"sigs.k8s.io/kpng/client/serviceevents"
)

func NewServiceInfo(service *localv1.Service, port *localv1.PortMapping,
	isNew bool, schedulingMethod string, weight int32) *ServiceInfo {

	clusterIP := getClusterIPByFamily(v1.IPv4Protocol, service)
	lbIP := getLoadBalancerIPByFamily(v1.IPv4Protocol, service)
	serviceType := ServiceType(service.GetType())

	return &ServiceInfo{
		IP:                clusterIP,
		LBIP:              lbIP,
		isNew:             isNew,
		port:              port.Port,
		targetPort:        port.TargetPort,
		targetPortName:    port.Name,
		nodePort:          port.NodePort,
		protocol:          port.Protocol,
		schedulingMethod:  schedulingMethod,
		weight:            weight,
		serviceType:       serviceType,
		sessionAffinity:   serviceevents.GetSessionAffinity(service.SessionAffinity),
		nodeLocalExternal: service.GetExternalTrafficToLocal(),
		nodeLocalInternal: service.GetInternalTrafficToLocal(),
		ipFilters:         service.IPFilters,
	}
}

// Clone creates a deep-copy of ServiceInfo object
func (b *ServiceInfo) Clone() *ServiceInfo {
	return &ServiceInfo{
		IP:               b.IP,
		isNew:            b.isNew,
		port:             b.port,
		targetPort:       b.targetPort,
		targetPortName:   b.targetPortName,
		nodePort:         b.nodePort,
		protocol:         b.protocol,
		schedulingMethod: b.schedulingMethod,
		weight:           b.weight,
		serviceType:      b.serviceType,
		sessionAffinity:  b.sessionAffinity,
	}
}

// ServiceIP returns service IP
func (b *ServiceInfo) ServiceIP() string {
	return b.IP
}

// Port return service port
func (b *ServiceInfo) Port() int32 {
	return b.port
}

// NodePort return service node port
func (b *ServiceInfo) NodePort() int32 {
	return b.nodePort
}

// TargetPort return service target port
func (b *ServiceInfo) TargetPort() int32 {
	return b.targetPort
}

// TargetPortName return name of the target port
func (b *ServiceInfo) TargetPortName() string {
	return b.targetPortName
}

// Protocol return service protocol
func (b *ServiceInfo) Protocol() localv1.Protocol {
	return b.protocol
}

// GetVirtualServer return IPVS LB object
func (b *ServiceInfo) GetVirtualServer() ipvsLB {
	vs := ipvsLB{
		IP:               b.IP,
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
