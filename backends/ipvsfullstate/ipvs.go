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

	// service store for storing ServicePortInfo object in diffstore
	svcStore *lightdiffstore.DiffStore

	// endpoint store for storing EndpointInfo  object in diffstore
	epStore *lightdiffstore.DiffStore

	iptables util.IPTableInterface
	ipset    util.Interface
	exec     exec.Interface
	proxier  *proxier

	// Create + Update + Delete handler for respective ServiceType
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

	ipv4Proxier.initializeIPSets()
	ipv4Proxier.setIPTableRulesForIPVS()

	// service handlers
	handlers := make(map[ServiceType]Handler)
	handlers[ClusterIPService] = newClusterIPHandler(ipv4Proxier)
	handlers[NodePortService] = newNodePortHandler(ipv4Proxier)
	// TODO - add handler for LoadBalancer serviceType
	// handlers[LoadBalancerService] = newLoadBalancerHandler(ipv4Proxier)

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

			// generate service; key format: [namespace + name + port + protocol]
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
	patchGroups := c.generatePatchGroups()

	// apply patches
	c.apply(patchGroups)

	et := time.Now()
	klog.V(3).Infof("took %.2f ms to sync", 1000*et.Sub(st).Seconds())
}

func (c *IpvsController) apply(groups []PatchGroup) {
	// TODO get rid of this
	// delete ops should precede create ops
	sort.Slice(groups, func(i, j int) bool {
		return groups[i].svc.op > groups[j].svc.op
	})

	for _, group := range groups {
		klog.V(3).Infof("patch group: service=%s; %d; endpoints=%d", group.svc.servicePortInfo.NamespacedName(), group.svc.op, len(group.eps))
		// get handler for the serviceType
		handler, ok := c.handlers[group.svc.servicePortInfo.serviceType]
		if ok {
			// apply the patch
			group.apply(handler.getServiceHandlers(), handler.getEndpointHandlers())
		} else {
			klog.V(3).Infof("IPVS fullstate not yet implemented for %v", group.svc.servicePortInfo.serviceType)
		}
	}
}
