package main

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
