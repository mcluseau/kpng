# Initialization

## Kernel Parameters
https://www.kernel.org/doc/html/v5.14/networking/ipvs-sysctl.html

| Parameter                             |                                                                  Description                                                                   | Value |
|---------------------------------------|:----------------------------------------------------------------------------------------------------------------------------------------------:|------:|
| net.bridge.bridge-nf-call-iptables    |                                        Determine whether packets crossing a bridge are sent to iptables                                        |     1 |
| net.ipv4.vs.conntrack                 |                      Determines whether to maintain connectiion tracking entries for connections handled by load balancer                      |     1 |
| net.ipv4.vs.expire_nodest_conn        | Determine whether load balancer will close/expire the connection immediately when a packet arrives and its destination server is not available |     1 |
| net.ipv4.vs.expire_quiescent_template |                     Determine whether load balancer will expire persistent templates when destination server is quiescent                      |     1 |
| net.ipv4.vs.conn_reuses_mode          |                             Determine whether to enable connection meant for other destinations will be forwarded                              |     0 |

## Kernel Version > 4.1

## Dummy Interface [kube-ipvs0]

## IPSets and IPTables

IPVS handles load balancing but can't take care of other workarounds in Kube Proxy such as packet filtering, hairpin
masquerading etc.

IPVS Proxier leverage IPTables to achieve these workarounds, and proxier implements IPSets to prevent bombarding
IPTables with all the rules.

### IPSets

[*Information on IPSet Types](https://ipset.netfilter.org/ipset.man.html#SET%20TYPES:~:text=foo%20hash%3Aip%20forceadd-,SET%20TYPES,-bitmap%3Aip)

| Name                           |                        Description                        |             Type |
|--------------------------------|:---------------------------------------------------------:|-----------------:|
| KUBE-LOOP-BACK                 |          Endpoint IPs if locally present on node          |  hash:ip,port,ip |
| KUBE-CLUSTER-IP                |                        Cluster IPs                        |     hash:ip,port |
| KUBE-EXTERNAL-IP               |                       External IPs                        |     hash:ip,port |
| KUBE-EXTERNAL-IP-LOCAL         |                            ??                             |     hash:ip,port |
| KUBE-LOAD-BALANCER             |                 LoadBalancer Ingress IPs                  |     hash:ip,port |
| KUBE-LOAD-BALANCER-LOCAL       |  LoadBalancer Ingress IPs (externalTrafficPolicy=local)   |     hash:ip,port | 
| KUBE-LOAD-BALANCER-FW          |        LoadBalancer Ingress IPs with  SourceRanges        |     hash:ip,port |
| KUBE-LOAD-BALANCER-SOURCE-IP   |                            ??                             |  hash:ip,port,ip |
| KUBE-LOAD-BALANCER-SOURCE-CIDR |        LoadBalancer Ingress IPs with Source CIDRs         | hash:ip,port,net |
| KUBE-NODE-PORT-TCP             |                TCP NodePort Service Ports                 |      bitmap:port |
| KUBE-NODE-PORT-LOCAL-TCP       | TCP NodePort Service Ports (externalTrafficPolicy=local)  |      bitmap:port |
| KUBE-NODE-PORT-UDP             |                UDP NodePort Service Ports                 |      bitmap:port |
| KUBE-NODE-PORT-LOCAL-UDP       | UDP NodePort Service Ports (externalTrafficPolicy=local)  |      bitmap:port |
| KUBE-NODE-PORT-SCTP-HASH       |                SCTP NodePort Service Ports                |     hash:ip,port |
| KUBE-NODE-PORT-LOCAL-SCTP-HASH | SCTP NodePort Service Ports (externalTrafficPolicy=local) |     hash:ip,port |
| KUBE-HEALTH-CHECK-NODE-PORT    |                            ??                             |      bitmap:port |

### IPTable rules with IPSets

| IPSET                          |           Chain           |             Target |                                                                   Purpose | 
|--------------------------------|:-------------------------:|-------------------:|--------------------------------------------------------------------------:|
| KUBE-LOOP-BACK                 |     KUBE-POSTROUTING      |         MASQUERADE |                                    masquerade for resolving hairpin issue |
| KUBE-CLUSTER-IP                |       KUBE-SERVICES       |     KUBE-MARK-MASQ |                             masquerade for cases that masquerade-all=true |
| KUBE-EXTERNAL-IP               |       KUBE-SERVICES       |     KUBE-MARK-MASQ |                                    masquerade for packets to external IPs |
| KUBE-EXTERNAL-IP-LOCAL         |            ??             |                 ?? |                                                                        ?? |
| KUBE-LOAD-BALANCER             |       KUBE-SERVICES       | KUBE-LOAD-BALANCER |                      masquerade for packets to Load Balancer type service |
| KUBE-LOAD-BALANCER-LOCAL       |    KUBE-LOAD-BALANCER     |             RETURN |          accept packets to Load Balancer with externalTrafficPolicy=local |
| KUBE-LOAD-BALANCER-FW          |    KUBE-LOAD-BALANCER     |      KUBE-FIREWALL |    drop packets for LoadBalancer type Service with Source Range specified |
| KUBE-LOAD-BALANCER-SOURCE-IP   |       KUBE-FIREWALL       |             RETURN |                                                                        ?? |
| KUBE-LOAD-BALANCER-SOURCE-CIDR |       KUBE-FIREWALL       |             RETURN |  accept packets for Load Balancer type Service with Source CIDR specified |
| KUBE-NODE-PORT-TCP             |    KUBE-NODE-PORT-TCP     |     KUBE-MARK-MASQ |                                   masquerade for packets to NodePort(TCP) |
| KUBE-NODE-PORT-LOCAL-TCP       | KUBE-NODE-PORT-TCP-LOCAL  |     KUBE-MARK-MASQ |  accept packets to NodePort Service(TCP) with externalTrafficPolicy=local |
| KUBE-NODE-PORT-UDP             |    KUBE-NODE-PORT-UDP     |     KUBE-MARK-MASQ |                                   masquerade for packets to NodePort(UDP) |
| KUBE-NODE-PORT-LOCAL-UDP       | KUBE-NODE-PORT-UDP-LOCAL  |     KUBE-MARK-MASQ |  accept packets to NodePort Service(UDP) with externalTrafficPolicy=local |
| KUBE-NODE-PORT-SCTP-HASH       |    KUBE-NODE-PORT-SCTP    |     KUBE-MARK-MASQ |                                  masquerade for packets to NodePort(SCTP) |
| KUBE-NODE-PORT-LOCAL-SCTP-HASH | KUBE-NODE-PORT-SCTP-LOCAL |     KUBE-MARK-MASQ | accept packets to NodePort Service(SCTP) with externalTrafficPolicy=local |
| KUBE-HEALTH-CHECK-NODE-PORT    |            ??             |                 ?? |                                                                        ?? |

# Handling

## ClusterIP

### CreateService

1. Create IPVS virtual server for ClusterIP
2. Add ClusterIP to kubeClusterIPSet
3. Add ClusterIP to KubeIPVS interface
4. Create IPVS virtual server for ExternalIP if exists
5. Add ExternalIP to kubeExternalIPSet

### CreateEndpoint

1. Add EndpointIP(real server) to IPVS virtual server for ClusterIP
2. Add EndpointIP to kubeLoopBackIPSet if endpoint is local
3. Add EndpointIP(real server) to IPVS virtual server for ExternalIP

### DeleteService

1. Remove ClusterIP from KubeIPVS interface
2. Remove ClusterIP from kubeClusterIPSet
3. Delete IPVS virtual server for ClusterIP
4. Remove ClusterIP from kubeExternalIPSet 
5. Delete IPVS virtual server for ExternalIP

### DeleteEndpoint
1. Remove EndpointIP(real server) from IPVS virtual server for ExternalIP
2. Remove EndpointIP from kubeLoopBackIPSet if endpoint is local
3. Remove EndpointIP(real server) from IPVS virtual server for ClusterIP

## NodePort

### CreateService

1. Create IPVS virtual server for ClusterIP
2. Add ClusterIP to kubeClusterIPSet
3. Add ClusterIP to KubeIPVS interface
4. Create IPVS virtual server for NodeIPs 
5. Add NodePort to KubeNodePortTCPSet or KubeNodePortUDPSet in case of TCP and UDP protocol respectively;
   Add NodeIP and NodePort to KubeNodePortSCTPSet in case of SCTP protocol

### CreateEndpoint

1. Add EndpointIP(real server) to IPVS virtual server for ClusterIP
2. Add EndpointIP(real server) to IPVS virtual server for NodeIPs
3. Add EndpointIP to kubeLoopBackIPSet if endpoint is local

### DeleteService

1. Remove NodePort from KubeNodePortTCPSet or KubeNodePortUDPSet in case of TCP and UDP protocol respectively;
   Remove NodeIP and NodePort from KubeNodePortSCTPSet in case of SCTP protocol
2. Delete IPVS virtual server for NodeIPs
3. Remove ClusterIP from KubeIPVS interface
4. Remove ClusterIP from kubeClusterIPSet
5. Delete IPVS virtual server for ClusterIP

### DeleteEndpoint

1. Remove EndpointIP from kubeLoopBackIPSet if endpoint is local
2. Remove EndpointIP(real server) to IPVS virtual server for NodeIPs
3. Remove EndpointIP(real server) from IPVS virtual server for ClusterIP
