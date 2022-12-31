package ipvsfullsate

import (
	"fmt"
	IPVS "github.com/google/seesaw/ipvs"
	"github.com/vishvananda/netlink"
	"k8s.io/apimachinery/pkg/util/version"
	"k8s.io/klog/v2"
	"net"
	"sigs.k8s.io/kpng/backends/ipvs-as-sink/util"
)

const dummyName = "kube-ipvs0"

// In IPVS proxy mode, the following flags need to be set
const (
	sysctlBridgeCallIPTables           = "net/bridge/bridge-nf-call-iptables"
	sysctlVSConnTrack                  = "net/ipv4/vs/conntrack"
	sysctlConnReuse                    = "net/ipv4/vs/conn_reuse_mode"
	sysctlExpireNoDestConn             = "net/ipv4/vs/expire_nodest_conn"
	sysctlExpireQuiescentTemplate      = "net/ipv4/vs/expire_quiescent_template"
	sysctlForward                      = "net/ipv4/ip_forward"
	sysctlArpIgnore                    = "net/ipv4/conf/all/arp_ignore"
	sysctlArpAnnounce                  = "net/ipv4/conf/all/arp_announce"
	connReuseMinSupportedKernelVersion = "4.1"
	// https://github.com/torvalds/linux/commit/35dfb013149f74c2be1ff9c78f14e6a3cd1539d1
	connReuseFixedKernelVersion = "5.9"
)

func (b *backend) Setup() {
	var err error

	err = initializeKernelConfig(util.NewLinuxKernelHandler())
	if err != nil {
		klog.Fatal("Unable to initialize kernel for IPVS mode")
	}

	err = IPVS.Init()
	if err != nil {
		klog.Fatal("Unable to initialize ipvs interface")
	}

	dummy, err := createIPVSDummyInterface()
	if err != nil {
		klog.Fatal("failed to initialize dummy interface")
	}

	controller = NewIPVSController(dummy)
	klog.V(4).Info("IPVS controller initialized")
}

func createIPVSDummyInterface() (netlink.Link, error) {
	// populate dummyIPs

	dummy, err := netlink.LinkByName(dummyName)
	if err != nil {
		if _, ok := err.(netlink.LinkNotFoundError); !ok {
			klog.Fatal("failed to get dummy interface: ", err)
			return nil, err
		}

		// not found => create the dummy
		dummy = &netlink.Dummy{
			LinkAttrs: netlink.LinkAttrs{Name: dummyName},
		}

		klog.Info("creating dummy interface ", dummyName)
		if err = netlink.LinkAdd(dummy); err != nil {
			klog.Fatal("failed to create dummy interface: ", err)
			return nil, err
		}

		dummy, err = netlink.LinkByName(dummyName)
		if err != nil {
			klog.Fatal("failed to get link after create: ", err)
			return nil, err
		}
	}

	if dummy.Attrs().Flags&net.FlagUp == 0 {
		klog.Info("setting dummy interface ", dummyName, " up")
		if err = netlink.LinkSetUp(dummy); err != nil {
			klog.Fatal("failed to set dummy interface up: ", err)
			return nil, err
		}
	}
	return dummy, nil
}

func initializeKernelConfig(kernelHandler util.KernelHandler) error {
	// Proxy needs br_netfilter and bridge-nf-call-iptables=1 when containers
	// are connected to a Linux bridge (but not SDN bridges).  Until most
	// plugins handle this, log when config is missing
	sysctl := util.NewSysInterface()
	if val, err := sysctl.GetSysctl(sysctlBridgeCallIPTables); err == nil && val != 1 {
		klog.Info("Missing br-netfilter module or unset sysctl br-nf-call-iptables, proxy may not work as intended")
	}

	// Set the conntrack sysctl we need for
	if err := util.EnsureSysctl(sysctl, sysctlVSConnTrack, 1); err != nil {
		return err
	}

	kernelVersionStr, err := kernelHandler.GetKernelVersion()
	if err != nil {
		return fmt.Errorf("error determining kernel version to find required kernel modules for ipvs support: %v", err)
	}
	kernelVersion, err := version.ParseGeneric(kernelVersionStr)
	if err != nil {
		return fmt.Errorf("error parsing kernel version %q: %v", kernelVersionStr, err)
	}
	if kernelVersion.LessThan(version.MustParseGeneric(connReuseMinSupportedKernelVersion)) {
		klog.Error(nil, "Can't set sysctl, kernel version doesn't satisfy minimum version requirements", "sysctl", sysctlConnReuse, "minimumKernelVersion", connReuseMinSupportedKernelVersion)
	} else if kernelVersion.AtLeast(version.MustParseGeneric(connReuseFixedKernelVersion)) {
		// https://github.com/kubernetes/kubernetes/issues/93297
		klog.V(2).Info("Left as-is", "sysctl", sysctlConnReuse)
	} else {
		// Set the connection reuse mode
		if err := util.EnsureSysctl(sysctl, sysctlConnReuse, 0); err != nil {
			return err
		}
	}

	// Set the expire_nodest_conn sysctl we need for
	if err := util.EnsureSysctl(sysctl, sysctlExpireNoDestConn, 1); err != nil {
		return err
	}

	// Set the expire_quiescent_template sysctl we need for
	if err := util.EnsureSysctl(sysctl, sysctlExpireQuiescentTemplate, 1); err != nil {
		return err
	}

	// Set the ip_forward sysctl we need for
	if err := util.EnsureSysctl(sysctl, sysctlForward, 1); err != nil {
		return err
	}

	//if strictARP {
	//	// Set the arp_ignore sysctl we need for
	//	if err := utilproxy.EnsureSysctl(sysctl, sysctlArpIgnore, 1); err != nil {
	//		return err
	//	}
	//
	//	// Set the arp_announce sysctl we need for
	//	if err := utilproxy.EnsureSysctl(sysctl, sysctlArpAnnounce, 2); err != nil {
	//		return err
	//	}
	//}
	return nil
}
