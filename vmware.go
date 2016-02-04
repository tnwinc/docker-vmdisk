/*
Copyright (c) 2015 VMware, Inc. All Rights Reserved.

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

/*
This example program shows how the `finder` and `property` packages can
be used to navigate a vSphere inventory structure using govmomi.
*/

package main

import (
	//"flag"
	"fmt"
	//"net/url"
	//"os"
	//"reflect"
	//"strings"
	//"text/tabwriter"
	"regexp"

	//"github.com/vmware/govmomi"
	//"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/govc/flags"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
	"golang.org/x/net/context"
)

type configSpec types.VirtualMachineConfigSpec
type vmdk struct {
	*flags.DatastoreFlag
	*flags.ResourcePoolFlag
	*flags.OutputFlag

	upload bool
	force  bool
	keep   bool

	Client       *vim25.Client
	Datacenter   *object.Datacenter
	Datastore    *object.Datastore
	ResourcePool *object.ResourcePool
}

var dsPathRegexp = regexp.MustCompile(`^\[.*\] (.*)$`)

func (c *configSpec) AddChange(d types.BaseVirtualDeviceConfigSpec) {
	c.DeviceChange = append(c.DeviceChange, d)
}
func (c *configSpec) ToSpec() types.VirtualMachineConfigSpec {
	return types.VirtualMachineConfigSpec(*c)
}
func (c *configSpec) RemoveDisk(vm *mo.VirtualMachine, diskfile string) string {
	var file string
	for _, d := range vm.Config.Hardware.Device {
		switch device := d.(type) {
		case *types.VirtualDisk:

			switch backing := device.Backing.(type) {
			case *types.VirtualDiskFlatVer1BackingInfo:
				file = backing.FileName
				fmt.Printf("VirtualDiskFlatVer1BackingInfo--> %+v\n", backing)
			case *types.VirtualDiskFlatVer2BackingInfo:
				file = backing.FileName
				fmt.Printf("VirtualDiskFlatVer2BackingInfo--> %+v\n", backing)
			case *types.VirtualDiskSeSparseBackingInfo:
				file = backing.FileName
				fmt.Printf("VirtualDiskSeSparseBackingInfo--> %+v\n", backing)
			case *types.VirtualDiskSparseVer1BackingInfo:
				file = backing.FileName
				fmt.Printf("VirtualDiskSparseVer1BackingInfo--> %+v\n", backing)
			case *types.VirtualDiskSparseVer2BackingInfo:
				file = backing.FileName
				fmt.Printf("VirtualDiskSparseVer2BackingInfo--> %+v\n", backing)
			}

			if file == diskfile {
				fmt.Printf("found %s\n", file)
				removeOp := &types.VirtualDeviceConfigSpec{
					Operation: types.VirtualDeviceConfigSpecOperationRemove,
					Device:    device,
				}
				c.AddChange(removeOp)
				return file

			}
		}
	}

	return ""
}

func ddisk(vm *object.VirtualMachine, client *vim25.Client, diskfile string) (string, error) {
	var mvm mo.VirtualMachine

	pc := property.DefaultCollector(client)
	err := pc.RetrieveOne(context.TODO(), vm.Reference(), []string{"config.hardware"}, &mvm)
	if err != nil {
		return "", err
	}

	spec := new(configSpec)
	dsFile := spec.RemoveDisk(&mvm, diskfile)
	if dsFile != "" {
		task, err := vm.Reconfigure(context.TODO(), spec.ToSpec())
		if err != nil {
			return "", err
		}

		err = task.Wait(context.TODO())
		if err != nil {
			return "", err
		}
	}
	return dsFile, nil
}
