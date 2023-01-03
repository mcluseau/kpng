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
	"k8s.io/klog/v2"
	"math/rand"
	"os"
	"sigs.k8s.io/kpng/api/localv1"
	"sigs.k8s.io/kpng/client"
	"time"
)

const NCallbacks int = 100
const NNodes int = 20

const MinServices int = 10
const MaxServices int = 300

const MinEndpoints int = 10
const MaxEndpoints int = 300

var Protocols = []localv1.Protocol{localv1.Protocol_TCP, localv1.Protocol_UDP, localv1.Protocol_SCTP}
var Ports = []int32{80, 3000, 5000, 8080}

var StressServiceTypes = []ServiceType{ClusterIPService, NodePortService}

func generateRandomPortMappings() []*localv1.PortMapping {
	portMaps := make([]*localv1.PortMapping, 0)

	rand.Shuffle(len(Ports), func(i, j int) { Ports[i], Ports[j] = Ports[j], Ports[i] })

	for i, p := range Ports[:rand.Intn(len(Ports)-1)] {
		portMaps = append(portMaps, &localv1.PortMapping{
			Name:           fmt.Sprintf("port-%d", i+1),
			Protocol:       Protocols[rand.Intn(len(Protocols)-1)],
			Port:           p,
			NodePort:       int32(rand.Intn(32767-30000) + 30000),
			TargetPort:     p,
			TargetPortName: fmt.Sprintf("target-port-%d", i+1),
		})
	}

	return portMaps
}

func generateRandomLabels(i int) map[string]string {
	labels := make(map[string]string)

	labels["svc"] = fmt.Sprintf("svc-%d", i)

	for l := 0; l < 10; l++ {
		labels[fmt.Sprintf("key-%d", l)] = fmt.Sprintf("value-%d", l)
	}

	return labels
}

func getHostNameLocalVals() (string, bool) {
	hostname, _ := os.Hostname()
	if rand.Float64() < 1.0/float64(NNodes) {
		return hostname, true
	} else {
		return fmt.Sprintf("host-%d", rand.Intn(NNodes)), false
	}
}
func generateRandomEndpoints(s int) []*localv1.Endpoint {
	eps := make([]*localv1.Endpoint, 0)

	for e := 0; e < rand.Intn(MaxEndpoints-MinEndpoints)+MinEndpoints; e++ {
		hostname, local := getHostNameLocalVals()
		eps = append(eps, &localv1.Endpoint{
			Hostname: hostname,
			IPs:      &localv1.IPSet{V4: []string{fmt.Sprintf("10.%d.%d.%d", s%255, rand.Intn(255), rand.Intn(255))}},
			Local:    local,
		})
	}

	return eps

}

func generateRandomFullstate(fullstate chan *client.ServiceEndpoints) {
	defer close(fullstate)

	for s := 0; s < rand.Intn(MaxServices-MinServices)+MinServices; s++ {
		fullstate <- &client.ServiceEndpoints{
			Service: &localv1.Service{
				Name:      fmt.Sprintf("svc-%d", s),
				Namespace: "stress",
				Type:      StressServiceTypes[rand.Intn(len(StressServiceTypes)-1)].String(),
				Labels:    generateRandomLabels(s),
				IPs: &localv1.ServiceIPs{
					ClusterIPs:      &localv1.IPSet{V4: []string{fmt.Sprintf("172.18.%d.%d", s/255, s%255)}},
					ExternalIPs:     nil,
					LoadBalancerIPs: nil,
					Headless:        false,
				},
				Ports: generateRandomPortMappings(),
			},
			Endpoints: generateRandomEndpoints(s),
		}
	}

}

func (c *IpvsController) bombardSink() {

	st := time.Now()
	for i := 0; i < NCallbacks; i++ {
		// setting rand.Seed to get same set of values for every bombardSink call
		rand.Seed(int64(i))

		klog.Infof("callback %d", i)
		seps := make(chan *client.ServiceEndpoints, 1)
		go generateRandomFullstate(seps)
		c.Callback(seps)
	}
	et := time.Now()

	klog.Info("\n############################################\n")
	klog.Infof("Took %d ms to process everything", et.Sub(st).Milliseconds())
	klog.Info("\n############################################\n")

	time.Sleep(time.Hour)
}
