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

import ipsetutil "sigs.k8s.io/kpng/backends/ipvsfullstate/util"

// ClusterIPHandler has all the logic for invocation of IPVS, IPSETS and INTERFACES for CRUD on ClusterIP services
type ClusterIPHandler struct {
	proxier *proxier
}

func newClusterIPHandler(proxier *proxier) *ClusterIPHandler {
	return &ClusterIPHandler{proxier: proxier}

}

func (h *ClusterIPHandler) createService(servicePortInfo *ServicePortInfo) {
	var entries []*ipsetutil.Entry

	// 1. create IPVS Virtual Server for ClusterIP
	h.proxier.createVirtualServerForClusterIPs(servicePortInfo)

	// 2. add ClusterIP entry to kubeClusterIPSet
	entries = getIPSetEntriesForClusterIP("", servicePortInfo)
	h.proxier.addEntriesToIPSet(entries, h.proxier.ipsetList[kubeClusterIPSet])

	// 3. add ClusterIP to IPVS Interface
	h.proxier.addIPsToIPVSInterface(servicePortInfo.GetClusterIPs())

	// if service has an external IP
	if len(servicePortInfo.GetExternalIPs()) > 0 {

		// 4. create IPVS Virtual Server for ExternalIP
		h.proxier.createVirtualServerForExternalIP(servicePortInfo)

		// 5. add ExternalIP entry to kubeExternalIPSet
		entries = getIPSetEntriesForExternalIPs("", servicePortInfo)
		h.proxier.addEntriesToIPSet(entries, h.proxier.ipsetList[kubeExternalIPSet])
	}

}

func (h *ClusterIPHandler) createEndpoint(endpointInfo *EndpointInfo, servicePortInfo *ServicePortInfo) {
	// 1. add EndpointIP to IPVS Load Balancer for ClusterIP
	h.proxier.addRealServerForClusterIPs(servicePortInfo, endpointInfo)

	if endpointInfo.isLocal {
		// 2. add Endpoint IP to kubeLoopBackIPSet IPSET if endpoint is local
		entry := getIPSetEntryForEndPoint(endpointInfo, servicePortInfo)
		h.proxier.addEntryInIPSet(entry, h.proxier.ipsetList[kubeLoopBackIPSet])
	}

	// if service has an external IP
	if len(servicePortInfo.GetExternalIPs()) > 0 {
		// 3. add EndpointIP to IPVS Load Balancer for ExternalIP
		h.proxier.addRealServerForExternalIPs(servicePortInfo, endpointInfo)
	}
}

func (h *ClusterIPHandler) deleteService(servicePortInfo *ServicePortInfo) {
	var entries []*ipsetutil.Entry

	// 1. remove clusterIP from IPVS Interface
	h.proxier.removeIPsFromIPVSInterface(servicePortInfo.GetClusterIPs())

	// 2. remove ClusterIP entry from kubeClusterIPSet
	entries = getIPSetEntriesForClusterIP("", servicePortInfo)
	h.proxier.removeEntriesFromIPSet(entries, h.proxier.ipsetList[kubeClusterIPSet])

	// 3. delete IPVS Virtual Server
	h.proxier.deleteVirtualServerForClusterIP(servicePortInfo)

	// if service has an external IP
	if len(servicePortInfo.GetExternalIPs()) > 0 {

		// 4. add ExternalIP entry to kubeExternalIPSet
		entries = getIPSetEntriesForExternalIPs("", servicePortInfo)
		h.proxier.removeEntriesFromIPSet(entries, h.proxier.ipsetList[kubeExternalIPSet])

		// 5. create IPVS Virtual Server for ExternalIP
		h.proxier.deleteVirtualServerForExternalIP(servicePortInfo)
	}
}

func (h *ClusterIPHandler) deleteEndpoint(endpointInfo *EndpointInfo, servicePortInfo *ServicePortInfo) {

	// if service has an external IP
	if len(servicePortInfo.GetExternalIPs()) > 0 {
		// 3. remove EndpointIP from IPVS Load Balancer for ExternalIP
		h.proxier.deleteRealServerForExternalIPs(servicePortInfo, endpointInfo)
	}

	if endpointInfo.isLocal {
		// 2. remove EndpointIP from kubeLoopBackIPSet IPSET if endpoint is local
		entry := getIPSetEntryForEndPoint(endpointInfo, servicePortInfo)
		h.proxier.removeEntryFromIPSet(entry, h.proxier.ipsetList[kubeLoopBackIPSet])
	}

	// 3. remove EndpointIP from IPVS Load Balancer for ClusterIP
	h.proxier.deleteRealServerForClusterIPs(servicePortInfo, endpointInfo)
}

func (h *ClusterIPHandler) getServiceHandlers() map[Operation]func(*ServicePortInfo) {
	// CRUD services
	handlers := make(map[Operation]func(*ServicePortInfo))
	handlers[Create] = h.createService
	handlers[Delete] = h.deleteService
	return handlers
}

func (h *ClusterIPHandler) getEndpointHandlers() map[Operation]func(*EndpointInfo, *ServicePortInfo) {
	// CRUD endpoints
	handlers := make(map[Operation]func(*EndpointInfo, *ServicePortInfo))
	handlers[Create] = h.createEndpoint
	handlers[Delete] = h.deleteEndpoint
	return handlers
}
