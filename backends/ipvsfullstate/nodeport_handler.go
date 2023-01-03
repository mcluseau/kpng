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
	ipsetutil "sigs.k8s.io/kpng/backends/ipvsfullstate/util"
	"strings"
)

type NodePortHandler struct {
	proxier *proxier
}

func newNodePortHandler(proxier *proxier) *NodePortHandler {
	return &NodePortHandler{proxier: proxier}
}

func (h *NodePortHandler) createService(servicePortInfo *ServicePortInfo) {
	var entry *ipsetutil.Entry
	var entries []*ipsetutil.Entry

	// ClusterIP operations for NodePort Service

	// 1. create IPVS Virtual Server for ClusterIP
	h.proxier.createVirtualServerForClusterIPs(servicePortInfo)

	// 2. add ClusterIP entry to kubeClusterIPSet
	entries = getIPSetEntriesForClusterIP("", servicePortInfo)
	h.proxier.addEntriesToIPSet(entries, h.proxier.ipsetList[kubeClusterIPSet])

	// 3. add ClusterIP to IPVS Interface
	h.proxier.addIPsToIPVSInterface(servicePortInfo.GetClusterIPs())

	// Node Port Specific operations

	// 4. create IPVS Virtual Server all NodeIPs
	h.proxier.createVirtualServerForNodeIPs(servicePortInfo)

	// pick IPSET based on protocol of service
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
	// ClusterIP operations for NodePort Service

	// 1. add EndpointIP to IPVS Load Balancer[ClusterIP]
	h.proxier.addRealServerForClusterIPs(servicePortInfo, endpointInfo)

	if endpointInfo.isLocal {
		// 2. add Endpoint IP to kubeLoopBackIPSet IPSET if endpoint is local
		entry := getIPSetEntryForEndPoint(endpointInfo, servicePortInfo)
		h.proxier.addEntryInIPSet(entry, h.proxier.ipsetList[kubeLoopBackIPSet])
	}

	// Node Port Specific operations

	// 3. add EndpointIP to IPVS Load Balancer[NodeIP(s)]
	h.proxier.addRealServerForNodeIPs(servicePortInfo, endpointInfo)

}

func (h *NodePortHandler) deleteService(servicePortInfo *ServicePortInfo) {
	var entry *ipsetutil.Entry
	var entries []*ipsetutil.Entry

	// ClusterIP operations for NodePort Service

	// 1. remove clusterIP from IPVS Interface
	h.proxier.removeIPsFromIPVSInterface(servicePortInfo.GetClusterIPs())

	// 2. remove ClusterIP entry from kubeClusterIPSet
	entries = getIPSetEntriesForClusterIP("", servicePortInfo)
	h.proxier.removeEntriesFromIPSet(entries, h.proxier.ipsetList[kubeClusterIPSet])

	// 3. delete IPVS Virtual Server
	h.proxier.deleteVirtualServerForClusterIP(servicePortInfo)

	// Node Port Specific operations

	// pick IPSET based on protocol of service
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

	// 4. Remove entries from relevant IPSET
	h.proxier.removeEntriesFromIPSet(entries, set)

	// 5. delete IPVS Virtual Server all nodeIPs
	h.proxier.deleteVirtualServerForNodeIPs(servicePortInfo)

}

func (h *NodePortHandler) deleteEndpoint(endpointInfo *EndpointInfo, servicePortInfo *ServicePortInfo) {
	// ClusterIP operations for NodePort Service

	if endpointInfo.isLocal {
		// 1. remove EndpointIP from kubeLoopBackIPSet IPSET if endpoint is local
		entry := getIPSetEntryForEndPoint(endpointInfo, servicePortInfo)
		h.proxier.removeEntryFromIPSet(entry, h.proxier.ipsetList[kubeLoopBackIPSet])
	}

	// 2. remove EndpointIP from IPVS Load Balancer
	h.proxier.deleteRealServerForClusterIPs(servicePortInfo, endpointInfo)

	// Node Port Specific operations
	//if endpointInfo.isLocal {
	//	// 3. remove EndpointIP from kubeLoopBackIPSet IPSET if endpoint is local
	//	for _, nodeServicePortInfo := range nodeServicePortInfos {
	//		entry := getIPSetEntryForEndPoint(endpointInfo, nodeServicePortInfo)
	//		h.proxier.removeEntryFromIPSet(entry, h.proxier.ipsetList[kubeLoopBackIPSet])
	//	}
	//}

	// 4. remove EndpointIP from IPVS Load Balancer
	h.proxier.deleteRealServerForNodeIPs(servicePortInfo, endpointInfo)

}

func (h *NodePortHandler) getServiceHandlers() map[Operation]func(*ServicePortInfo) {
	handlers := make(map[Operation]func(*ServicePortInfo))
	handlers[Create] = h.createService
	handlers[Delete] = h.deleteService
	return handlers
}

func (h *NodePortHandler) getEndpointHandlers() map[Operation]func(*EndpointInfo, *ServicePortInfo) {
	handlers := make(map[Operation]func(*EndpointInfo, *ServicePortInfo))
	handlers[Create] = h.createEndpoint
	handlers[Delete] = h.deleteEndpoint
	return handlers
}
