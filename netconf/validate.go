package netconf

import (
	"net"
	"time"

	"github.com/dropbox/godropbox/container/set"
	"github.com/dropbox/godropbox/errors"
	"github.com/pritunl/pritunl-cloud/errortypes"
	"github.com/pritunl/pritunl-cloud/paths"
	"github.com/pritunl/pritunl-cloud/utils"
	"github.com/pritunl/pritunl-cloud/vm"
)

func (n *NetConf) Validate() (err error) {
	err = utils.ExistsMkdir(paths.GetLeasesPath(), 0755)
	if err != nil {
		return
	}

	if len(n.Virt.NetworkAdapters) == 0 {
		err = &errortypes.NotFoundError{
			errors.New("netconf: Missing network interfaces"),
		}
		return
	}

	ifaceNames := set.NewSet()

	for i := range n.Virt.NetworkAdapters {
		ifaceNames.Add(vm.GetIface(n.Virt.Id, i))
	}

	for i := range n.Virt.NetworkAdapters {
		ifaceNames.Add(vm.GetIface(n.Virt.Id, i))
	}

	for i := 0; i < 100; i++ {
		ifaces, e := net.Interfaces()
		if e != nil {
			err = &errortypes.ReadError{
				errors.Wrap(e, "qemu: Failed to get network interfaces"),
			}
			return
		}

		for _, iface := range ifaces {
			if ifaceNames.Contains(iface.Name) {
				ifaceNames.Remove(iface.Name)
			}
		}

		if ifaceNames.Len() == 0 {
			break
		}

		time.Sleep(250 * time.Millisecond)
	}

	if ifaceNames.Len() != 0 {
		err = &errortypes.ReadError{
			errors.New("qemu: Failed to find network interfaces"),
		}
		return
	}

	return
}
