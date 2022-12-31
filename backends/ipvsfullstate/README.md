# IPVS Fullstate Implementation
This implementation roughly follows [bridge design pattern](https://en.wikipedia.org/wiki/Bridge_pattern).  **IPVSController** acts as an abstraction and **Handler** acts as implementer.  
This implementation can be broken down into three major steps.

## 1. Registration
- **reister.go**

  ```init()``` registers backend with the brain

  ```Sink()``` implements the full state sink

  ```Setup()``` initializes required components - proxier, ipvs and dummy interfaces

- **setup.go**

  performs sanity checks and prepares kernel for IPVS implementation

- **ipvs.go**
  ```go
  type IpvsController struct {
      mu sync.Mutex
  
      ipFamily v1.IPFamily
      
      // service store for storing ServiceInfo object to diffstore
      svcStore *lightdiffstore.DiffStore
      // endpoint store for storing (ServiceInfo + EndpointInfo) object to diffstore
      epStore *lightdiffstore.DiffStore
  
      iptables util.IPTableInterface
      ipset    util.Interface
      exec     exec.Interface
      proxier  *proxier
    
      handlers map[ServiceType]Handler
  }
  ```
  ```IpvsController``` takes care of high order of things - **what needs to be done** after receiving full state callbacks.

- **proxier.go**
  
  **proxier** directly interacts with iptables, ipvs and ipsets.
  
  **proxier** has no business logic and acts as an adapter for **IpvsController** interaction with the **networking layer**.

  
  


## 2. Callback, Prepare diffs [ What to do? ]
- **types.go**

  ```ServiceInfo``` Ccontains base information of a service in a structure that can be directly consumed by the proxier
  
  ```EndpointInfo``` Contains base information of an endpoint in a structure that can be directly consumed by the proxier

  ```ResourceInfo``` Is used by generic functions 
  ```go
  type ResourceInfo interface {
      ServiceInfo | EndpointInfo
  }
  ```

- **patch.go**

  Here we leverage diffstore to get the deltas for service and endpoints and store them in form of patches. A Patch is basically a combination of resources and operations.
   ```go
  type Operation int32
  
  const (
        NoOp = iota
        Create
        Update
        Delete
  )
  ```
  The following structures are used to organize a ```client.ServiceEndpoints``` diffs into patches
  - ServicePatch
  ```go
  type ServicePatch struct {
        serviceInfo *ServiceInfo    
        op          Operation
  }
  ```
  
  ```go
  func (p *ServicePatch) apply(handler map[Operation]func(serviceInfo *ServiceInfo)) {
        handler[p.op](p.serviceInfo)
  }
  ```
  - EndpointPatch
  ```go
  type EndpointPatch struct {
        endpointInfo *EndpointInfo
        serviceInfo  *ServiceInfo
        op           Operation
  }
  ```
  ```go
  func (p *EndpointPatch) apply(handler map[Operation]func(endpointInfo *EndpointInfo, serviceInfo *ServiceInfo)) {
        handler[p.op](p.endpointInfo, p.serviceInfo)
  }
  ```
  - EndpointPatches
  ```go
  type EndpointPatches []EndpointPatch
  ```
  ```go
  func (e EndpointPatches) apply(handler map[Operation]func(*EndpointInfo, *ServiceInfo)) {
	    for _, patch := range e {
		     patch.apply(handler)
        }
  }
  ```
  - PatchGroup
  ```go
  type PatchGroup struct {
        svc ServicePatch
        eps EndpointPatches
  }
  ```

PatchGroups basically couples **all mutually dependent patches together**. Thus all the PatchGroups are mutually exclusivce and can be applied in parallel in future.
- **ipvs.go**

  ```generatePatchGroups()``` returns the list of PatchGroups required to transition ```client.ServiceEndpoints``` from state A to state B.

## 3. Implementation, Execute diffs [How to do?]
- **(clusterip | nodeport | loadbalacer)_handler.go**

  handlers directly interact with the proxier to implement the network patches
```go
type Handler interface {
	createService(*ServiceInfo)
	createEndpoint(*EndpointInfo, *ServiceInfo)

	updateService(*ServiceInfo)
	updateEndpoint(*EndpointInfo, *ServiceInfo)

	deleteService(*ServiceInfo)
	deleteEndpoint(*EndpointInfo, *ServiceInfo)

	getServiceHandlers() map[Operation]func(*ServiceInfo)
	getEndpointHandlers() map[Operation]func(*EndpointInfo, *ServiceInfo)
}
```

  ```getServiceHandlers()``` and ```getEndpointHandlers()``` of the ```Handler``` interface returns sets of functions that actually implement the low-level networking logic and interact with kernel.

- **patch.go**

  The ```apply()``` methods of the patches takes ```getServiceHandlers()``` and ```getEndpointHandlers()``` **dependency as an argument** to apply the change in the networking stack.
