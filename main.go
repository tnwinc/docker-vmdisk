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
	//"reflect"
	"strings"
	//"text/tabwriter"
	"regexp"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	//"github.com/vmware/govmomi/govc/flags"
	//"github.com/vmware/govmomi/object"
	//"github.com/vmware/govmomi/property"
	//"github.com/vmware/govmomi/vim25"
	//"github.com/vmware/govmomi/vim25/mo"
	//"github.com/vmware/govmomi/vim25/types"
	"golang.org/x/net/context"
)

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
		fmt.Printf("objecttype=%s\n", item.Object.Reference().Type)
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

	//todo: check if target is valid

	// Connect and log in to ESX or vCenter
	c, err := govmomi.NewClient(ctx, u, *insecureFlag)
	if err != nil {
		exit(err)
	}

	f := find.NewFinder(c.Client, true)

	//check that target was specified and is valid
	target := os.Getenv(envTarget)
	if target == "" {
		fmt.Fprintf(os.Stderr, "Target VM must be specified with env var %s\n", envTarget)
		os.Exit(1)
	}
	fmt.Printf("target=%s\n", target)
	targetvm, err := f.VirtualMachine(ctx, target)
	if targetvm == nil {
		fmt.Fprintf(os.Stderr, "Error: No VM found at %s. It needs to be a full path.\n", target)
		os.Exit(1)
	}
	if err != nil {
		exit(err)
	}

	fmt.Printf("found target at %s\n", target)

	//check if disk is specified
	disk := os.Getenv(envDisk)
	if disk == "" {
		fmt.Fprintf(os.Stderr, "Target disk must be specified with env var %s\n", envDisk)
		os.Exit(1)
	}

	fmt.Printf("disk=%s\n", disk)
	if dsPathRegexp.MatchString(disk) == false {
		fmt.Fprintf(os.Stderr, "Target disk must be in format [datastorename] path/to/vmdk.vmdk")
		os.Exit(1)
	}
	var dsNameRegexp = regexp.MustCompile("\\[(.*?)\\]")
	diskdatastore := dsNameRegexp.FindStringSubmatch(disk)[1]
	diskfilepath := dsPathRegexp.FindStringSubmatch(disk)[1]
	fmt.Printf("diskfilepath=%s\n", diskfilepath)
	fmt.Printf("diskdatastore=%s\n", diskdatastore)

	//try to figure out the DC from the target machine
	dcNameRegexp := regexp.MustCompile("^\\/(.*?)\\/")
	targetdc := dcNameRegexp.FindStringSubmatch(target)[1]
	fmt.Printf("targetdc=%s\n", targetdc)

	//based on the safe assumption that the target VM and the disk datastore are in the same DC, build a path
	diskdatastorepath := "/" + targetdc + "/datastore/" + diskdatastore
	fmt.Printf("diskdatastorepath=%s\n", diskdatastorepath)

	//targetds, err := f.Datastore(diskdatastorepath)
	targetds, err := f.Datastore(ctx, diskdatastorepath)
	fmt.Printf("targetds--> %+v\n", targetds)

	//initialize the basepath to the target datacenter if not specified
	basepath := os.Getenv(envBasepath)
	basepath = strings.TrimRight(basepath, "/")
	if basepath == "" {
		//note: use the DC of the target machine if none specified - you can't mount disks between anyway
		basepath = "/" + targetdc
	}
	fmt.Printf("basepath=%s\n", basepath)

	vmpaths := findAllObjectsOfType(f, ctx, basepath, "VirtualMachine")

	//try to find and detach the target disk for all VMs in the path
	for _, vmpath := range vmpaths {
		fmt.Printf("%s\n", vmpath)
		vm, err := f.VirtualMachine(ctx, vmpath)
		if err != nil {
			exit(err)
		}

		_, err = ddisk(vm, c.Client, disk)

	}

	//targetvm
	//targetds
	//diskfilepath

	vmdevices, err := targetvm.Device(ctx)

	//note: Just find and use the first disk controller - this may require rethinking
	diskcontroller, err := vmdevices.FindDiskController("")
	fmt.Printf("diskcontroller--> %+v\n", diskcontroller)

	vmdisk := vmdevices.CreateDisk(diskcontroller, targetds.Path(diskfilepath))
	//backing := vmdisk.Backing.(*types.VirtualDiskFlatVer2BackingInfo)

	targetvm.AddDevice(ctx, vmdisk)
}
