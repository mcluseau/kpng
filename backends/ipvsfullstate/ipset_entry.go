package ipvsfullsate

import (
	ipsetutil "sigs.k8s.io/kpng/backends/ipvsfullstate/util"
	"strings"
)

// functions in this file creates entries for IPSet which can be directly passed to proxier which adds/deletes them in actual ipsets
// these functions will be called by the objects which implement Handler interface only

func getIPSetEntryForClusterIP(srcAddr string, servicePortInfo *ServicePortInfo) *ipsetutil.Entry {
	if srcAddr != "" {
		return &ipsetutil.Entry{
			IP:       servicePortInfo.GetIP(),
			Port:     int(servicePortInfo.Port()),
			Protocol: strings.ToLower(servicePortInfo.Protocol().String()),
			SetType:  ipsetutil.HashIPPort,
			Net:      srcAddr,
		}
	}
	return &ipsetutil.Entry{
		IP:       servicePortInfo.GetIP(),
		Port:     int(servicePortInfo.Port()),
		Protocol: strings.ToLower(servicePortInfo.Protocol().String()),
		SetType:  ipsetutil.HashIPPort,
	}
}

func getIPSetEntryForEndPoint(endpointInfo *EndpointInfo, servicePortInfo *ServicePortInfo) *ipsetutil.Entry {
	return &ipsetutil.Entry{
		IP:       endpointInfo.GetIP(),
		Port:     int(servicePortInfo.TargetPort()),
		Protocol: strings.ToLower(servicePortInfo.Protocol().String()),
		IP2:      endpointInfo.GetIP(),
		SetType:  ipsetutil.HashIPPortIP,
	}
}

func getIPSetEntryForNodePort(servicePortInfo *ServicePortInfo) *ipsetutil.Entry {
	return &ipsetutil.Entry{
		Port:     int(servicePortInfo.NodePort()),
		Protocol: strings.ToLower(servicePortInfo.Protocol().String()),
		SetType:  ipsetutil.BitmapPort,
	}
}

func getIPSetEntryForNodePortSCTP(servicePortInfo *ServicePortInfo) *ipsetutil.Entry {
	return &ipsetutil.Entry{
		Port:     int(servicePortInfo.NodePort()),
		Protocol: strings.ToLower(servicePortInfo.Protocol().String()),
		SetType:  ipsetutil.HashIPPort,
	}
}
