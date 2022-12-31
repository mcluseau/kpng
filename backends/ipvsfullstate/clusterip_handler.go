package ipvsfullsate

// ClusterIPHandler has all the logic for invocation of IPVS, IPSETS and INTERFACES for CRUD on ClusterIP services
type ClusterIPHandler struct {
	proxier *proxier
}

func newClusterIPHandler(proxier *proxier) *ClusterIPHandler {
	return &ClusterIPHandler{proxier: proxier}

}

func (h *ClusterIPHandler) createService(serviceInfo *ServiceInfo) {
	// 1. create IPVS Virtual Server for ClusterIP
	h.proxier.createVirtualServer(serviceInfo)

	// 2. add ClusterIP entry to kubeClusterIPSet
	entry := getIPSetEntryForClusterIP("", serviceInfo)
	h.proxier.addEntryInIPSet(entry, h.proxier.ipsetList[kubeClusterIPSet])

	// 3. add ClusterIP to IPVS Interface
	h.proxier.addIPToIPVSInterface(serviceInfo.IP)
}

func (h *ClusterIPHandler) createEndpoint(endpointInfo *EndpointInfo, serviceInfo *ServiceInfo) {
	// 1. add EndpointIP to IPVS Load Balancer
	h.proxier.addRealServer(serviceInfo, endpointInfo)

	if endpointInfo.isLocal {
		// 2. add Endpoint IP to kubeLoopBackIPSet IPSET if endpoint is local
		entry := getIPSetEntryForEndPoint(endpointInfo, serviceInfo)
		h.proxier.addEntryInIPSet(entry, h.proxier.ipsetList[kubeLoopBackIPSet])
	}
}

// TODO what to do here ?
func (h *ClusterIPHandler) updateService(serviceInfo *ServiceInfo) {

}

// TODO what to do here ?
func (h *ClusterIPHandler) updateEndpoint(endpointInfo *EndpointInfo, serviceInfo *ServiceInfo) {

}

func (h *ClusterIPHandler) deleteService(serviceInfo *ServiceInfo) {
	// 1. remove clusterIP from IPVS Interface
	h.proxier.removeIPFromIPVSInterface(serviceInfo.IP)

	// 2. remove ClusterIP entry from kubeClusterIPSet
	entry := getIPSetEntryForClusterIP("", serviceInfo)
	h.proxier.removeEntryFromIPSet(entry, h.proxier.ipsetList[kubeClusterIPSet])

	// 3. delete IPVS Virtual Server
	h.proxier.deleteVirtualServer(serviceInfo)
}

func (h *ClusterIPHandler) deleteEndpoint(endpointInfo *EndpointInfo, serviceInfo *ServiceInfo) {
	if endpointInfo.isLocal {
		// 1. remove EndpointIP from kubeLoopBackIPSet IPSET if endpoint is local
		entry := getIPSetEntryForEndPoint(endpointInfo, serviceInfo)
		h.proxier.removeEntryFromIPSet(entry, h.proxier.ipsetList[kubeLoopBackIPSet])
	}

	// 2. remove EndpointIP from IPVS Load Balancer
	h.proxier.deleteRealServer(serviceInfo, endpointInfo)
}

func (h *ClusterIPHandler) getServiceHandlers() map[Operation]func(*ServiceInfo) {
	// CRUD services
	handlers := make(map[Operation]func(*ServiceInfo))
	handlers[Create] = h.createService
	handlers[Update] = h.updateService
	handlers[Delete] = h.deleteService
	return handlers
}

func (h *ClusterIPHandler) getEndpointHandlers() map[Operation]func(*EndpointInfo, *ServiceInfo) {
	// CRUD endpoints
	handlers := make(map[Operation]func(*EndpointInfo, *ServiceInfo))
	handlers[Create] = h.createEndpoint
	handlers[Update] = h.updateEndpoint
	handlers[Delete] = h.deleteEndpoint
	return handlers
}
