package overlay

import (
	"fmt"
	"net"

	log "github.com/sirupsen/logrus"
	"github.com/docker/libnetwork/driverapi"
	"github.com/docker/libnetwork/netutils"
	"github.com/vishvananda/netlink"
)

type endpointTable map[string]*endpoint

type endpoint struct {
	id     string
	ifName string
	mac    net.HardwareAddr
	addr   *net.IPNet
}

func (n *network) endpoint(eid string) *endpoint {
	n.Lock()
	defer n.Unlock()

	return n.endpoints[eid]
}

func (n *network) addEndpoint(ep *endpoint) {
	n.Lock()
	n.endpoints[ep.id] = ep
	n.Unlock()
}

func (n *network) deleteEndpoint(eid string) {
	n.Lock()
	delete(n.endpoints, eid)
	n.Unlock()
}

func (d *driver) CreateEndpoint(nid, eid string, ifInfo driverapi.InterfaceInfo,
	epOptions map[string]interface{}) error {
	var err error

	if err = validateID(nid, eid); err != nil {
		return err
	}

	// Since we perform lazy configuration make sure we try
	// configuring the driver when we enter CreateEndpoint since
	// CreateNetwork may not be called in every node.
	if err := d.configure(); err != nil {
		return err
	}

	n := d.network(nid)
	if n == nil {
		return fmt.Errorf("network id %q not found", nid)
	}

	ep := &endpoint{
		id:   eid,
		addr: ifInfo.Address(),
		mac:  ifInfo.MacAddress(),
	}
	if ep.addr == nil {
		return fmt.Errorf("create endpoint was not passed interface IP address")
	}

	if s := n.getSubnetforIP(ep.addr); s == nil {
		return fmt.Errorf("no matching subnet for IP %q in network %q\n", ep.addr, nid)
	}

	if ep.mac == nil {
		ep.mac = netutils.GenerateMACFromIP(ep.addr.IP)
		if err := ifInfo.SetMacAddress(ep.mac); err != nil {
			return err
		}
	}

	n.addEndpoint(ep)

	return nil
}

func (d *driver) DeleteEndpoint(nid, eid string) error {
	if err := validateID(nid, eid); err != nil {
		return err
	}

	n := d.network(nid)
	if n == nil {
		return fmt.Errorf("network id %q not found", nid)
	}

	ep := n.endpoint(eid)
	if ep == nil {
		return fmt.Errorf("endpoint id %q not found", eid)
	}

	n.deleteEndpoint(eid)

	if ep.ifName == "" {
		return nil
	}

	link, err := netlink.LinkByName(ep.ifName)
	if err != nil {
		log.Debugf("Failed to retrieve interface (%s)'s link on endpoint (%s) delete: %v", ep.ifName, ep.id, err)
		return nil
	}
	if err := netlink.LinkDel(link); err != nil {
		log.Debugf("Failed to delete interface (%s)'s link on endpoint (%s) delete: %v", ep.ifName, ep.id, err)
	}

	return nil
}

func (d *driver) EndpointOperInfo(nid, eid string) (map[string]interface{}, error) {
	return make(map[string]interface{}, 0), nil
}
