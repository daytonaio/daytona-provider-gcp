package types

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/daytonaio/daytona/pkg/provider"
)

type TargetOptions struct {
	CredentialFile string `json:"Credential File"`
	ProjectID      string `json:"Project Id"`
	Zone           string `json:"Zone"`
	MachineType    string `json:"Machine Type"`
	DiskType       string `json:"Disk Type"`
	DiskSize       int    `json:"Disk Size"`
	VMImage        string `json:"VM Image"`
}

func GetTargetManifest() *provider.ProviderTargetManifest {
	return &provider.ProviderTargetManifest{
		"Credential File": provider.ProviderTargetProperty{
			Type: provider.ProviderTargetPropertyTypeFilePath,
			Description: "Full path to the GCP service account JSON key file.\nLeave blank if you've set the GCP_CREDENTIAL_FILE" +
				"environment variable.\nEnsure that the file is secure and accessible only to authorized users.",
			DefaultValue: "~/.config/gcloud",
		},
		"Project Id": provider.ProviderTargetProperty{
			Type:        provider.ProviderTargetPropertyTypeString,
			InputMasked: true,
			Description: "The GCP project ID where the resources will be created.\nLeave blank if you've set the GCP_PROJECT_ID.\n" +
				"How to locate the project ID:\nhttps://support.google.com/googleapi/answer/7014113?hl=en",
		},
		"Zone": provider.ProviderTargetProperty{
			Type: provider.ProviderTargetPropertyTypeString,
			Description: "The GCP zone where the resources will be created. Default is us-central1-a.\n" +
				"https://cloud.google.com/compute/docs/regions-zones\n" +
				"List of available zones can be retrieved using the command:\ngcloud compute zones list",
			DefaultValue: "us-central1-a",
			Suggestions:  zones,
		},
		"Machine Type": provider.ProviderTargetProperty{
			Type: provider.ProviderTargetPropertyTypeString,
			Description: "The GCP machine type to use for the VM. Default is List n1-standard-1.\n" +
				"https://cloud.google.com/compute/docs/general-purpose-machines\n" +
				"List of available machine types can be retrieved using the command:\ngcloud compute machine-types list",
			DefaultValue: "n1-standard-1",
			Suggestions:  machineTypes,
		},
		"Disk Type": provider.ProviderTargetProperty{
			Type: provider.ProviderTargetPropertyTypeString,
			Description: "The GCP disk type to use for the VM. Default is pd-standard.\n" +
				"https://cloud.google.com/compute/docs/disks\n" +
				"List of available disk types can be retrieved using the command:\ngcloud compute disk-types list",
			DefaultValue: "pd-standard",
			Suggestions:  diskTypes,
		},
		"Disk Size": provider.ProviderTargetProperty{
			Type:         provider.ProviderTargetPropertyTypeInt,
			Description:  "The size of the instance volume, in GB. Default is 20 GB.",
			DefaultValue: "20",
		},
		"VM Image": provider.ProviderTargetProperty{
			Type: provider.ProviderTargetPropertyTypeString,
			Description: "The GCP image to use for the VM.\nDefault is projects/ubuntu-os-cloud/global/images/family/ubuntu-2204-lts\n" +
				"https://cloud.google.com/compute/docs/images\n" +
				"List of available images can be retrieved using the command:\ngcloud compute images list",
			DefaultValue: "projects/ubuntu-os-cloud/global/images/family/ubuntu-2204-lts",
			Suggestions:  vmImages,
		},
	}
}

// ParseTargetOptions parses the target options from the JSON string.
func ParseTargetOptions(optionsJson string) (*TargetOptions, error) {
	var targetOptions TargetOptions
	err := json.Unmarshal([]byte(optionsJson), &targetOptions)
	if err != nil {
		return nil, err
	}

	if targetOptions.CredentialFile == "" {
		path, ok := os.LookupEnv("GCP_CREDENTIAL_FILE")
		if ok {
			targetOptions.CredentialFile = path
		}
	}

	if targetOptions.ProjectID == "" {
		projectID, ok := os.LookupEnv("GCP_PROJECT_ID")
		if ok {
			targetOptions.ProjectID = projectID
		}
	}

	if targetOptions.CredentialFile == "" {
		return nil, fmt.Errorf("credential file not set in env/target options")
	}
	if targetOptions.ProjectID == "" {
		return nil, fmt.Errorf("project id not set in env/target options")
	}

	return &targetOptions, nil
}
