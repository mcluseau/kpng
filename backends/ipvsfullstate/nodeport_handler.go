package ipvsfullsate

import (
	ipsetutil "sigs.k8s.io/kpng/backends/ipvsfullstate/util"
	"strings"
)

type NodePortHandler struct {
	proxier *proxier
}

func newNodePortHandler(proxier *proxier) *NodePortHandler {
	return &NodePortHandler{proxier: proxier}
}

func (h *NodePortHandler) getServiceCounterparts(serviceInfo *ServiceInfo) (*ServiceInfo, []*ServiceInfo) {
	// clusterIPServiceInfo is ClusterIP counterpart of the serviceInfo
	clusterIPServiceInfo := serviceInfo.Clone()
	clusterIPServiceInfo.serviceType = ClusterIPService

	// nodeServiceInfos is list of serviceInfo, cloned with setting IP to NodeIP instead of ClusterIP of serviceInfo
	nodeServiceInfos := make([]*ServiceInfo, 0)
	for _, nodeIP := range h.proxier.nodeAddresses {
		nodeServiceInfo := serviceInfo.Clone()
		nodeServiceInfo.IP = nodeIP
		nodeServiceInfos = append(nodeServiceInfos, nodeServiceInfo)
	}
	return clusterIPServiceInfo, nodeServiceInfos
}

func (h *NodePortHandler) createService(serviceInfo *ServiceInfo) {
	var entry *ipsetutil.Entry

	// get ClusterIP counterpart and NodePort services with IP set to NodeIPs
	clusterIPServiceInfo, nodeServiceInfos := h.getServiceCounterparts(serviceInfo)

	// ClusterIP operations for NodePort Service

	// 1. create IPVS Virtual Server for ClusterIP
	h.proxier.createVirtualServer(clusterIPServiceInfo)

	// 2. add ClusterIP entry to kubeClusterIPSet
	entry = getIPSetEntryForClusterIP("", clusterIPServiceInfo)
	h.proxier.addEntryInIPSet(entry, h.proxier.ipsetList[kubeClusterIPSet])

	// 3. add ClusterIP to IPVS Interface
	h.proxier.addIPToIPVSInterface(clusterIPServiceInfo.IP)

	// Node Port Specific operations

	// 4. create IPVS Virtual Server all NodeIPs
	for _, nodeServiceInfo := range nodeServiceInfos {
		h.proxier.createVirtualServer(nodeServiceInfo)
	}

	// pick IPSET based on protocol of service
	var entries []*ipsetutil.Entry
	protocol := strings.ToLower(serviceInfo.Protocol().String())
	ipSetName := protocolIPSetMap[protocol]
	set := h.proxier.ipsetList[ipSetName]

	// create entries for IPSET
	switch protocol {
	case ipsetutil.ProtocolTCP, ipsetutil.ProtocolUDP:
		entries = []*ipsetutil.Entry{getIPSetEntryForNodePort(serviceInfo)}

	case ipsetutil.ProtocolSCTP:
		// Since hash ip:port is used for SCTP, all the nodeIPs to be used in the SCTP ipset entries.
		entries = []*ipsetutil.Entry{}
		for _, nodeIP := range h.proxier.nodeAddresses {
			entry = getIPSetEntryForNodePortSCTP(serviceInfo)
			entry.IP = nodeIP
			entries = append(entries, entry)
		}
	}

	// 5. Add entries in relevant IPSET
	for _, entry = range entries {
		h.proxier.addEntryInIPSet(entry, set)
	}
}

func (h *NodePortHandler) createEndpoint(endpointInfo *EndpointInfo, serviceInfo *ServiceInfo) {
	// get ClusterIP counterpart and NodePort services with IP set to NodeIPs
	clusterIPServiceInfo, nodeServiceInfos := h.getServiceCounterparts(serviceInfo)

	// ClusterIP operations for NodePort Service

	// 1. add EndpointIP to IPVS Load Balancer[ClusterIP]
	h.proxier.addRealServer(clusterIPServiceInfo, endpointInfo)

	if endpointInfo.isLocal {
		// 2. add Endpoint IP to kubeLoopBackIPSet IPSET if endpoint is local
		entry := getIPSetEntryForEndPoint(endpointInfo, clusterIPServiceInfo)
		h.proxier.addEntryInIPSet(entry, h.proxier.ipsetList[kubeLoopBackIPSet])
	}

	// Node Port Specific operations

	// 3. add EndpointIP to IPVS Load Balancer[NodeIP(s)]
	for _, nodeServiceInfo := range nodeServiceInfos {
		h.proxier.addRealServer(nodeServiceInfo, endpointInfo)
	}

	if endpointInfo.isLocal {
		// 4. add Endpoint IP to kubeLoopBackIPSet IPSET if endpoint is local
		for _, nodeServiceInfo := range nodeServiceInfos {
			entry := getIPSetEntryForEndPoint(endpointInfo, nodeServiceInfo)
			h.proxier.addEntryInIPSet(entry, h.proxier.ipsetList[kubeLoopBackIPSet])
		}
	}
}

// TODO what to do here ?
func (h *NodePortHandler) updateService(serviceInfo *ServiceInfo) {

}

// TODO what to do here ?
func (h *NodePortHandler) updateEndpoint(endpointInfo *EndpointInfo, serviceInfo *ServiceInfo) {

}

func (h *NodePortHandler) deleteService(serviceInfo *ServiceInfo) {
	var entry *ipsetutil.Entry
	// get ClusterIP counterpart and NodePort services with IP set to NodeIPs
	clusterIPServiceInfo, nodeServiceInfos := h.getServiceCounterparts(serviceInfo)

	// ClusterIP operations for NodePort Service

	// 1. remove clusterIP from IPVS Interface
	h.proxier.removeIPFromIPVSInterface(clusterIPServiceInfo.IP)

	// 2. remove ClusterIP entry from kubeClusterIPSet
	entry = getIPSetEntryForClusterIP("", clusterIPServiceInfo)
	h.proxier.removeEntryFromIPSet(entry, h.proxier.ipsetList[kubeClusterIPSet])

	// 3. delete IPVS Virtual Server
	h.proxier.deleteVirtualServer(clusterIPServiceInfo)

	// Node Port Specific operations

	// pick IPSET based on protocol of service
	var entries []*ipsetutil.Entry
	protocol := strings.ToLower(serviceInfo.Protocol().String())
	ipSetName := protocolIPSetMap[protocol]
	set := h.proxier.ipsetList[ipSetName]

	// create entries for IPSET

	switch protocol {
	case ipsetutil.ProtocolTCP, ipsetutil.ProtocolUDP:
		entries = []*ipsetutil.Entry{getIPSetEntryForNodePort(serviceInfo)}

	case ipsetutil.ProtocolSCTP:
		// Since hash ip:port is used for SCTP, all the nodeIPs to be used in the SCTP ipset entries.
		entries = []*ipsetutil.Entry{}
		for _, nodeIP := range h.proxier.nodeAddresses {
			entry = getIPSetEntryForNodePortSCTP(serviceInfo)
			entry.IP = nodeIP
			entries = append(entries, entry)
		}
	}

	// 4. Remove entries from relevant IPSET
	for _, entry = range entries {
		h.proxier.removeEntryFromIPSet(entry, set)
	}

	// 5. delete IPVS Virtual Server all nodeIPs
	for _, nodeServiceInfo := range nodeServiceInfos {
		h.proxier.deleteVirtualServer(nodeServiceInfo)
	}

}

func (h *NodePortHandler) deleteEndpoint(endpointInfo *EndpointInfo, serviceInfo *ServiceInfo) {
	// get ClusterIP counterpart and NodePort services with IP set to NodeIPs
	clusterIPServiceInfo, nodeServiceInfos := h.getServiceCounterparts(serviceInfo)

	// ClusterIP operations for NodePort Service

	if endpointInfo.isLocal {
		// 1. remove EndpointIP from kubeLoopBackIPSet IPSET if endpoint is local
		entry := getIPSetEntryForEndPoint(endpointInfo, clusterIPServiceInfo)
		h.proxier.removeEntryFromIPSet(entry, h.proxier.ipsetList[kubeLoopBackIPSet])
	}

	// 2. remove EndpointIP from IPVS Load Balancer
	h.proxier.deleteRealServer(clusterIPServiceInfo, endpointInfo)

	// Node Port Specific operations

	if endpointInfo.isLocal {
		// 3. remove EndpointIP from kubeLoopBackIPSet IPSET if endpoint is local
		for _, nodeServiceInfo := range nodeServiceInfos {
			entry := getIPSetEntryForEndPoint(endpointInfo, nodeServiceInfo)
			h.proxier.removeEntryFromIPSet(entry, h.proxier.ipsetList[kubeLoopBackIPSet])
		}
	}
	// 4. remove EndpointIP from IPVS Load Balancer
	for _, nodeServiceInfo := range nodeServiceInfos {
		h.proxier.deleteRealServer(nodeServiceInfo, endpointInfo)
	}
}

func (h *NodePortHandler) getServiceHandlers() map[Operation]func(*ServiceInfo) {
	handlers := make(map[Operation]func(*ServiceInfo))
	handlers[Create] = h.createService
	handlers[Update] = h.updateService
	handlers[Delete] = h.deleteService
	return handlers
}

func (h *NodePortHandler) getEndpointHandlers() map[Operation]func(*EndpointInfo, *ServiceInfo) {
	handlers := make(map[Operation]func(*EndpointInfo, *ServiceInfo))
	handlers[Create] = h.createEndpoint
	handlers[Update] = h.updateEndpoint
	handlers[Delete] = h.deleteEndpoint
	return handlers
}
