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
	"k8s.io/klog/v2"
)

// ServicePatch -> ServicePortInfo and Operation
type ServicePatch struct {
	servicePortInfo *ServicePortInfo
	op              Operation
}

// apply will invoke the handler which interacts with proxier to implement network rules
// low level networking logic is injected as a dependency. see Handler interface for reference
func (p *ServicePatch) apply(handler map[Operation]func(servicePortInfo *ServicePortInfo)) {
	handler[p.op](p.servicePortInfo)
}

// EndpointPatch -> EndpointInfo, ServicePortInfo, and Operation
type EndpointPatch struct {
	endpointInfo    *EndpointInfo
	servicePortInfo *ServicePortInfo
	op              Operation
}

// apply will invoke the handler which interacts with proxier to implement network rules
// low level networking logic is injected as a dependency. see Handler interface for reference
func (p *EndpointPatch) apply(handler map[Operation]func(endpointInfo *EndpointInfo, servicePortInfo *ServicePortInfo)) {
	handler[p.op](p.endpointInfo, p.servicePortInfo)
}

// EndpointPatches -> [] EndpointPatch
// only purpose of this type -> syntactic sugar
type EndpointPatches []EndpointPatch

// apply will call apply on each EndpointPatch
func (e EndpointPatches) apply(handler map[Operation]func(*EndpointInfo, *ServicePortInfo)) {
	for _, patch := range e {
		patch.apply(handler)
	}
}

// ServiceEndpointsPatch is a collection of ServicePatch and EndpointPatches. When applied, it will complete the transition
// of a fullstate.ServiceEndpoints from state A -> state B on underlying network layer
// ServiceEndpointsPatch = fullstate.ServiceEndpoints(after callback) - fullstate.ServiceEndpoints(before callback)
// ServiceEndpointsPatch gives control on order of execution of mutually inclusive patches
// for example: we need to create service before creating endpoints, and reverse of this for delete
// ServiceEndpointsPatch also opens possibilities for concurrent executions and rollbacks in the future.
type ServiceEndpointsPatch struct {
	svc ServicePatch
	eps EndpointPatches
}

// apply will apply ServicePatch and EndpointPatches in the order which we want
// networking logic is passed as a dependency; on application this will update the kernel to reach the desired the state
func (p *ServiceEndpointsPatch) apply(
	serviceHandler map[Operation]func(*ServicePortInfo),
	endpointHandler map[Operation]func(*EndpointInfo, *ServicePortInfo),
) {
	// switching on ServicePatch Operation and maintaining order accordingly
	switch p.svc.op {
	case NoOp:
		// service no-op; only endpoints in this case
		p.eps.apply(endpointHandler)
	case Create:
		// first service; then endpoints
		p.svc.apply(serviceHandler)
		p.eps.apply(endpointHandler)
	case Update:
		// there should be no Update Operation;
		// since we are hashing the servicePortInfo to generate key for the diffstore, an update in service
		// will result in Delete + Create Operation
		klog.Fatal("Update Operation should not exists now")
	case Delete:
		// first endpoints; then service
		p.eps.apply(endpointHandler)
		p.svc.apply(serviceHandler)
	}
}

// generatePatches prepares ServiceEndpointsPatch for the network layer using diffstore deltas after processing fullstate callback
func (c *IpvsController) generatePatches() []ServiceEndpointsPatch {
	// patchMap <service_key:ServiceEndpointsPatch>
	// this maps helps in creating patches; lookup by service; we only need the values of this map at the end
	patchMap := make(map[string]ServiceEndpointsPatch)

	//////////////////////////////////////////////// Service Store - Updates //////////////////////////////////////////////////
	// iterate and create ServiceEndpointsPatch for newly added services in store
	for _, KV := range c.svcStore.Updated() {
		svcKey := string(KV.Key)
		servicePortInfo := KV.Value.(ServicePortInfo)

		// create new patch; add service with Create operation; initialise endpoints patch
		patchMap[svcKey] = ServiceEndpointsPatch{
			ServicePatch{servicePortInfo: &servicePortInfo, op: Create},
			make([]EndpointPatch, 0),
		}
	}
	///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

	//////////////////////////////////////////////// Service Store - Deletes //////////////////////////////////////////////////
	// iterate and create ServiceEndpointsPatch for services deleted in the store
	for _, KV := range c.svcStore.Deleted() {
		svcKey := string(KV.Key)
		servicePortInfo := KV.Value.(ServicePortInfo)

		// create new patch; add service with delete operation; initialise endpoints patch
		patchMap[svcKey] = ServiceEndpointsPatch{
			ServicePatch{servicePortInfo: &servicePortInfo, op: Delete},
			make([]EndpointPatch, 0),
		}

		// delete the service from service diffstore
		c.svcStore.Delete([]byte(svcKey))
	}
	/////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

	////////////////////////////////////////////////// Endpoint Store - Updates /////////////////////////////////////////////////
	// iterate and add endpoints (with Create operation) to ServiceEndpointsPatch for newly added endpoints in store
	for _, KV := range c.epStore.Updated() {
		endpointInfo := KV.Value.(EndpointInfo)
		svcKey := endpointInfo.svcKey

		// check if patch was created above during service diffstore iterations
		if patch, ok := patchMap[svcKey]; ok {
			// append endpoint to patch with create operation
			patch.eps = append(patchMap[svcKey].eps,
				EndpointPatch{endpointInfo: &endpointInfo, servicePortInfo: patchMap[svcKey].svc.servicePortInfo, op: Create})
			patchMap[svcKey] = patch

		} else {
			// this only happens if we are adding endpoints to a pre-existing service;
			// if service is intact, diffstore won't return it in Updated() or Deleted() call and patch won't exist in patch map
			// lookup the service from the service store
			serviceResults := c.svcStore.GetByPrefix([]byte(endpointInfo.svcKey))[0]
			servicePortInfo := serviceResults.Value.(ServicePortInfo)

			// create new patch; add service with no-op operation; initialise endpoints patch with
			// endpoint and create operation
			patchMap[endpointInfo.svcKey] = ServiceEndpointsPatch{
				svc: ServicePatch{servicePortInfo: &servicePortInfo, op: NoOp},
				eps: []EndpointPatch{{endpointInfo: &endpointInfo, servicePortInfo: &servicePortInfo, op: Create}},
			}
		}
	}
	/////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

	////////////////////////////////////////////////// Endpoint Store - Deletes /////////////////////////////////////////////////
	// iterate and add endpoints (with Delete operation) to ServiceEndpointsPatch for endpoints deleted from the store
	for _, KV := range c.epStore.Deleted() {
		endpointInfo := KV.Value.(EndpointInfo)
		epKey := string(KV.Key)
		svcKey := endpointInfo.svcKey

		// check if patch was created above during service diffstore iterations
		if patch, ok := patchMap[svcKey]; ok {
			// append endpoint to patch with delete operation
			patch.eps = append(patchMap[svcKey].eps,
				EndpointPatch{endpointInfo: &endpointInfo, servicePortInfo: patchMap[svcKey].svc.servicePortInfo, op: Delete})
			patchMap[svcKey] = patch
		} else {
			// this only happens if we are removing endpoints to a pre-existing service;
			// if service is intact, diffstore won't return it in Updated() or Deleted() call and patch won't exist in patch map
			// lookup the service from the service store
			serviceResults := c.svcStore.GetByPrefix([]byte(endpointInfo.svcKey))[0]
			servicePortInfo := serviceResults.Value.(ServicePortInfo)

			// create new patch; add service with no-op operation; initialise endpoints patch with
			// endpoint and delete operation
			patchMap[endpointInfo.svcKey] = ServiceEndpointsPatch{
				svc: ServicePatch{servicePortInfo: &servicePortInfo, op: NoOp},
				eps: []EndpointPatch{{endpointInfo: &endpointInfo, servicePortInfo: &servicePortInfo, op: Delete}},
			}
		}

		// delete endpoint from endpoint diffstore
		c.epStore.Delete([]byte(epKey))

	}
	/////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

	// return all the patches
	// we don't need the keys of the patch map
	// only values are required to execute the changes at network layer
	patches := make([]ServiceEndpointsPatch, 0)
	for _, patch := range patchMap {
		patches = append(patches, patch)
	}
	return patches
}
