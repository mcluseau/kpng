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
	"fmt"
	"sort"

	"github.com/vishvananda/netlink"
	v1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/kpng/backends/ipvsfullstate/exec"
	"sigs.k8s.io/kpng/backends/ipvsfullstate/util"
	"sigs.k8s.io/kpng/client"
	"sigs.k8s.io/kpng/client/lightdiffstore"
	"sync"
	"time"
)

type IpvsController struct {
	mu sync.Mutex

	ipFamily v1.IPFamily

	// service store for storing ServicePortInfo objects in diffstore
	svcStore *lightdiffstore.DiffStore

	// endpoint store for storing EndpointInfo  objects in diffstore
	epStore *lightdiffstore.DiffStore

	iptables util.IPTableInterface
	ipset    util.Interface
	exec     exec.Interface
	proxier  *proxier

	// Handlers hold the actual networking logic and interactions with kernel modules
	// we need handler for all types of services; see Handler interface for reference
	handlers map[ServiceType]Handler
}

// NewIPVSController returns fully initialized IpvsController
func NewIPVSController(dummy netlink.Link) IpvsController {
	execer := exec.New()
	ipsetInterface := util.New(execer)
	iptInterface := util.NewIPTableInterface(execer, util.Protocol(v1.IPv4Protocol))

	masqueradeBit := 14
	masqueradeValue := 1 << uint(masqueradeBit)
	masqueradeMark := fmt.Sprintf("%#08x", masqueradeValue)

	// initialize Proxier which interacts with the kernel
	ipv4Proxier := NewProxier(
		v1.IPv4Protocol,
		dummy,
		ipsetInterface,
		iptInterface,
		interfaceAddresses(),
		IPVSSchedulingMethod,
		masqueradeMark,
		true,
		IPVSWeight,
	)

	// initialize IPSets
	ipv4Proxier.initializeIPSets()

	// create IPTable rules for IPSets
	ipv4Proxier.setIPTableRulesForIPVS()

	// populate service handlers
	handlers := make(map[ServiceType]Handler)
	handlers[ClusterIPService] = newClusterIPHandler(ipv4Proxier)
	handlers[NodePortService] = newNodePortHandler(ipv4Proxier)
	// TODO - add handler for LoadBalancer serviceType

	return IpvsController{
		svcStore: lightdiffstore.New(),
		epStore:  lightdiffstore.New(),
		ipFamily: v1.IPv4Protocol,
		proxier:  ipv4Proxier,
		handlers: handlers,
	}
}

func (c *IpvsController) Callback(ch <-chan *client.ServiceEndpoints) {
	// TODO - go through client n fullstate code to see if this lock is required ?
	c.mu.Lock()
	defer c.mu.Unlock()

	// for tracking time
	st := time.Now()

	// reset both the stores to capture changes
	c.svcStore.Reset(lightdiffstore.ItemDeleted)
	c.epStore.Reset(lightdiffstore.ItemDeleted)

	// iterate over the ServiceEndpoints
	for serviceEndpoints := range ch {

		service := serviceEndpoints.Service
		endpoints := serviceEndpoints.Endpoints

		klog.V(3).Infof("received service %s with %d endpoints", service.NamespacedName(), len(endpoints))

		for _, port := range service.Ports {

			// ServicePortInfo, can be directly consumed by proxier
			servicePortInfo := NewServicePortInfo(service, port, IPVSSchedulingMethod, IPVSWeight)

			// generate service key -> [hash of everything in ServicePortInfo]
			// using this key, we can trick the diffstore to convert an Update operation into Delete + Create
			// any change in service will create new, key thus for update we delete the old one and create new one
			svcKey := getSvcKey(servicePortInfo)

			// add ServicePortInfo to service diffstore
			c.svcStore.Set([]byte(svcKey), getHashForDiffstore(servicePortInfo), *servicePortInfo)

			// iterate over all endpoints
			for _, endpoint := range endpoints {
				for _, endpointIP := range endpoint.GetIPs().V4 {

					// generate endpoint; key format: [service key + endpoint ip]
					epKey := getEpKey(svcKey, endpointIP)

					// EndpointInfo, can be directly consumed by proxier
					endpointInfo := NewEndpointInfo(svcKey, endpointIP, endpoint)

					// add EndpointInfo to endpoint diffstore
					c.epStore.Set([]byte(epKey), getHashForDiffstore(endpointInfo), *endpointInfo)
				}
			}
		}
	}

	// prepare patch for network layer
	// here a patch will  represent delta in a fullstate.ServiceEndpoints after processing the callback
	patches := c.generatePatches()

	// apply all the patches to manipulate the network layer for the required changes
	c.apply(patches)

	et := time.Now()
	klog.V(3).Infof("took %.2f ms to sync", 1000*et.Sub(st).Seconds())
}

func (c *IpvsController) apply(patches []ServiceEndpointsPatch) {

	// When applying patches we need to make sure all Delete operations occurs before Create operations
	// This handles corner cases of service update. If we execute Create before Delete, Create will be a NoOp since objects will
	// already exist in the network layer and Delete will just remove those objects
	// sorting patches on integer value of Operation
	sort.Slice(patches, func(i, j int) bool {
		return patches[i].svc.op > patches[j].svc.op
	})

	for _, patch := range patches {
		klog.V(3).Infof("patch group: service=%s; %d; endpoints=%d",
			patch.svc.servicePortInfo.NamespacedName(), patch.svc.op, len(patch.eps))

		// get handler for the serviceType
		serviceType := patch.svc.servicePortInfo.serviceType

		// handler contains set of functions which will interact with kernel
		handler, ok := c.handlers[serviceType]
		if ok {
			// apply the patch
			patch.apply(handler.getServiceHandlers(), handler.getEndpointHandlers())
		} else {
			klog.V(3).Infof("IPVS fullstate not yet implemented for %v", serviceType)
		}
	}
}
