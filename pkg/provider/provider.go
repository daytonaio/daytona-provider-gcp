package provider

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"time"

	"github.com/daytonaio/daytona-provider-gcp/internal"
	logwriters "github.com/daytonaio/daytona-provider-gcp/internal/log"
	gcputil "github.com/daytonaio/daytona-provider-gcp/pkg/provider/util"
	"github.com/daytonaio/daytona-provider-gcp/pkg/types"
	"github.com/daytonaio/daytona/pkg/agent/ssh/config"
	"github.com/daytonaio/daytona/pkg/docker"
	"github.com/daytonaio/daytona/pkg/models"
	"github.com/daytonaio/daytona/pkg/ssh"
	"github.com/daytonaio/daytona/pkg/tailscale"
	"tailscale.com/tsnet"

	"github.com/daytonaio/daytona/pkg/logs"
	"github.com/daytonaio/daytona/pkg/provider"
	"github.com/daytonaio/daytona/pkg/provider/util"
)

type GCPProvider struct {
	BasePath           *string
	DaytonaDownloadUrl *string
	DaytonaVersion     *string
	ServerUrl          *string
	NetworkKey         *string
	ApiUrl             *string
	ApiKey             *string
	ApiPort            *uint32
	ServerPort         *uint32
	TargetLogsDir      *string
	WorkspaceLogsDir   *string
	tsnetConn          *tsnet.Server
}

func (g *GCPProvider) Initialize(req provider.InitializeProviderRequest) (*util.Empty, error) {
	g.BasePath = &req.BasePath
	g.DaytonaDownloadUrl = &req.DaytonaDownloadUrl
	g.DaytonaVersion = &req.DaytonaVersion
	g.ServerUrl = &req.ServerUrl
	g.NetworkKey = &req.NetworkKey
	g.ApiUrl = &req.ApiUrl
	g.ApiKey = req.ApiKey
	g.ApiPort = &req.ApiPort
	g.ServerPort = &req.ServerPort
	g.TargetLogsDir = &req.TargetLogsDir
	g.WorkspaceLogsDir = &req.WorkspaceLogsDir

	return new(util.Empty), nil
}

func (g *GCPProvider) GetInfo() (models.ProviderInfo, error) {
	label := "GCP"

	return models.ProviderInfo{
		Label:                &label,
		Name:                 "gcp-provider",
		Version:              internal.Version,
		TargetConfigManifest: *types.GetTargetConfigManifest(),
	}, nil
}

func (g *GCPProvider) GetPresetTargetConfigs() (*[]provider.TargetConfig, error) {
	return new([]provider.TargetConfig), nil
}

func (g *GCPProvider) CreateTarget(targetReq *provider.TargetRequest) (*util.Empty, error) {
	if g.DaytonaDownloadUrl == nil {
		return nil, errors.New("DaytonaDownloadUrl not set. Did you forget to call Initialize")
	}
	logWriter, cleanupFunc := g.getTargetLogWriter(targetReq.Target.Id, targetReq.Target.Name)
	defer cleanupFunc()

	targetOptions, err := types.ParseTargetOptions(targetReq.Target.TargetConfig.Options)
	if err != nil {
		logWriter.Write([]byte("Failed to parse target options: " + err.Error() + "\n"))
		return nil, err
	}

	initScript := fmt.Sprintf(`curl -sfL -H "Authorization: Bearer %s" %s | bash`, targetReq.Target.ApiKey, *g.DaytonaDownloadUrl)
	err = gcputil.CreateTarget(targetReq.Target, targetOptions, initScript, logWriter)
	if err != nil {
		logWriter.Write([]byte("Failed to create target: " + err.Error() + "\n"))
		return nil, err
	}

	agentSpinner := logwriters.ShowSpinner(logWriter, "Waiting for the agent to start", "Agent started")
	err = g.waitForDial(targetReq.Target.Id, 10*time.Minute)
	close(agentSpinner)
	if err != nil {
		logWriter.Write([]byte("Failed to dial: " + err.Error() + "\n"))
		return nil, err
	}

	client, err := g.getDockerClient(targetReq.Target.Id)
	if err != nil {
		logWriter.Write([]byte("Failed to get client: " + err.Error() + "\n"))
		return nil, err
	}

	targetDir := getTargetDir(targetReq.Target.Id)
	sshClient, err := tailscale.NewSshClient(g.tsnetConn, &ssh.SessionConfig{
		Hostname: targetReq.Target.Id,
		Port:     config.SSH_PORT,
	})
	if err != nil {
		logWriter.Write([]byte("Failed to create ssh client: " + err.Error() + "\n"))
		return new(util.Empty), err
	}
	defer sshClient.Close()

	return new(util.Empty), client.CreateTarget(targetReq.Target, targetDir, logWriter, sshClient)
}

func (g *GCPProvider) StartTarget(targetReq *provider.TargetRequest) (*util.Empty, error) {
	logWriter, cleanupFunc := g.getTargetLogWriter(targetReq.Target.Id, targetReq.Target.Name)
	defer cleanupFunc()

	targetOptions, err := types.ParseTargetOptions(targetReq.Target.TargetConfig.Options)
	if err != nil {
		logWriter.Write([]byte("Failed to parse target options: " + err.Error() + "\n"))
		return nil, err
	}

	err = g.waitForDial(targetReq.Target.Id, 10*time.Minute)
	if err != nil {
		logWriter.Write([]byte("Failed to dial: " + err.Error() + "\n"))
		return nil, err
	}

	return new(util.Empty), gcputil.StartTarget(targetReq.Target, targetOptions)
}

func (g *GCPProvider) StopTarget(targetReq *provider.TargetRequest) (*util.Empty, error) {
	logWriter, cleanupFunc := g.getTargetLogWriter(targetReq.Target.Id, targetReq.Target.Name)
	defer cleanupFunc()

	targetOptions, err := types.ParseTargetOptions(targetReq.Target.TargetConfig.Options)
	if err != nil {
		logWriter.Write([]byte("Failed to parse target options: " + err.Error() + "\n"))
		return nil, err
	}

	return new(util.Empty), gcputil.StopTarget(targetReq.Target, targetOptions)
}

func (g *GCPProvider) DestroyTarget(targetReq *provider.TargetRequest) (*util.Empty, error) {
	logWriter, cleanupFunc := g.getTargetLogWriter(targetReq.Target.Id, targetReq.Target.Name)
	defer cleanupFunc()

	targetOptions, err := types.ParseTargetOptions(targetReq.Target.TargetConfig.Options)
	if err != nil {
		logWriter.Write([]byte("Failed to parse target options: " + err.Error() + "\n"))
		return nil, err
	}

	return new(util.Empty), gcputil.DeleteTarget(targetReq.Target, targetOptions)
}

func (g *GCPProvider) CreateWorkspace(workspaceReq *provider.WorkspaceRequest) (*util.Empty, error) {
	logWriter, cleanupFunc := g.getWorkspaceLogWriter(workspaceReq.Workspace.Id, workspaceReq.Workspace.Name)
	defer cleanupFunc()
	logWriter.Write([]byte("\033[?25h\n"))

	dockerClient, err := g.getDockerClient(workspaceReq.Workspace.TargetId)
	if err != nil {
		logWriter.Write([]byte("Failed to get docker client: " + err.Error() + "\n"))
		return nil, err
	}

	sshClient, err := tailscale.NewSshClient(g.tsnetConn, &ssh.SessionConfig{
		Hostname: workspaceReq.Workspace.TargetId,
		Port:     config.SSH_PORT,
	})
	if err != nil {
		logWriter.Write([]byte("Failed to create ssh client: " + err.Error() + "\n"))
		return new(util.Empty), err
	}
	defer sshClient.Close()

	return new(util.Empty), dockerClient.CreateWorkspace(&docker.CreateWorkspaceOptions{
		Workspace:           workspaceReq.Workspace,
		WorkspaceDir:        getWorkspaceDir(workspaceReq),
		ContainerRegistries: workspaceReq.ContainerRegistries,
		BuilderImage:        workspaceReq.BuilderImage,
		LogWriter:           logWriter,
		Gpc:                 workspaceReq.GitProviderConfig,
		SshClient:           sshClient,
	})
}

func (g *GCPProvider) StartWorkspace(workspaceReq *provider.WorkspaceRequest) (*util.Empty, error) {
	if g.DaytonaDownloadUrl == nil {
		return nil, errors.New("DaytonaDownloadUrl not set. Did you forget to call Initialize")
	}
	logWriter, cleanupFunc := g.getWorkspaceLogWriter(workspaceReq.Workspace.Id, workspaceReq.Workspace.Name)
	defer cleanupFunc()

	dockerClient, err := g.getDockerClient(workspaceReq.Workspace.TargetId)
	if err != nil {
		logWriter.Write([]byte("Failed to get docker client: " + err.Error() + "\n"))
		return nil, err
	}

	sshClient, err := tailscale.NewSshClient(g.tsnetConn, &ssh.SessionConfig{
		Hostname: workspaceReq.Workspace.TargetId,
		Port:     config.SSH_PORT,
	})
	if err != nil {
		logWriter.Write([]byte("Failed to create ssh client: " + err.Error() + "\n"))
		return new(util.Empty), err
	}
	defer sshClient.Close()

	return new(util.Empty), dockerClient.StartWorkspace(&docker.CreateWorkspaceOptions{
		Workspace:           workspaceReq.Workspace,
		WorkspaceDir:        getWorkspaceDir(workspaceReq),
		ContainerRegistries: workspaceReq.ContainerRegistries,
		BuilderImage:        workspaceReq.BuilderImage,
		LogWriter:           logWriter,
		Gpc:                 workspaceReq.GitProviderConfig,
		SshClient:           sshClient,
	}, *g.DaytonaDownloadUrl)
}

func (g *GCPProvider) StopWorkspace(workspaceReq *provider.WorkspaceRequest) (*util.Empty, error) {
	logWriter, cleanupFunc := g.getWorkspaceLogWriter(workspaceReq.Workspace.Id, workspaceReq.Workspace.Name)
	defer cleanupFunc()

	dockerClient, err := g.getDockerClient(workspaceReq.Workspace.TargetId)
	if err != nil {
		logWriter.Write([]byte("Failed to get docker client: " + err.Error() + "\n"))
		return nil, err
	}

	return new(util.Empty), dockerClient.StopWorkspace(workspaceReq.Workspace, logWriter)
}

func (g *GCPProvider) DestroyWorkspace(workspaceReq *provider.WorkspaceRequest) (*util.Empty, error) {
	logWriter, cleanupFunc := g.getWorkspaceLogWriter(workspaceReq.Workspace.Id, workspaceReq.Workspace.Name)
	defer cleanupFunc()

	dockerClient, err := g.getDockerClient(workspaceReq.Workspace.TargetId)
	if err != nil {
		logWriter.Write([]byte("Failed to get docker client: " + err.Error() + "\n"))
		return nil, err
	}

	sshClient, err := tailscale.NewSshClient(g.tsnetConn, &ssh.SessionConfig{
		Hostname: workspaceReq.Workspace.TargetId,
		Port:     config.SSH_PORT,
	})
	if err != nil {
		logWriter.Write([]byte("Failed to create ssh client: " + err.Error() + "\n"))
		return new(util.Empty), err
	}
	defer sshClient.Close()

	return new(util.Empty), dockerClient.DestroyWorkspace(workspaceReq.Workspace, getWorkspaceDir(workspaceReq), sshClient)
}

func (g *GCPProvider) GetWorkspaceProviderMetadata(workspaceReq *provider.WorkspaceRequest) (string, error) {
	logWriter, cleanupFunc := g.getWorkspaceLogWriter(workspaceReq.Workspace.Id, workspaceReq.Workspace.Name)
	defer cleanupFunc()

	dockerClient, err := g.getDockerClient(workspaceReq.Workspace.TargetId)
	if err != nil {
		logWriter.Write([]byte("Failed to get docker client: " + err.Error() + "\n"))
		return "", err
	}

	return dockerClient.GetWorkspaceProviderMetadata(workspaceReq.Workspace)
}

func (g *GCPProvider) GetTargetProviderMetadata(targetReq *provider.TargetRequest) (string, error) {
	logWriter, cleanupFunc := g.getTargetLogWriter(targetReq.Target.Id, targetReq.Target.Name)
	defer cleanupFunc()

	targetOptions, err := types.ParseTargetOptions(targetReq.Target.TargetConfig.Options)
	if err != nil {
		logWriter.Write([]byte("Failed to parse target options: " + err.Error() + "\n"))
		return "", err
	}

	vm, err := gcputil.GetComputeInstance(targetReq.Target, targetOptions)
	if err != nil {
		return "", err
	}

	metadata := types.ToTargetMetadata(vm)

	jsonMetadata, err := json.Marshal(metadata)
	if err != nil {
		return "", err
	}

	return string(jsonMetadata), nil
}

func (g *GCPProvider) getTargetLogWriter(targetId, targetName string) (io.Writer, func()) {
	logWriter := io.MultiWriter(&logwriters.InfoLogWriter{})
	cleanupFunc := func() {}

	if g.TargetLogsDir != nil {
		loggerFactory := logs.NewLoggerFactory(logs.LoggerFactoryConfig{
			LogsDir:     *g.TargetLogsDir,
			ApiUrl:      g.ApiUrl,
			ApiKey:      g.ApiKey,
			ApiBasePath: &logs.ApiBasePathTarget,
		})
		targetLogWriter, err := loggerFactory.CreateLogger(targetId, targetName, logs.LogSourceProvider)
		if err == nil {
			logWriter = io.MultiWriter(&logwriters.InfoLogWriter{}, targetLogWriter)
			cleanupFunc = func() { targetLogWriter.Close() }
		}
	}

	return logWriter, cleanupFunc
}

func (g *GCPProvider) getWorkspaceLogWriter(workspaceId, workspaceName string) (io.Writer, func()) {
	logWriter := io.MultiWriter(&logwriters.InfoLogWriter{})
	cleanupFunc := func() {}

	if g.WorkspaceLogsDir != nil {
		loggerFactory := logs.NewLoggerFactory(logs.LoggerFactoryConfig{
			LogsDir:     *g.WorkspaceLogsDir,
			ApiUrl:      g.ApiUrl,
			ApiKey:      g.ApiKey,
			ApiBasePath: &logs.ApiBasePathWorkspace,
		})
		workspaceLogWriter, err := loggerFactory.CreateLogger(workspaceId, workspaceName, logs.LogSourceProvider)
		if err == nil {
			logWriter = io.MultiWriter(&logwriters.InfoLogWriter{}, workspaceLogWriter)
			cleanupFunc = func() { workspaceLogWriter.Close() }
		}
	}

	return logWriter, cleanupFunc
}

func (g *GCPProvider) CheckRequirements() (*[]provider.RequirementStatus, error) {
	results := []provider.RequirementStatus{}
	return &results, nil
}

func getTargetDir(targetId string) string {
	return fmt.Sprintf("/home/daytona/%s", targetId)
}

func getWorkspaceDir(workspaceReq *provider.WorkspaceRequest) string {
	return path.Join(
		getTargetDir(workspaceReq.Workspace.TargetId),
		fmt.Sprintf("%s-%s", workspaceReq.Workspace.TargetId, workspaceReq.Workspace.Name),
	)
}
