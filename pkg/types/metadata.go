package types

import (
	"cloud.google.com/go/compute/apiv1/computepb"
)

type WorkspaceMetadata struct {
	VirtualMachineId   uint64
	VirtualMachineName string
	Platform           string
	Location           string
	Created            string
}

// ToWorkspaceMetadata converts and maps values from an *computepb.Instance to a WorkspaceMetadata.
func ToWorkspaceMetadata(vm *computepb.Instance) WorkspaceMetadata {
	return WorkspaceMetadata{
		VirtualMachineId:   vm.GetId(),
		VirtualMachineName: vm.GetName(),
		Platform:           vm.GetCpuPlatform(),
		Location:           vm.GetZone(),
		Created:            vm.GetCreationTimestamp(),
	}
}
