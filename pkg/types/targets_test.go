package types

import (
	"reflect"
	"testing"
)

func TestGetTargetManifest(t *testing.T) {
	targetManifest := GetTargetManifest()
	if targetManifest == nil {
		t.Fatalf("Expected target manifest but got nil")
	}

	fields := [7]string{"Credential File", "Project Id", "Zone", "Machine Type", "Disk Type", "Disk Size", "VM Image"}
	for _, field := range fields {
		if _, ok := (*targetManifest)[field]; !ok {
			t.Errorf("Expected field %s in target manifest but it was not found", field)
		}
	}
}

func TestParseTargetOptions(t *testing.T) {
	tests := []struct {
		name        string
		optionsJson string
		envVars     map[string]string
		want        *TargetOptions
		wantErr     bool
	}{
		{
			name: "Valid JSON with all fields",
			optionsJson: `{
				"Credential File": "/path/to/cred.json",
				"PROJECT Id": "my-project",
				"Zone": "us-central1-a",
				"Machine Type": "n1-standard-1",
				"Disk Type": "pd-standard",
				"Disk Size": 30,
				"VM Image": "projects/ubuntu-os-cloud/global/images/family/ubuntu-2204-lts"
			}`,
			want: &TargetOptions{
				CredentialFile: "/path/to/cred.json",
				ProjectID:      "my-project",
				Zone:           "us-central1-a",
				MachineType:    "n1-standard-1",
				DiskType:       "pd-standard",
				DiskSize:       30,
				VMImage:        "projects/ubuntu-os-cloud/global/images/family/ubuntu-2204-lts",
			},
			wantErr: false,
		},
		{
			name: "Valid JSON with missing fields, using env vars",
			optionsJson: `{
				"Zone": "us-central1-a",
				"Machine Type": "n1-standard-1",
				"Disk Type": "pd-standard",
				"Disk Size": 30,
				"VM Image": "projects/ubuntu-os-cloud/global/images/family/ubuntu-2204-lts"
			}`,
			envVars: map[string]string{
				"GCP_CREDENTIAL_FILE": "/env/path/to/cred.json",
				"GCP_PROJECT_ID":      "env-project",
			},
			want: &TargetOptions{
				CredentialFile: "/env/path/to/cred.json",
				ProjectID:      "env-project",
				Zone:           "us-central1-a",
				MachineType:    "n1-standard-1",
				DiskType:       "pd-standard",
				DiskSize:       30,
				VMImage:        "projects/ubuntu-os-cloud/global/images/family/ubuntu-2204-lts",
			},
			wantErr: false,
		},
		{
			name:        "Invalid JSON",
			optionsJson: `{"Credential File": "/path/to/cred.json", "PROJECT Id": "my-project"`,
			wantErr:     true,
		},
		{
			name: "Missing all required fields in both JSON and env vars",
			optionsJson: `{
				"Zone": "us-central1-a"
			}`,
			wantErr: true,
		},
		{
			name:        "Empty JSON",
			optionsJson: `{}`,
			envVars: map[string]string{
				"GCP_CREDENTIAL_FILE": "/env/path/to/cred.json",
				"GCP_PROJECT_ID":      "env-project",
			},
			want: &TargetOptions{
				CredentialFile: "/env/path/to/cred.json",
				ProjectID:      "env-project",
			},
			wantErr: false,
		},
		{
			name: "Partial JSON with some valid env vars",
			optionsJson: `{
				"Credential File": "/path/to/cred.json"
			}`,
			envVars: map[string]string{
				"GCP_PROJECT_ID": "env-project",
			},
			want: &TargetOptions{
				CredentialFile: "/path/to/cred.json",
				ProjectID:      "env-project",
			},
			wantErr: false,
		},
		{
			name: "JSON with additional non-required fields",
			optionsJson: `{
				"Credential File": "/path/to/cred.json",
				"PROJECT Id": "my-project",
				"Zone": "us-central1-a",
				"Machine Type": "n1-standard-1",
				"Disk Type": "pd-standard",
				"Disk Size": 30,
				"VM Image": "projects/ubuntu-os-cloud/global/images/family/ubuntu-2204-lts",
				"ExtraField": "extra-value"
			}`,
			want: &TargetOptions{
				CredentialFile: "/path/to/cred.json",
				ProjectID:      "my-project",
				Zone:           "us-central1-a",
				MachineType:    "n1-standard-1",
				DiskType:       "pd-standard",
				DiskSize:       30,
				VMImage:        "projects/ubuntu-os-cloud/global/images/family/ubuntu-2204-lts",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.envVars {
				t.Setenv(k, v)
			}

			got, err := ParseTargetOptions(tt.optionsJson)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseTargetOptions() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseTargetOptions() = %v, want %v", got, tt.want)
			}
		})
	}
}
