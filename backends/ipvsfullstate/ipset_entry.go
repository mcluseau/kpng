package ipvsfullsate

import (
	ipsetutil "sigs.k8s.io/kpng/backends/ipvsfullstate/util"
	"strings"
)

// functions in this file creates entries for IPSet which can be directly passed to proxier which adds/deletes them in actual ipsets
// these functions will be called by the objects which implement Handler interface only

func getIPSetEntryForClusterIP(srcAddr string, serviceInfo *ServiceInfo) *ipsetutil.Entry {
	if srcAddr != "" {
		return &ipsetutil.Entry{
			IP:       serviceInfo.ServiceIP(),
			Port:     int(serviceInfo.Port()),
			Protocol: strings.ToLower(serviceInfo.Protocol().String()),
			SetType:  ipsetutil.HashIPPort,
			Net:      srcAddr,
		}
	}
	return &ipsetutil.Entry{
		IP:       serviceInfo.ServiceIP(),
		Port:     int(serviceInfo.Port()),
		Protocol: strings.ToLower(serviceInfo.Protocol().String()),
		SetType:  ipsetutil.HashIPPort,
	}
}

func getIPSetEntryForEndPoint(endpointInfo *EndpointInfo, serviceInfo *ServiceInfo) *ipsetutil.Entry {
	return &ipsetutil.Entry{
		IP:       endpointInfo.IP,
		Port:     int(serviceInfo.TargetPort()),
		Protocol: strings.ToLower(serviceInfo.Protocol().String()),
		IP2:      endpointInfo.IP,
		SetType:  ipsetutil.HashIPPortIP,
	}
}

func getIPSetEntryForNodePort(serviceInfo *ServiceInfo) *ipsetutil.Entry {
	return &ipsetutil.Entry{
		Port:     int(serviceInfo.NodePort()),
		Protocol: strings.ToLower(serviceInfo.Protocol().String()),
		SetType:  ipsetutil.BitmapPort,
	}
}

func getIPSetEntryForNodePortSCTP(serviceInfo *ServiceInfo) *ipsetutil.Entry {
	return &ipsetutil.Entry{
		Port:     int(serviceInfo.NodePort()),
		Protocol: strings.ToLower(serviceInfo.Protocol().String()),
		SetType:  ipsetutil.HashIPPort,
	}
}
