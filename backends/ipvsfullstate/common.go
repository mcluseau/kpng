package ipvsfullsate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/cespare/xxhash"
	v1 "k8s.io/api/core/v1"
	netutils "k8s.io/utils/net"
	"net"
	"sigs.k8s.io/kpng/api/localv1"
)

const (
	// FlagPersistent specify IPVS service session affinity
	FlagPersistent = 0x1
	// FlagHashed specify IPVS service hash flag
	FlagHashed = 0x2
)

const DiffstoreDelimiter = "||"

func getSvcKey(svcNamespacedName string, port int32, protocol localv1.Protocol) string {
	return fmt.Sprintf("%s%s%d%s%d",
		svcNamespacedName,
		DiffstoreDelimiter,
		port,
		DiffstoreDelimiter,
		protocol,
	)
}

func getEpKey(svcKey string, endpointIp string) string {
	return fmt.Sprintf("%s%s%s",
		svcKey,
		DiffstoreDelimiter,
		endpointIp,
	)
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

func getHashForDiffstore[Info ResourceInfo](info Info) uint64 {
	infoBytes := new(bytes.Buffer)
	_ = json.NewEncoder(infoBytes).Encode(info)
	return xxhash.Sum64(infoBytes.Bytes())
}
