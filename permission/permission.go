package permission

import (
	"os"

	"github.com/dropbox/godropbox/errors"
	"github.com/pritunl/pritunl-cloud/errortypes"
	"github.com/pritunl/pritunl-cloud/paths"
	"github.com/pritunl/pritunl-cloud/utils"
	"github.com/pritunl/pritunl-cloud/vm"
)

func chown(virt *vm.VirtualMachine, path string) (err error) {
	err = os.Chown(path, virt.UnixId, 0)
	if err != nil {
		err = &errortypes.WriteError{
			errors.Newf(
				"permission: Failed to set owner of '%s' to '%d'",
				path, virt.UnixId,
			),
		}
		return
	}

	return
}

func touchChown(virt *vm.VirtualMachine, path string) (err error) {
	_, err = utils.ExecCombinedOutputLogged(nil,
		"touch", path,
	)
	if err != nil {
		return
	}

	err = chown(virt, path)
	if err != nil {
		return
	}

	return
}

func InitVirt(virt *vm.VirtualMachine) (err error) {
	err = UserAdd(virt)
	if err != nil {
		return
	}

	if virt.Uefi {
		err = chown(virt, paths.GetOvmfVarsPath(virt.Id))
		if err != nil {
			return
		}
	}

	err = chown(virt, paths.GetInitPath(virt.Id))
	if err != nil {
		return
	}

	for _, disk := range virt.Disks {
		err = chown(virt, disk.Path)
		if err != nil {
			return
		}
	}

	for _, device := range virt.DriveDevices {
		err = chown(virt, paths.GetDrivePath(device.Id))
		if err != nil {
			return
		}
	}

	return
}

func InitDisk(virt *vm.VirtualMachine, dsk *vm.Disk) (err error) {
	err = UserAdd(virt)
	if err != nil {
		return
	}

	err = chown(virt, dsk.Path)
	if err != nil {
		return
	}

	return
}
