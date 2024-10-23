package provider

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	gcputil "github.com/daytonaio/daytona-provider-gcp/pkg/provider/util"
	"github.com/daytonaio/daytona-provider-gcp/pkg/types"
	"github.com/daytonaio/daytona/pkg/provider"
	"github.com/daytonaio/daytona/pkg/workspace"
)

var (
	credentialFile = os.Getenv("GCP_CREDENTIAL_FILE")
	projectId      = os.Getenv("GCP_PROJECT_ID")

	azureProvider = &GCPProvider{}
	targetOptions = &types.TargetOptions{
		CredentialFile: credentialFile,
		ProjectID:      projectId,
		Zone:           "us-central1-a",
		MachineType:    "n1-standard-1",
		DiskType:       "pd-standard",
		DiskSize:       20,
		VMImage:        "projects/ubuntu-os-cloud/global/images/family/ubuntu-2204-lts",
	}

	workspaceReq *provider.WorkspaceRequest
)

func TestCreateWorkspace(t *testing.T) {
	_, _ = azureProvider.CreateWorkspace(workspaceReq)

	_, err := gcputil.GetComputeInstance(workspaceReq.Workspace, targetOptions)
	if err != nil {
		t.Fatalf("Error getting machine: %s", err)
	}
}

func TestWorkspaceInfo(t *testing.T) {
	workspaceInfo, err := azureProvider.GetWorkspaceInfo(workspaceReq)
	if err != nil {
		t.Fatalf("Error getting workspace info: %s", err)
	}

	var workspaceMetadata types.WorkspaceMetadata
	err = json.Unmarshal([]byte(workspaceInfo.ProviderMetadata), &workspaceMetadata)
	if err != nil {
		t.Fatalf("Error unmarshalling workspace metadata: %s", err)
	}

	vm, err := gcputil.GetComputeInstance(workspaceReq.Workspace, targetOptions)
	if err != nil {
		t.Fatalf("Error getting machine: %s", err)
	}

	expectedMetadata := types.ToWorkspaceMetadata(vm)

	if expectedMetadata.VirtualMachineId != workspaceMetadata.VirtualMachineId {
		t.Fatalf("Expected vm id %d, got %d",
			expectedMetadata.VirtualMachineId,
			workspaceMetadata.VirtualMachineId,
		)
	}

	if expectedMetadata.VirtualMachineName != workspaceMetadata.VirtualMachineName {
		t.Fatalf("Expected vm name %s, got %s",
			expectedMetadata.VirtualMachineName,
			workspaceMetadata.VirtualMachineName,
		)
	}

	if expectedMetadata.Platform != workspaceMetadata.Platform {
		t.Fatalf("Expected vm platform %s, got %s",
			expectedMetadata.Platform,
			workspaceMetadata.Platform,
		)
	}

	if expectedMetadata.Location != workspaceMetadata.Location {
		t.Fatalf("Expected vm location %s, got %s",
			expectedMetadata.Location,
			workspaceMetadata.Location,
		)
	}

	if expectedMetadata.Created != workspaceMetadata.Created {
		t.Fatalf("Expected vm created at %s, got %s",
			expectedMetadata.Created,
			workspaceMetadata.Created,
		)
	}
}

func TestDestroyWorkspace(t *testing.T) {
	_, err := azureProvider.DestroyWorkspace(workspaceReq)
	if err != nil {
		t.Fatalf("Error destroying workspace: %s", err)
	}
	time.Sleep(3 * time.Second)

	_, err = gcputil.GetComputeInstance(workspaceReq.Workspace, targetOptions)
	if err == nil {
		t.Fatalf("Error destroyed workspace still exists")
	}
}

func init() {
	_, err := azureProvider.Initialize(provider.InitializeProviderRequest{
		BasePath:           "/tmp/workspaces",
		DaytonaDownloadUrl: "https://download.daytona.io/daytona/install.sh",
		DaytonaVersion:     "latest",
		ServerUrl:          "",
		ApiUrl:             "",
		LogsDir:            "/tmp/logs",
	})
	if err != nil {
		panic(err)
	}

	opts, err := json.Marshal(targetOptions)
	if err != nil {
		panic(err)
	}

	workspaceReq = &provider.WorkspaceRequest{
		TargetOptions: string(opts),
		Workspace: &workspace.Workspace{
			Id:   "123",
			Name: "workspace",
		},
	}
}
