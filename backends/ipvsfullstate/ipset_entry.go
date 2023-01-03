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
	ipsetutil "sigs.k8s.io/kpng/backends/ipvsfullstate/util"
	"strings"
)

// functions in this file creates entries for IPSet which can be directly passed to proxier which adds/deletes them in actual ipsets
// these functions will be called by the objects which implement Handler interface only

func getIPSetEntriesForClusterIP(srcAddr string, servicePortInfo *ServicePortInfo) []*ipsetutil.Entry {
	entries := make([]*ipsetutil.Entry, 0)
	for _, ip := range servicePortInfo.GetClusterIPs() {
		entries = append(entries, getIPSetEntryForIP(srcAddr, servicePortInfo, ip))
	}
	return entries
}

func getIPSetEntriesForExternalIPs(srcAddr string, servicePortInfo *ServicePortInfo) []*ipsetutil.Entry {
	entries := make([]*ipsetutil.Entry, 0)
	for _, ip := range servicePortInfo.GetExternalIPs() {
		entries = append(entries, getIPSetEntryForIP(srcAddr, servicePortInfo, ip))
	}
	return entries
}

func getIPSetEntryForIP(srcAddr string, servicePortInfo *ServicePortInfo, ip string) *ipsetutil.Entry {
	if srcAddr != "" {
		return &ipsetutil.Entry{
			IP:       ip,
			Port:     int(servicePortInfo.Port()),
			Protocol: strings.ToLower(servicePortInfo.Protocol().String()),
			SetType:  ipsetutil.HashIPPort,
			Net:      srcAddr,
		}
	}
	return &ipsetutil.Entry{
		IP:       ip,
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
