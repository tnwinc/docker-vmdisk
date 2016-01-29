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
	"flag"
	"fmt"
	"net/url"
	"os"
	"reflect"
	"strings"
	//"text/tabwriter"
	"regexp"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
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

/*
func init() {
	cli.Register("import.vmdk", &vmdk{})
	cli.Alias("import.vmdk", "datastore.import")
}
*/

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
			if file != "" {
				panic("expected VM to have only one disk")
			}

			switch backing := device.Backing.(type) {
			case *types.VirtualDiskFlatVer1BackingInfo:
				file = backing.FileName
			case *types.VirtualDiskFlatVer2BackingInfo:
				file = backing.FileName
			case *types.VirtualDiskSeSparseBackingInfo:
				file = backing.FileName
			case *types.VirtualDiskSparseVer1BackingInfo:
				file = backing.FileName
			case *types.VirtualDiskSparseVer2BackingInfo:
				file = backing.FileName
			default:
				name := reflect.TypeOf(device.Backing).String()
				panic(fmt.Sprintf("unexpected backing type: %s", name))
			}
			fmt.Printf("%+v\n", file)
			// Remove [datastore] prefix
			m := dsPathRegexp.FindStringSubmatch(file)
			if len(m) != 2 {
				panic(fmt.Sprintf("expected regexp match for %#v", file))
			}
			file = m[1]

			removeOp := &types.VirtualDeviceConfigSpec{
				Operation: types.VirtualDeviceConfigSpecOperationRemove,
				Device:    device,
			}

			c.AddChange(removeOp)
		}
	}

	return file
}

/*
func (cmd *vmdk) detachDisk(vm *object.VirtualMachine) (string, error) {
	var mvm mo.VirtualMachine

	pc := property.DefaultCollector(cmd.Client)
	err := pc.RetrieveOne(context.TODO(), vm.Reference(), []string{"config.hardware"}, &mvm)
	if err != nil {
		return "", err
	}

	spec := new(configSpec)
	dsFile := spec.RemoveDisk(&mvm)

	task, err := vm.Reconfigure(context.TODO(), spec.ToSpec())
	if err != nil {
		return "", err
	}

	err = task.Wait(context.TODO())
	if err != nil {
		return "", err
	}

	return dsFile, nil
}
*/
func ddisk(vm *object.VirtualMachine, client *vim25.Client, diskfile string) (string, error) {
	var mvm mo.VirtualMachine

	pc := property.DefaultCollector(client)
	err := pc.RetrieveOne(context.TODO(), vm.Reference(), []string{"config.hardware"}, &mvm)
	if err != nil {
		return "", err
	}

	spec := new(configSpec)
	dsFile := spec.RemoveDisk(&mvm, diskfile)

	task, err := vm.Reconfigure(context.TODO(), spec.ToSpec())
	if err != nil {
		return "", err
	}

	err = task.Wait(context.TODO())
	if err != nil {
		return "", err
	}

	return dsFile, nil
}

// GetEnvString returns string from environment variable.
func GetEnvString(v string, def string) string {
	r := os.Getenv(v)
	if r == "" {
		return def
	}

	return r
}

// GetEnvBool returns boolean from environment variable.
func GetEnvBool(v string, def bool) bool {
	r := os.Getenv(v)
	if r == "" {
		return def
	}

	switch strings.ToLower(r[0:1]) {
	case "t", "y", "1":
		return true
	}

	return false
}

const (
	envURL      = "GOVMOMI_URL"
	envUserName = "GOVMOMI_USERNAME"
	envPassword = "GOVMOMI_PASSWORD"
	envInsecure = "GOVMOMI_INSECURE"
	envBasepath = "VMDISK_BASEPATH"
	envTarget   = "VMDISK_TARGET"
	envDisk     = "VMDISK_DISK"
)

var urlDescription = fmt.Sprintf("ESX or vCenter URL [%s]", envURL)
var urlFlag = flag.String("url", GetEnvString(envURL, "https://root:vmware@192.168.229.10/sdk"), urlDescription)

var insecureDescription = fmt.Sprintf("Don't verify the server's certificate chain [%s]", envInsecure)
var insecureFlag = flag.Bool("insecure", GetEnvBool(envInsecure, true), insecureDescription)

func processOverride(u *url.URL) {
	envUsername := os.Getenv(envUserName)
	envPassword := os.Getenv(envPassword)

	// Override username if provided
	if envUsername != "" {
		var password string
		var ok bool

		if u.User != nil {
			password, ok = u.User.Password()
		}

		if ok {
			u.User = url.UserPassword(envUsername, password)
		} else {
			u.User = url.User(envUsername)
		}
	}

	// Override password if provided
	if envPassword != "" {
		var username string

		if u.User != nil {
			username = u.User.Username()
		}

		u.User = url.UserPassword(username, envPassword)
	}
}

func exit(err error) {
	fmt.Fprintf(os.Stderr, "Error: %s\n", err)
	os.Exit(1)
}

func findAllObjectsOfType(f *find.Finder, ctx context.Context, path string, findtype string) []string {
	out := []string{}
	items, err := f.ManagedObjectListChildren(ctx, path)
	if err != nil {
		exit(err)
	} //todo: this should return the err
	for _, item := range items {
		//fmt.Printf("%s\n", item.Object.Reference().Type)
		if item.Object.Reference().Type == findtype {
			out = append(out, item.Path)
		}
		if item.Object.Reference().Type == "Folder" {
			out = append(out, findAllObjectsOfType(f, ctx, item.Path, findtype)...)
		}
		if item.Object.Reference().Type == "Datacenter" {
			out = append(out, findAllObjectsOfType(f, ctx, item.Path, findtype)...)
		}
	}
	return out

}
func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	flag.Parse()

	// Parse URL from string
	u, err := url.Parse(*urlFlag)
	if err != nil {
		exit(err)
	}

	// Override username and/or password as required
	processOverride(u)

	//check that target was specified
	target := os.Getenv(envTarget)
	if target == "" {
		fmt.Fprintf(os.Stderr, "Target VM must be specified with env var %s\n", envTarget)
		os.Exit(1)
	}
	//todo: check if target is valid

	//check if disk is specified
	disk := os.Getenv(envTarget)
	if disk == "" {
		fmt.Fprintf(os.Stderr, "Target disk must be specified with env var %s\n", envDisk)
		os.Exit(1)
	}
	//todo: check if disk is valid

	// Connect and log in to ESX or vCenter
	c, err := govmomi.NewClient(ctx, u, *insecureFlag)
	if err != nil {
		exit(err)
	}

	f := find.NewFinder(c.Client, true)
	//listAllVM(f, ctx, "/datacenter0/vm")

	basepath := os.Getenv(envBasepath)
	basepath = strings.TrimRight(basepath, "/")
	if basepath == "" {
		basepath = "/"
	}
	fmt.Printf("basepath=%s\n", basepath)

	vmpaths := findAllObjectsOfType(f, ctx, basepath, "VirtualMachine")

	for _, vmpath := range vmpaths {
		fmt.Printf("%s\n", vmpath)
		//vm, err := f.ManagedObjectList(ctx, vmpath)
		vm, err := f.VirtualMachine(ctx, vmpath)
		if err != nil {
			exit(err)
		}

		vmdk, err := ddisk(vm, c.Client, "[datastore1] scrap1/scrap1.vmdk")
		fmt.Printf("this is the VMDK %+v\n", vmdk)
		fmt.Printf("%+v\n", vm)
		pc := property.DefaultCollector(c.Client)
		var mvm mo.VirtualMachine
		err = pc.RetrieveOne(ctx, vm.Reference(), []string{"config.hardware"}, &mvm)
		spec := new(types.VirtualMachineConfigSpec)

		fmt.Printf("%+v\n", mvm)
		fmt.Printf("%+v\n", spec)

		devices, err := vm.Device(ctx)

		fmt.Printf("%+v\n", devices)

		for _, device := range devices {
			fmt.Printf("%+v\n", device.GetVirtualDevice().DeviceInfo)
		}
		/*
			controller, err := devices.FindSCSIController("")
			fmt.Printf("%+v\n", controller)
		*/
		//fmt.Printf("%+v\n", controllers)
		/*
			fmt.Println(reflect.TypeOf(vm))
			o := vm.Value.(Object)
		*/
		break
	}

	//vms, err := f.VirtualMachine(ctx, "*")
	//vms, err := f.VirtualMachineList(ctx, "/")

	/*
		datacenters := []string{}
		dss, err := f.ManagedObjectListChildren(ctx, "/")
		for _, ds := range dss {
			//fmt.Sprintf("derp %s\n", ds.Path)
			fmt.Printf("%+v\n", ds)
			fmt.Printf("%s\n", ds.Path)
			datacenters = append(datacenters, ds.Path)
		}
		//fmt.Printf("%+v\n", dss)
		fmt.Printf("%+v\n", err)
		fmt.Printf("%i\n", len(dss))
		fmt.Printf("%v+\n", datacenters)

		vms, err := f.ManagedObjectListChildren(ctx, "/datacenter0/vm")
		for _, vm := range vms {
			fmt.Printf("%+v\n", vm)
		}
	*/
	/*
		for _, vm := range vms {
			fmt.Sprintf("%s\t", vm.Summary.Name)
		}
	*/
	/*
		// Find one and only datacenter
		dc, err := f.DefaultDatacenter(ctx)
		if err != nil {
			exit(err)
		}

		// Make future calls local to this datacenter
		f.SetDatacenter(dc)

		// Find datastores in datacenter
		dss, err := f.DatastoreList(ctx, "*")
		if err != nil {
			exit(err)
		}

		pc := property.DefaultCollector(c.Client)

		// Convert datastores into list of references
		var refs []types.ManagedObjectReference
		for _, ds := range dss {
			refs = append(refs, ds.Reference())
		}

		// Retrieve summary property for all datastores
		var dst []mo.Datastore
		err = pc.Retrieve(ctx, refs, []string{"summary"}, &dst)
		if err != nil {
			exit(err)
		}

		// Print summary per datastore
		tw := tabwriter.NewWriter(os.Stdout, 2, 0, 2, ' ', 0)
		fmt.Fprintf(tw, "Name:\tType:\tCapacity:\tFree:\n")
		for _, ds := range dst {
			fmt.Fprintf(tw, "%s\t", ds.Summary.Name)
			fmt.Fprintf(tw, "%s\t", ds.Summary.Type)
			fmt.Fprintf(tw, "%s\t", units.ByteSize(ds.Summary.Capacity))
			fmt.Fprintf(tw, "%s\t", units.ByteSize(ds.Summary.FreeSpace))
			fmt.Fprintf(tw, "\n")
		}
		tw.Flush()
	*/
}
