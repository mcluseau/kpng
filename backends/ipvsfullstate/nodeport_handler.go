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

func (h *NodePortHandler) getServiceCounterparts(servicePortInfo *ServicePortInfo) (*ServicePortInfo, []*ServicePortInfo) {
	// clusterIPServicePortInfo is ClusterIP counterpart of the servicePortInfo
	clusterIPServicePortInfo := servicePortInfo.Clone()
	clusterIPServicePortInfo.serviceType = ClusterIPService

	// nodeServicePortInfos is list of servicePortInfo, cloned with setting IP to NodeIP instead of ClusterIP of servicePortInfo
	nodeServicePortInfos := make([]*ServicePortInfo, 0)
	for _, nodeIP := range h.proxier.nodeAddresses {
		nodeServicePortInfo := servicePortInfo.Clone()
		nodeServicePortInfo.SetIP(nodeIP)
		nodeServicePortInfos = append(nodeServicePortInfos, nodeServicePortInfo)
	}
	return clusterIPServicePortInfo, nodeServicePortInfos
}

func (h *NodePortHandler) createService(servicePortInfo *ServicePortInfo) {
	var entry *ipsetutil.Entry

	// get ClusterIP counterpart and NodePort services with IP set to NodeIPs
	clusterIPServicePortInfo, nodeServicePortInfos := h.getServiceCounterparts(servicePortInfo)

	// ClusterIP operations for NodePort Service

	// 1. create IPVS Virtual Server for ClusterIP
	h.proxier.createVirtualServer(clusterIPServicePortInfo)

	// 2. add ClusterIP entry to kubeClusterIPSet
	entry = getIPSetEntryForClusterIP("", clusterIPServicePortInfo)
	h.proxier.addEntryInIPSet(entry, h.proxier.ipsetList[kubeClusterIPSet])

	// 3. add ClusterIP to IPVS Interface
	h.proxier.addIPToIPVSInterface(clusterIPServicePortInfo.GetIP())

	// Node Port Specific operations

	// 4. create IPVS Virtual Server all NodeIPs
	for _, nodeServicePortInfo := range nodeServicePortInfos {
		h.proxier.createVirtualServer(nodeServicePortInfo)
	}

	// pick IPSET based on protocol of service
	var entries []*ipsetutil.Entry
	protocol := strings.ToLower(servicePortInfo.Protocol().String())
	ipSetName := protocolIPSetMap[protocol]
	set := h.proxier.ipsetList[ipSetName]

	// create entries for IPSET
	switch protocol {
	case ipsetutil.ProtocolTCP, ipsetutil.ProtocolUDP:
		entries = []*ipsetutil.Entry{getIPSetEntryForNodePort(servicePortInfo)}

	case ipsetutil.ProtocolSCTP:
		// Since hash ip:port is used for SCTP, all the nodeIPs to be used in the SCTP ipset entries.
		entries = []*ipsetutil.Entry{}
		for _, nodeIP := range h.proxier.nodeAddresses {
			entry = getIPSetEntryForNodePortSCTP(servicePortInfo)
			entry.IP = nodeIP
			entries = append(entries, entry)
		}
	}

	// 5. Add entries in relevant IPSET
	for _, entry = range entries {
		h.proxier.addEntryInIPSet(entry, set)
	}
}

func (h *NodePortHandler) createEndpoint(endpointInfo *EndpointInfo, servicePortInfo *ServicePortInfo) {
	// get ClusterIP counterpart and NodePort services with IP set to NodeIPs
	clusterIPServicePortInfo, nodeServicePortInfos := h.getServiceCounterparts(servicePortInfo)

	// ClusterIP operations for NodePort Service

	// 1. add EndpointIP to IPVS Load Balancer[ClusterIP]
	h.proxier.addRealServer(clusterIPServicePortInfo, endpointInfo)

	if endpointInfo.isLocal {
		// 2. add Endpoint IP to kubeLoopBackIPSet IPSET if endpoint is local
		entry := getIPSetEntryForEndPoint(endpointInfo, clusterIPServicePortInfo)
		h.proxier.addEntryInIPSet(entry, h.proxier.ipsetList[kubeLoopBackIPSet])
	}

	// Node Port Specific operations

	// 3. add EndpointIP to IPVS Load Balancer[NodeIP(s)]
	for _, nodeServicePortInfo := range nodeServicePortInfos {
		h.proxier.addRealServer(nodeServicePortInfo, endpointInfo)
	}

	//if endpointInfo.isLocal {
	//	// 4. add Endpoint IP to kubeLoopBackIPSet IPSET if endpoint is local
	//	for _, nodeServicePortInfo := range nodeServicePortInfos {
	//		entry := getIPSetEntryForEndPoint(endpointInfo, nodeServicePortInfo)
	//		h.proxier.addEntryInIPSet(entry, h.proxier.ipsetList[kubeLoopBackIPSet])
	//	}
	//}
}

// TODO what to do here ?
func (h *NodePortHandler) updateService(servicePortInfo *ServicePortInfo) {

}

// TODO what to do here ?
func (h *NodePortHandler) updateEndpoint(endpointInfo *EndpointInfo, servicePortInfo *ServicePortInfo) {

}

func (h *NodePortHandler) deleteService(servicePortInfo *ServicePortInfo) {
	var entry *ipsetutil.Entry
	// get ClusterIP counterpart and NodePort services with IP set to NodeIPs
	clusterIPServicePortInfo, nodeServicePortInfos := h.getServiceCounterparts(servicePortInfo)

	// ClusterIP operations for NodePort Service

	// 1. remove clusterIP from IPVS Interface
	h.proxier.removeIPFromIPVSInterface(clusterIPServicePortInfo.GetIP())

	// 2. remove ClusterIP entry from kubeClusterIPSet
	entry = getIPSetEntryForClusterIP("", clusterIPServicePortInfo)
	h.proxier.removeEntryFromIPSet(entry, h.proxier.ipsetList[kubeClusterIPSet])

	// 3. delete IPVS Virtual Server
	h.proxier.deleteVirtualServer(clusterIPServicePortInfo)

	// Node Port Specific operations

	// pick IPSET based on protocol of service
	var entries []*ipsetutil.Entry
	protocol := strings.ToLower(clusterIPServicePortInfo.Protocol().String())
	ipSetName := protocolIPSetMap[protocol]
	set := h.proxier.ipsetList[ipSetName]

	// create entries for IPSET

	switch protocol {
	case ipsetutil.ProtocolTCP, ipsetutil.ProtocolUDP:
		entries = []*ipsetutil.Entry{getIPSetEntryForNodePort(clusterIPServicePortInfo)}

	case ipsetutil.ProtocolSCTP:
		// Since hash ip:port is used for SCTP, all the nodeIPs to be used in the SCTP ipset entries.
		entries = []*ipsetutil.Entry{}
		for _, nodeIP := range h.proxier.nodeAddresses {
			entry = getIPSetEntryForNodePortSCTP(clusterIPServicePortInfo)
			entry.IP = nodeIP
			entries = append(entries, entry)
		}
	}

	// 4. Remove entries from relevant IPSET
	for _, entry = range entries {
		h.proxier.removeEntryFromIPSet(entry, set)
	}

	// 5. delete IPVS Virtual Server all nodeIPs
	for _, nodeServicePortInfo := range nodeServicePortInfos {
		h.proxier.deleteVirtualServer(nodeServicePortInfo)
	}

}

func (h *NodePortHandler) deleteEndpoint(endpointInfo *EndpointInfo, servicePortInfo *ServicePortInfo) {
	// get ClusterIP counterpart and NodePort services with IP set to NodeIPs
	clusterIPServicePortInfo, nodeServicePortInfos := h.getServiceCounterparts(servicePortInfo)

	// ClusterIP operations for NodePort Service

	if endpointInfo.isLocal {
		// 1. remove EndpointIP from kubeLoopBackIPSet IPSET if endpoint is local
		entry := getIPSetEntryForEndPoint(endpointInfo, clusterIPServicePortInfo)
		h.proxier.removeEntryFromIPSet(entry, h.proxier.ipsetList[kubeLoopBackIPSet])
	}

	// 2. remove EndpointIP from IPVS Load Balancer
	h.proxier.deleteRealServer(clusterIPServicePortInfo, endpointInfo)

	// Node Port Specific operations
	//if endpointInfo.isLocal {
	//	// 3. remove EndpointIP from kubeLoopBackIPSet IPSET if endpoint is local
	//	for _, nodeServicePortInfo := range nodeServicePortInfos {
	//		entry := getIPSetEntryForEndPoint(endpointInfo, nodeServicePortInfo)
	//		h.proxier.removeEntryFromIPSet(entry, h.proxier.ipsetList[kubeLoopBackIPSet])
	//	}
	//}
	// 4. remove EndpointIP from IPVS Load Balancer
	for _, nodeServicePortInfo := range nodeServicePortInfos {
		h.proxier.deleteRealServer(nodeServicePortInfo, endpointInfo)
	}
}

func (h *NodePortHandler) getServiceHandlers() map[Operation]func(*ServicePortInfo) {
	handlers := make(map[Operation]func(*ServicePortInfo))
	handlers[Create] = h.createService
	handlers[Update] = h.updateService
	handlers[Delete] = h.deleteService
	return handlers
}

func (h *NodePortHandler) getEndpointHandlers() map[Operation]func(*EndpointInfo, *ServicePortInfo) {
	handlers := make(map[Operation]func(*EndpointInfo, *ServicePortInfo))
	handlers[Create] = h.createEndpoint
	handlers[Update] = h.updateEndpoint
	handlers[Delete] = h.deleteEndpoint
	return handlers
}
