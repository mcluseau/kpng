package ipvsfullsate

import (
	"fmt"
	"github.com/cespare/xxhash"

	v1 "k8s.io/api/core/v1"
	netutils "k8s.io/utils/net"
	"net"
	"sigs.k8s.io/kpng/api/localv1"
	"strconv"
)

const (
	// FlagPersistent specify IPVS service session affinity
	FlagPersistent = 0x1
	// FlagHashed specify IPVS service hash flag
	FlagHashed = 0x2
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

// getClusterIPByFamily returns a service clusterIP by family
func getClusterIPByFamily(ipFamily v1.IPFamily, service *localv1.Service) string {
	if ipFamily == v1.IPv4Protocol {
		if len(service.IPs.ClusterIPs.V4) > 0 {
			return service.IPs.ClusterIPs.V4[0]
		}
	}
	if ipFamily == v1.IPv6Protocol {
		if len(service.IPs.ClusterIPs.V6) > 0 {
			return service.IPs.ClusterIPs.V6[0]
		}
	}
	return ""
}

// getLoadBalancerIPByFamily returns a service clusterIP by family
func getLoadBalancerIPByFamily(ipFamily v1.IPFamily, service *localv1.Service) string {
	if service.IPs.LoadBalancerIPs != nil {
		if ipFamily == v1.IPv4Protocol {
			if len(service.IPs.LoadBalancerIPs.V4) > 0 {
				return service.IPs.LoadBalancerIPs.V4[0]
			}
		}
		if ipFamily == v1.IPv6Protocol {
			if len(service.IPs.LoadBalancerIPs.V6) > 0 {
				return service.IPs.LoadBalancerIPs.V6[0]
			}
		}
	}
	return ""
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
