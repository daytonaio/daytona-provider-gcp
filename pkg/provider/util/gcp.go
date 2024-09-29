package util

import (
	"context"
	"fmt"
	"io"

	compute "cloud.google.com/go/compute/apiv1"
	"cloud.google.com/go/compute/apiv1/computepb"
	logwriters "github.com/daytonaio/daytona-provider-gcp/internal/log"
	"github.com/daytonaio/daytona-provider-gcp/pkg/types"
	"github.com/daytonaio/daytona/pkg/workspace"
	"google.golang.org/api/option"
)

func CreateWorkspace(workspace *workspace.Workspace, opts *types.TargetOptions, initScript string, logWriter io.Writer) error {
	envVars := workspace.EnvVars
	envVars["DAYTONA_AGENT_LOG_FILE_PATH"] = "/home/daytona/.daytona-agent.log"

	customData := `#!/bin/bash
useradd -m -d /home/daytona daytona

curl -fsSL https://get.docker.com | bash

# Modify Docker daemon configuration
cat > /etc/docker/daemon.json <<EOF
{
  "hosts": ["unix:///var/run/docker.sock", "tcp://0.0.0.0:2375"]
}
EOF

# Create a systemd drop-in file to modify the Docker service
mkdir -p /etc/systemd/system/docker.service.d
cat > /etc/systemd/system/docker.service.d/override.conf <<EOF
[Service]
ExecStart=
ExecStart=/usr/bin/dockerd
EOF

systemctl daemon-reload
systemctl restart docker
systemctl start docker

usermod -aG docker daytona

if grep -q sudo /etc/group; then
	usermod -aG sudo,docker daytona
elif grep -q wheel /etc/group; then
	usermod -aG wheel,docker daytona
fi

echo "daytona ALL=(ALL) NOPASSWD:ALL" > /etc/sudoers.d/91-daytona

`

	for k, v := range envVars {
		customData += fmt.Sprintf("export %s=%s\n", k, v)
	}
	customData += initScript
	customData += `
echo '[Unit]
Description=Daytona Agent Service
After=network.target

[Service]
User=daytona
ExecStart=/usr/local/bin/daytona agent --host
Restart=always
`

	for k, v := range envVars {
		customData += fmt.Sprintf("Environment='%s=%s'\n", k, v)
	}

	customData += `
[Install]
WantedBy=multi-user.target' > /etc/systemd/system/daytona-agent.service
systemctl daemon-reload
systemctl enable daytona-agent.service
systemctl start daytona-agent.service
`
	return createComputeInstance(workspace.Id, customData, opts, logWriter)
}

func StartWorkspace(workspace *workspace.Workspace, opts *types.TargetOptions) error {
	client, err := compute.NewInstancesRESTClient(context.Background(), option.WithCredentialsFile(opts.CredentialFile))
	if err != nil {
		return err
	}
	defer client.Close()

	op, err := client.Start(context.Background(), &computepb.StartInstanceRequest{
		Project:  opts.ProjectID,
		Zone:     opts.Zone,
		Instance: getResourceName(workspace.Id),
	})
	if err != nil {
		return err
	}

	return op.Wait(context.Background())
}

func StopWorkspace(workspace *workspace.Workspace, opts *types.TargetOptions) error {
	client, err := compute.NewInstancesRESTClient(context.Background(), option.WithCredentialsFile(opts.CredentialFile))
	if err != nil {
		return err
	}
	defer client.Close()

	op, err := client.Stop(context.Background(), &computepb.StopInstanceRequest{
		Project:  opts.ProjectID,
		Zone:     opts.Zone,
		Instance: getResourceName(workspace.Id),
	})
	if err != nil {
		return err
	}

	return op.Wait(context.Background())
}

func DeleteWorkspace(workspace *workspace.Workspace, opts *types.TargetOptions) error {
	client, err := compute.NewInstancesRESTClient(context.Background(), option.WithCredentialsFile(opts.CredentialFile))
	if err != nil {
		return err
	}
	defer client.Close()

	op, err := client.Delete(context.Background(), &computepb.DeleteInstanceRequest{
		Project:  opts.ProjectID,
		Zone:     opts.Zone,
		Instance: getResourceName(workspace.Id),
	})
	if err != nil {
		return err
	}

	return op.Wait(context.Background())
}

func createComputeInstance(workspaceId string, initScript string, opts *types.TargetOptions, logWriter io.Writer) error {
	instancesClient, err := compute.NewInstancesRESTClient(context.Background(), option.WithCredentialsFile(opts.CredentialFile))
	if err != nil {
		return err
	}
	defer instancesClient.Close()

	instanceName := getResourceName(workspaceId)
	machineType := fmt.Sprintf("zones/%s/machineTypes/%s", opts.Zone, opts.MachineType)
	diskType := fmt.Sprintf("projects/%s/zones/%s/diskTypes/%s", opts.ProjectID, opts.Zone, opts.DiskType)

	spinner := logwriters.ShowSpinner(logWriter, "Creating GCP virtual machine", "GCP virtual machine created")
	operation, err := instancesClient.Insert(context.Background(), &computepb.InsertInstanceRequest{
		Project: opts.ProjectID,
		Zone:    opts.Zone,
		InstanceResource: &computepb.Instance{
			Name:        toPtr(instanceName),
			MachineType: toPtr(machineType),
			Disks: []*computepb.AttachedDisk{
				{
					AutoDelete: toPtr(true),
					Boot:       toPtr(true),
					Type:       toPtr(computepb.AttachedDisk_PERSISTENT.String()),
					InitializeParams: &computepb.AttachedDiskInitializeParams{
						DiskType:    toPtr(diskType),
						SourceImage: toPtr(opts.VMImage),
						DiskSizeGb:  toPtr(int64(opts.DiskSize)),
					},
				},
			},
			NetworkInterfaces: []*computepb.NetworkInterface{
				{
					Name: toPtr("global/networks/default"),
					AccessConfigs: []*computepb.AccessConfig{
						{
							Name: toPtr("External NAT"),
							Type: toPtr(computepb.AccessConfig_ONE_TO_ONE_NAT.String()),
						},
					},
				},
			},
			Metadata: &computepb.Metadata{
				Items: []*computepb.Items{
					{
						Key:   toPtr("startup-script"),
						Value: &initScript,
					},
				},
			},
		},
	})
	if err != nil {
		close(spinner)
		return err
	}
	defer func() { close(spinner) }()

	return operation.Wait(context.Background())
}

func GetComputeInstance(workspace *workspace.Workspace, opts *types.TargetOptions) (*computepb.Instance, error) {
	client, err := compute.NewInstancesRESTClient(context.Background(), option.WithCredentialsFile(opts.CredentialFile))
	if err != nil {
		return nil, err
	}
	defer client.Close()

	return client.Get(context.Background(), &computepb.GetInstanceRequest{
		Project:  opts.ProjectID,
		Zone:     opts.Zone,
		Instance: getResourceName(workspace.Id),
	})
}

func getResourceName(identifier string) string {
	return fmt.Sprintf("daytona-%s", identifier)
}

func toPtr[T any](v T) *T {
	return &v
}
