/*
Copyright 2021 The Kubernetes Authors.

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
	"bytes"
	"github.com/google/seesaw/ipvs"
	"net"
	"strings"

	"github.com/vishvananda/netlink"
	v1 "k8s.io/api/core/v1"

	"k8s.io/klog/v2"
	"sigs.k8s.io/kpng/api/localv1"
	iptablesutil "sigs.k8s.io/kpng/backends/iptables/util"
	"sigs.k8s.io/kpng/backends/ipvsfullstate/exec"
	"sigs.k8s.io/kpng/backends/ipvsfullstate/util"
	ipsetutil "sigs.k8s.io/kpng/backends/ipvsfullstate/util"
)

var protocolIPSetMap = map[string]string{
	ipsetutil.ProtocolTCP:  kubeNodePortSetTCP,
	ipsetutil.ProtocolUDP:  kubeNodePortSetUDP,
	ipsetutil.ProtocolSCTP: kubeNodePortSetSCTP,
}

type proxier struct {
	ipFamily v1.IPFamily

	dryRun           bool
	nodeAddresses    []string
	schedulingMethod string
	weight           int32
	masqueradeMark   string
	masqueradeAll    bool

	dummy netlink.Link

	iptables util.IPTableInterface
	ipset    util.Interface
	exec     exec.Interface
	//ipvs           util.Interface
	//localDetector  proxyutiliptables.LocalTrafficDetector
	//portMapper     netutils.PortOpener
	//recorder       events.EventRecorder
	//serviceHealthServer healthcheck.ServiceHealthServer
	//healthzServer       healthcheck.ProxierHealthUpdater

	ipsetList map[string]*IPSet
	//servicePortMap map[string]map[string]*ServicePortInfo
	portMap map[string]map[string]localv1.PortMapping
	// The following buffers are used to reuse memory and avoid allocations
	// that are significantly impacting performance.
	iptablesData     *bytes.Buffer
	filterChainsData *bytes.Buffer
	natChains        iptablesutil.LineBuffer
	filterChains     iptablesutil.LineBuffer
	natRules         iptablesutil.LineBuffer
	filterRules      iptablesutil.LineBuffer
}

func NewProxier(ipFamily v1.IPFamily,
	dummy netlink.Link,
	ipsetInterface util.Interface,
	iptInterface util.IPTableInterface,
	nodeIPs []string,
	schedulingMethod, masqueradeMark string,
	masqueradeAll bool,
	weight int32) *proxier {
	return &proxier{
		ipFamily:         ipFamily,
		dummy:            dummy,
		nodeAddresses:    nodeIPs,
		schedulingMethod: schedulingMethod,
		weight:           weight,
		ipset:            ipsetInterface,
		iptables:         iptInterface,
		masqueradeMark:   masqueradeMark,
		masqueradeAll:    masqueradeAll,
		ipsetList:        make(map[string]*IPSet),
		portMap:          make(map[string]map[string]localv1.PortMapping),
		iptablesData:     bytes.NewBuffer(nil),
		filterChainsData: bytes.NewBuffer(nil),
		natChains:        iptablesutil.LineBuffer{},
		natRules:         iptablesutil.LineBuffer{},
		filterChains:     iptablesutil.LineBuffer{},
		filterRules:      iptablesutil.LineBuffer{},
	}
}

func (p *proxier) initializeIPSets() {
	// initialize ipsetList with all sets we needed
	for _, is := range ipsetInfo {
		p.ipsetList[is.name] = newIPSet(p.ipset, is.name, is.setType, p.ipFamily, is.comment)
	}

	// make sure ip sets exists in the system.
	for _, set := range p.ipsetList {
		if err := ensureIPSet(set); err != nil {
			return
		}
	}
}

func (p *proxier) addIPToIPVSInterface(serviceIP string) {
	ipFamily := getIPFamily(serviceIP)

	ip := asDummyIPs(serviceIP, ipFamily)

	_, ipNet, err := net.ParseCIDR(ip)
	if err != nil {
		klog.Error("failed to parse ip/net %q: %v", ip, err)
		return
	}

	if p.dummy == nil {
		klog.Error("exit early while adding dummy ip ", ip, "; dummy link device not found")
		return
	}
	klog.V(2).Info("adding dummy ip ", ip)
	if err = netlink.AddrAdd(p.dummy, &netlink.Addr{IPNet: ipNet}); err != nil {
		klog.Error("failed to add dummy ip ", ip, ": ", err)
	}
}

func (p *proxier) removeIPFromIPVSInterface(serviceIP string) {

	ipFamily := getIPFamily(serviceIP)

	ip := asDummyIPs(serviceIP, ipFamily)

	_, ipNet, err := net.ParseCIDR(ip)
	if err != nil {
		klog.Error("failed to parse ip/net %q: %v", ip, err)
		return
	}

	if p.dummy == nil {
		klog.Error("exit early while deleting dummy IP ", ip, "; dummy link device not found")
		return
	}

	klog.V(2).Info("deleting dummy IP ", ip)
	if err = netlink.AddrDel(p.dummy, &netlink.Addr{IPNet: ipNet}); err != nil {
		klog.Error("failed to delete dummy IP ", ip, ": ", err)
	}
}

func (p *proxier) createVirtualServer(servicePortInfo *ServicePortInfo) {
	vs := servicePortInfo.GetVirtualServer()

	klog.V(2).Infof("adding AddVirtualServer: port: %v", servicePortInfo)
	// Programme virtual-server directly
	ipvsSvc := vs.ToService()
	err := ipvs.AddService(ipvsSvc)
	if err != nil && !strings.HasSuffix(err.Error(), "object exists") {
		klog.Error("failed to add service in IPVS", ": ", err)
	}
}

func (p *proxier) deleteVirtualServer(servicePortInfo *ServicePortInfo) {
	klog.V(2).Infof("deleting service , IP (%v) , port (%v)", servicePortInfo.GetIP(), servicePortInfo.Port())
	err := ipvs.DeleteService(servicePortInfo.GetVirtualServer().ToService())
	if err != nil {
		klog.Error("failed to delete service from IPVS", servicePortInfo.GetIP(), ": ", err)
	}
}

func (p *proxier) addEntryInIPSet(entry *ipsetutil.Entry, set *IPSet) {
	if valid := set.validateEntry(entry); !valid {
		klog.Errorf("error adding entry to ipset. entry:%s, ipset:%s", entry.String(), set.Name)
		return
	}

	if err := set.handle.AddEntry(entry.String(), &set.IPSet, true); err != nil {
		klog.Errorf("Failed to add entry %v into ip set: %s, error: %v", entry, set.Name, err)
	} else {
		klog.V(3).Infof("Successfully add entry: %v into ip set: %s", entry, set.Name)
	}

}

func (p *proxier) removeEntryFromIPSet(entry *ipsetutil.Entry, set *IPSet) {
	if valid := set.validateEntry(entry); !valid {
		klog.Errorf("error adding entry to ipset. entry:%s, ipset:%s", entry.String(), set.Name)
		return
	}

	if err := set.handle.DelEntry(entry.String(), set.Name); err != nil {
		klog.Errorf("failed to remove entry %v from ipset: %s, error: %v", entry, set.Name, err)
	} else {
		klog.V(3).Infof("successfully removed entry: %v from ipset: %s", entry, set.Name)
	}

}

func (p *proxier) addRealServer(servicePortInfo *ServicePortInfo, endpointInfo *EndpointInfo) {
	destination := ipvsSvcDst{
		Svc: servicePortInfo.GetVirtualServer().ToService(),
		Dst: ipvsDestination(*endpointInfo, servicePortInfo),
	}
	klog.V(2).Infof("adding destination ep (%v)", endpointInfo.GetIP())
	if err := ipvs.AddDestination(destination.Svc, destination.Dst); err != nil && !strings.HasSuffix(err.Error(), "object exists") {
		klog.Error("failed to add destination : ", err)
	}
}

func (p *proxier) deleteRealServer(baseServicePort *ServicePortInfo, endpointInfo *EndpointInfo) {

	vs := baseServicePort.GetVirtualServer()
	klog.V(2).Infof("deleteRealServer, portInfo : %v", endpointInfo)
	dest := ipvsSvcDst{
		Svc: vs.ToService(),
		Dst: ipvsDestination(*endpointInfo, baseServicePort),
	}

	klog.V(2).Infof("deleting destination : %v", dest)
	if err := ipvs.DeleteDestination(dest.Svc, dest.Dst); err != nil {
		klog.Error("failed to delete destination ", dest, ": ", err)
	}

}
