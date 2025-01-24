package types

import (
	"cloud.google.com/go/compute/apiv1/computepb"
)

type TargetMetadata struct {
	VirtualMachineId   uint64
	VirtualMachineName string
	Platform           string
	Location           string
	Created            string
}

// ToTargetMetadata converts and maps values from an *computepb.Instance to a TargetMetadata.
func ToTargetMetadata(vm *computepb.Instance) TargetMetadata {
	return TargetMetadata{
		VirtualMachineId:   vm.GetId(),
		VirtualMachineName: vm.GetName(),
		Platform:           vm.GetCpuPlatform(),
		Location:           vm.GetZone(),
		Created:            vm.GetCreationTimestamp(),
	}
}
