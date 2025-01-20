package provider

import (
	"os"
	"testing"
	"time"

	gcputil "github.com/daytonaio/daytona-provider-gcp/pkg/provider/util"
	"github.com/daytonaio/daytona-provider-gcp/pkg/types"
	"github.com/daytonaio/daytona/pkg/models"
	"github.com/daytonaio/daytona/pkg/provider"
)

var (
	credentialFile = os.Getenv("GCP_CREDENTIAL_FILE")
	workspaceId    = os.Getenv("GCP_PROJECT_ID")

	azureProvider = &GCPProvider{}
	targetOptions = &types.TargetOptions{
		CredentialFile: credentialFile,
		WorkspaceID:    workspaceId,
		Zone:           "us-central1-a",
		MachineType:    "n1-standard-1",
		DiskType:       "pd-standard",
		DiskSize:       20,
		VMImage:        "projects/ubuntu-os-cloud/global/images/family/ubuntu-2204-lts",
	}

	targetReq *provider.TargetRequest
)

func TestCreateTarget(t *testing.T) {
	_, _ = azureProvider.CreateTarget(targetReq)

	_, err := gcputil.GetComputeInstance(targetReq.Target, targetOptions)
	if err != nil {
		t.Fatalf("Error getting machine: %s", err)
	}
}

func TestDestroyTarget(t *testing.T) {
	_, err := azureProvider.DestroyTarget(targetReq)
	if err != nil {
		t.Fatalf("Error destroying target: %s", err)
	}
	time.Sleep(3 * time.Second)

	_, err = gcputil.GetComputeInstance(targetReq.Target, targetOptions)
	if err == nil {
		t.Fatalf("Error destroyed target still exists")
	}
}

func init() {
	_, err := azureProvider.Initialize(provider.InitializeProviderRequest{
		BasePath:           "/tmp/targets",
		DaytonaDownloadUrl: "https://download.daytona.io/daytona/install.sh",
		DaytonaVersion:     "latest",
		ServerUrl:          "",
		ApiUrl:             "",
		TargetLogsDir:      "/tmp/logs",
	})
	if err != nil {
		panic(err)
	}

	targetReq = &provider.TargetRequest{
		Target: &models.Target{
			Id:   "123",
			Name: "target",
		},
	}
}
