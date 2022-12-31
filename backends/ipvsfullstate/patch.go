package ipvsfullsate

// ServicePatch -> ServiceInfo and Operation
type ServicePatch struct {
	serviceInfo *ServiceInfo
	op          Operation
}

// apply will invoke the handler which interacts with proxier to implement network rules
// low level networking logic is injected as a dependency. see Handler interface
func (p *ServicePatch) apply(handler map[Operation]func(serviceInfo *ServiceInfo)) {
	handler[p.op](p.serviceInfo)
}

// EndpointPatch -> EndpointInfo, ServiceInfo, and Operation
type EndpointPatch struct {
	endpointInfo *EndpointInfo
	serviceInfo  *ServiceInfo
	op           Operation
}

// apply will invoke the handler which interacts with proxier to implement network rules
// low level networking logic is injected as a dependency. see Handler interface
func (p *EndpointPatch) apply(handler map[Operation]func(endpointInfo *EndpointInfo, serviceInfo *ServiceInfo)) {
	handler[p.op](p.endpointInfo, p.serviceInfo)
}

// EndpointPatches -> [] EndpointPatch
type EndpointPatches []EndpointPatch

// apply will call apply on each EndpointPatch
func (e EndpointPatches) apply(handler map[Operation]func(*EndpointInfo, *ServiceInfo)) {
	for _, patch := range e {
		patch.apply(handler)
	}
}

// PatchGroup is a collection of ServicePatch and EndpointPatches. On application, it will complete the transition
// of a fullstate.ServiceEndpoints from state A -> state B on underlying network layer
// PatchGroup gives control on order of execution of mutually inclusive patches (create/delete service first or endpoints first?),
// and it also opens possibilities for concurrent executions and rollbacks in the future.
type PatchGroup struct {
	svc ServicePatch
	eps EndpointPatches
}

// apply will apply ServicePatch and EndpointPatches in the order which we want
// networking logic is passed as a dependency
func (p *PatchGroup) apply(
	serviceHandler map[Operation]func(*ServiceInfo),
	endpointHandler map[Operation]func(*EndpointInfo, *ServiceInfo),
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
	//	TODO
	case Delete:
		// first endpoints; then service
		p.eps.apply(endpointHandler)
		p.svc.apply(serviceHandler)
	}
}

// generatePatchGroups prepares patch groups for the network layer using diffstore deltas
func (c *IpvsController) generatePatchGroups() []PatchGroup {
	// patchGroupMap <service_key:PatchGroup> help to combine service and endpoints together into a PatchGroup
	patchGroupMap := make(map[string]PatchGroup)

	//////////////////////////////////////////////// Service Store - Updates //////////////////////////////////////////////////
	for _, KV := range c.svcStore.Updated() {
		svcKey := string(KV.Key)
		serviceInfo := KV.Value.(ServiceInfo)

		// isNew flag was added at the time of callback; refer c.Callback
		if serviceInfo.isNew {
			// create new patch group; add service with create operation; initialise endpoints patch
			patchGroupMap[svcKey] = PatchGroup{
				ServicePatch{serviceInfo: &serviceInfo, op: Create},
				make([]EndpointPatch, 0),
			}
		} else {
			// create new patch group; add service with update operation; initialise endpoints patch
			patchGroupMap[svcKey] = PatchGroup{
				ServicePatch{serviceInfo: &serviceInfo, op: Update},
				make([]EndpointPatch, 0),
			}
		}
	}
	///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

	//////////////////////////////////////////////// Service Store - Deletes //////////////////////////////////////////////////
	for _, KV := range c.svcStore.Deleted() {
		svcKey := string(KV.Key)
		servicePortInfo := KV.Value.(ServiceInfo)

		// create new patch group; add service with delete operation; initialise endpoints patch
		patchGroupMap[svcKey] = PatchGroup{
			ServicePatch{serviceInfo: &servicePortInfo, op: Delete},
			make([]EndpointPatch, 0),
		}

		// delete the service from service diffstore
		c.svcStore.Delete([]byte(svcKey))
	}
	/////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

	////////////////////////////////////////////////// Endpoint Store - Updates /////////////////////////////////////////////////
	for _, KV := range c.epStore.Updated() {
		endpointInfo := KV.Value.(EndpointInfo)
		svcKey := endpointInfo.svcKey

		// check if patch group exists for endpoint's service
		if group, ok := patchGroupMap[svcKey]; ok {
			// isNew flag was added at the time of callback; refer c.Callback
			if endpointInfo.isNew {
				// append endpoint entry to patch group endpoints with create operation
				group.eps = append(patchGroupMap[svcKey].eps,
					EndpointPatch{endpointInfo: &endpointInfo, serviceInfo: patchGroupMap[svcKey].svc.serviceInfo, op: Create})
			} else {
				// append endpoint entry to patch group endpoints with update operation
				group.eps = append(patchGroupMap[svcKey].eps,
					EndpointPatch{endpointInfo: &endpointInfo, serviceInfo: patchGroupMap[svcKey].svc.serviceInfo, op: Update})
			}
			patchGroupMap[svcKey] = group

		} else {
			// this handles the cases when only endpoints are changed (created/updated/deleted) and service remains as it is
			// lookup the service from the service store; and add a NoOp entry
			serviceResults := c.svcStore.GetByPrefix([]byte(endpointInfo.svcKey))[0]
			serviceInfo := serviceResults.Value.(ServiceInfo)

			if endpointInfo.isNew {
				// create new patch group; add service with no-op operation; initialise endpoints patch with
				// endpoint entry and create operation
				patchGroupMap[endpointInfo.svcKey] = PatchGroup{
					svc: ServicePatch{serviceInfo: &serviceInfo, op: NoOp},
					eps: []EndpointPatch{{endpointInfo: &endpointInfo, serviceInfo: &serviceInfo, op: Create}},
				}
			} else {
				// create new patch group; add service with no-op operation; initialise endpoints patch with
				// endpoint entry and update operation
				patchGroupMap[endpointInfo.svcKey] = PatchGroup{
					svc: ServicePatch{serviceInfo: &serviceInfo, op: NoOp},
					eps: []EndpointPatch{{endpointInfo: &endpointInfo, serviceInfo: &serviceInfo, op: Update}},
				}
			}

		}
	}
	/////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

	////////////////////////////////////////////////// Endpoint Store - Deletes /////////////////////////////////////////////////
	for _, KV := range c.epStore.Deleted() {
		endpointInfo := KV.Value.(EndpointInfo)
		epKey := string(KV.Key)
		svcKey := endpointInfo.svcKey

		// check if patch group exists for endpoint's service
		if group, ok := patchGroupMap[svcKey]; ok {
			// append endpoint entry to patch group endpoints with delete operation
			group.eps = append(patchGroupMap[svcKey].eps,
				EndpointPatch{endpointInfo: &endpointInfo, serviceInfo: patchGroupMap[svcKey].svc.serviceInfo, op: Delete})
			patchGroupMap[svcKey] = group
		} else {
			// this handles the cases when only endpoints are changed (created/updated/deleted) and service remains as it is
			// lookup the service from the service store; and add a NoOp entry
			serviceResults := c.svcStore.GetByPrefix([]byte(endpointInfo.svcKey))[0]
			serviceInfo := serviceResults.Value.(ServiceInfo)

			// create new patch group; add service with no-op operation; initialise endpoints patch with
			// endpoint entry and delete operation
			patchGroupMap[endpointInfo.svcKey] = PatchGroup{
				svc: ServicePatch{serviceInfo: &serviceInfo, op: NoOp},
				eps: []EndpointPatch{{endpointInfo: &endpointInfo, serviceInfo: &serviceInfo, op: Delete}},
			}
		}

		// delete endpoint from endpoint diffstore
		c.epStore.Delete([]byte(epKey))

	}
	/////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

	// return all the patch groups
	groups := make([]PatchGroup, 0)
	for _, pg := range patchGroupMap {
		groups = append(groups, pg)
	}
	return groups
}
