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
	"github.com/cespare/xxhash"
	IPVS "github.com/google/seesaw/ipvs"

	v1 "k8s.io/api/core/v1"
	netutils "k8s.io/utils/net"
	"net"
	"sigs.k8s.io/kpng/api/localv1"
	"strconv"
)

const (
	// FlagPersistent specify IPVS service session affinity
	FlagPersistent = IPVS.SFPersistent
	// FlagHashed specify IPVS service hash flag
	FlagHashed = IPVS.SFHashed
)

const Delimiter = "||"

func getSvcKey(servicePortInfo *ServicePortInfo) string {
	hash := getHashForDiffstore(servicePortInfo)
	return strconv.FormatUint(hash, 10)
}

func getEpKey(svcKey string, endpointIp string) string {
	return fmt.Sprintf("%s%s%s", svcKey, Delimiter, endpointIp)
}

func asDummyIPs(ip string, ipFamily v1.IPFamily) string {
	if ipFamily == v1.IPv4Protocol {
		return ip + "/32"
	}

	if ipFamily == v1.IPv6Protocol {
		return ip + "/128"
	}
	return ip + "/32"
}

func interfaceAddresses() []string {
	ifacesAddress, err := net.InterfaceAddrs()
	if err != nil {
		panic(err)
	}

	var addresses []string
	for _, addr := range ifacesAddress {
		// TODO: Ignore interfaces in PodCIDR or ClusterCIDR
		ip, _, err := net.ParseCIDR(addr.String())
		if err != nil {
			panic(err)
		}
		// I want to deal only with IPv4 right now...
		if ipv4 := ip.To4(); ipv4 == nil {
			continue
		}

		addresses = append(addresses, ip.String())
	}
	return addresses
}

func getIPFamily(ipAddr string) v1.IPFamily {
	var ipAddrFamily v1.IPFamily
	if netutils.IsIPv4String(ipAddr) {
		ipAddrFamily = v1.IPv4Protocol
	}

	if netutils.IsIPv6String(ipAddr) {
		ipAddrFamily = v1.IPv6Protocol
	}
	return ipAddrFamily
}

// getClusterIPsByFamily returns a service clusterIP by family
func getClusterIPsByFamily(ipFamily v1.IPFamily, service *localv1.Service) []string {
	if ipFamily == v1.IPv4Protocol {
		return service.IPs.ClusterIPs.V4
	}
	if ipFamily == v1.IPv6Protocol {
		return service.IPs.ClusterIPs.V6
	}
	return make([]string, 0)
}

// getLoadBalancerIPsByFamily returns a service clusterIP by family
func getLoadBalancerIPsByFamily(ipFamily v1.IPFamily, service *localv1.Service) []string {
	if service.IPs.LoadBalancerIPs != nil {
		if ipFamily == v1.IPv4Protocol {
			return service.IPs.LoadBalancerIPs.V4
		}
		if ipFamily == v1.IPv6Protocol {
			return service.IPs.LoadBalancerIPs.V6
		}
	}
	return make([]string, 0)
}

// getExternalIPsByFamily returns a service clusterIP by family
func getExternalIPsByFamily(ipFamily v1.IPFamily, service *localv1.Service) []string {
	if service.IPs.ExternalIPs != nil {
		if ipFamily == v1.IPv4Protocol {
			return service.IPs.ExternalIPs.V4
		}
		if ipFamily == v1.IPv6Protocol {
			return service.IPs.ExternalIPs.V6
		}
	}
	return make([]string, 0)
}

func getSessionAffinity(affinity interface{}) SessionAffinity {
	var sessionAffinity SessionAffinity
	switch affinity.(type) {
	case *localv1.Service_ClientIP:
		sessionAffinity.ClientIP = affinity.(*localv1.Service_ClientIP)
	}
	return sessionAffinity
}

// getIPFilterTargetIpsAndSourceRanges returns a service clusterIP by family
func getIPFilterTargetIpsAndSourceRanges(ipFamily v1.IPFamily, service *localv1.Service) ([]string, []string) {
	targetIps := make([]string, 0)
	sourceRanges := make([]string, 0)

	if len(service.IPFilters) > 0 {
		for _, filters := range service.IPFilters {
			if ipFamily == v1.IPv4Protocol {
				for _, ip := range filters.TargetIPs.V4 {
					targetIps = append(targetIps, ip)
				}
			} else {
				for _, ip := range filters.TargetIPs.V6 {
					targetIps = append(targetIps, ip)
				}
			}
			sourceRanges = append(sourceRanges, filters.SourceRanges...)
		}

	}
	return targetIps, sourceRanges
}
func getHashForDiffstore(info ResourceInfo) uint64 {
	return xxhash.Sum64(info.ToBytes())
}
