package ipvsfullsate

// ClusterIPHandler has all the logic for invocation of IPVS, IPSETS and INTERFACES for CRUD on ClusterIP services
type ClusterIPHandler struct {
	proxier *proxier
}

func newClusterIPHandler(proxier *proxier) *ClusterIPHandler {
	return &ClusterIPHandler{proxier: proxier}

}

func (h *ClusterIPHandler) createService(servicePortInfo *ServicePortInfo) {
	// 1. create IPVS Virtual Server for ClusterIP
	h.proxier.createVirtualServer(servicePortInfo)

	// 2. add ClusterIP entry to kubeClusterIPSet
	entry := getIPSetEntryForClusterIP("", servicePortInfo)
	h.proxier.addEntryInIPSet(entry, h.proxier.ipsetList[kubeClusterIPSet])

	// 3. add ClusterIP to IPVS Interface
	h.proxier.addIPToIPVSInterface(servicePortInfo.IP)
}

func (h *ClusterIPHandler) createEndpoint(endpointInfo *EndpointInfo, servicePortInfo *ServicePortInfo) {
	// 1. add EndpointIP to IPVS Load Balancer
	h.proxier.addRealServer(servicePortInfo, endpointInfo)

	if endpointInfo.isLocal {
		// 2. add Endpoint IP to kubeLoopBackIPSet IPSET if endpoint is local
		entry := getIPSetEntryForEndPoint(endpointInfo, servicePortInfo)
		h.proxier.addEntryInIPSet(entry, h.proxier.ipsetList[kubeLoopBackIPSet])
	}
}

// TODO what to do here ?
func (h *ClusterIPHandler) updateService(servicePortInfo *ServicePortInfo) {

}

// TODO what to do here ?
func (h *ClusterIPHandler) updateEndpoint(endpointInfo *EndpointInfo, servicePortInfo *ServicePortInfo) {

}

func (h *ClusterIPHandler) deleteService(servicePortInfo *ServicePortInfo) {
	// 1. remove clusterIP from IPVS Interface
	h.proxier.removeIPFromIPVSInterface(servicePortInfo.IP)

	// 2. remove ClusterIP entry from kubeClusterIPSet
	entry := getIPSetEntryForClusterIP("", servicePortInfo)
	h.proxier.removeEntryFromIPSet(entry, h.proxier.ipsetList[kubeClusterIPSet])

	// 3. delete IPVS Virtual Server
	h.proxier.deleteVirtualServer(servicePortInfo)
}

func (h *ClusterIPHandler) deleteEndpoint(endpointInfo *EndpointInfo, servicePortInfo *ServicePortInfo) {
	if endpointInfo.isLocal {
		// 1. remove EndpointIP from kubeLoopBackIPSet IPSET if endpoint is local
		entry := getIPSetEntryForEndPoint(endpointInfo, servicePortInfo)
		h.proxier.removeEntryFromIPSet(entry, h.proxier.ipsetList[kubeLoopBackIPSet])
	}

	// 2. remove EndpointIP from IPVS Load Balancer
	h.proxier.deleteRealServer(servicePortInfo, endpointInfo)
}

func (h *ClusterIPHandler) getServiceHandlers() map[Operation]func(*ServicePortInfo) {
	// CRUD services
	handlers := make(map[Operation]func(*ServicePortInfo))
	handlers[Create] = h.createService
	handlers[Update] = h.updateService
	handlers[Delete] = h.deleteService
	return handlers
}

func (h *ClusterIPHandler) getEndpointHandlers() map[Operation]func(*EndpointInfo, *ServicePortInfo) {
	// CRUD endpoints
	handlers := make(map[Operation]func(*EndpointInfo, *ServicePortInfo))
	handlers[Create] = h.createEndpoint
	handlers[Update] = h.updateEndpoint
	handlers[Delete] = h.deleteEndpoint
	return handlers
}
