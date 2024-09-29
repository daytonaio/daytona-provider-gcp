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
	"github.com/daytonaio/daytona/pkg/ssh"
	"github.com/daytonaio/daytona/pkg/tailscale"
	"github.com/daytonaio/daytona/pkg/workspace/project"
	"tailscale.com/tsnet"

	"github.com/daytonaio/daytona/pkg/logs"
	"github.com/daytonaio/daytona/pkg/provider"
	"github.com/daytonaio/daytona/pkg/provider/util"
	"github.com/daytonaio/daytona/pkg/workspace"
)

type GCPProvider struct {
	BasePath           *string
	DaytonaDownloadUrl *string
	DaytonaVersion     *string
	ServerUrl          *string
	NetworkKey         *string
	ApiUrl             *string
	ApiPort            *uint32
	ServerPort         *uint32
	LogsDir            *string
	tsnetConn          *tsnet.Server
}

func (g *GCPProvider) Initialize(req provider.InitializeProviderRequest) (*util.Empty, error) {
	g.BasePath = &req.BasePath
	g.DaytonaDownloadUrl = &req.DaytonaDownloadUrl
	g.DaytonaVersion = &req.DaytonaVersion
	g.ServerUrl = &req.ServerUrl
	g.NetworkKey = &req.NetworkKey
	g.ApiUrl = &req.ApiUrl
	g.ApiPort = &req.ApiPort
	g.ServerPort = &req.ServerPort
	g.LogsDir = &req.LogsDir

	return new(util.Empty), nil
}

func (g *GCPProvider) GetInfo() (provider.ProviderInfo, error) {
	return provider.ProviderInfo{
		Name:    "gcp-provider",
		Version: internal.Version,
	}, nil
}

func (g *GCPProvider) GetTargetManifest() (*provider.ProviderTargetManifest, error) {
	return types.GetTargetManifest(), nil
}

func (g *GCPProvider) GetDefaultTargets() (*[]provider.ProviderTarget, error) {
	return new([]provider.ProviderTarget), nil
}

func (g *GCPProvider) CreateWorkspace(workspaceReq *provider.WorkspaceRequest) (*util.Empty, error) {
	if g.DaytonaDownloadUrl == nil {
		return nil, errors.New("DaytonaDownloadUrl not set. Did you forget to call Initialize")
	}
	logWriter, cleanupFunc := g.getWorkspaceLogWriter(workspaceReq.Workspace.Id)
	defer cleanupFunc()

	targetOptions, err := types.ParseTargetOptions(workspaceReq.TargetOptions)
	if err != nil {
		logWriter.Write([]byte("Failed to parse target options: " + err.Error() + "\n"))
		return nil, err
	}

	initScript := fmt.Sprintf(`curl -sfL -H "Authorization: Bearer %s" %s | bash`, workspaceReq.Workspace.ApiKey, *g.DaytonaDownloadUrl)
	err = gcputil.CreateWorkspace(workspaceReq.Workspace, targetOptions, initScript, logWriter)
	if err != nil {
		logWriter.Write([]byte("Failed to create workspace: " + err.Error() + "\n"))
		return nil, err
	}

	agentSpinner := logwriters.ShowSpinner(logWriter, "Waiting for the agent to start", "Agent started")
	err = g.waitForDial(workspaceReq.Workspace.Id, 10*time.Minute)
	close(agentSpinner)
	if err != nil {
		logWriter.Write([]byte("Failed to dial: " + err.Error() + "\n"))
		return nil, err
	}

	client, err := g.getDockerClient(workspaceReq.Workspace.Id)
	if err != nil {
		logWriter.Write([]byte("Failed to get client: " + err.Error() + "\n"))
		return nil, err
	}

	workspaceDir := getWorkspaceDir(workspaceReq.Workspace.Id)
	sshClient, err := tailscale.NewSshClient(g.tsnetConn, &ssh.SessionConfig{
		Hostname: workspaceReq.Workspace.Id,
		Port:     config.SSH_PORT,
	})
	if err != nil {
		logWriter.Write([]byte("Failed to create ssh client: " + err.Error() + "\n"))
		return new(util.Empty), err
	}
	defer sshClient.Close()

	return new(util.Empty), client.CreateWorkspace(workspaceReq.Workspace, workspaceDir, logWriter, sshClient)
}

func (g *GCPProvider) StartWorkspace(workspaceReq *provider.WorkspaceRequest) (*util.Empty, error) {
	logWriter, cleanupFunc := g.getWorkspaceLogWriter(workspaceReq.Workspace.Id)
	defer cleanupFunc()

	targetOptions, err := types.ParseTargetOptions(workspaceReq.TargetOptions)
	if err != nil {
		logWriter.Write([]byte("Failed to parse target options: " + err.Error() + "\n"))
		return nil, err
	}

	err = g.waitForDial(workspaceReq.Workspace.Id, 10*time.Minute)
	if err != nil {
		logWriter.Write([]byte("Failed to dial: " + err.Error() + "\n"))
		return nil, err
	}

	return new(util.Empty), gcputil.StartWorkspace(workspaceReq.Workspace, targetOptions)
}

func (g *GCPProvider) StopWorkspace(workspaceReq *provider.WorkspaceRequest) (*util.Empty, error) {
	logWriter, cleanupFunc := g.getWorkspaceLogWriter(workspaceReq.Workspace.Id)
	defer cleanupFunc()

	targetOptions, err := types.ParseTargetOptions(workspaceReq.TargetOptions)
	if err != nil {
		logWriter.Write([]byte("Failed to parse target options: " + err.Error() + "\n"))
		return nil, err
	}

	return new(util.Empty), gcputil.StopWorkspace(workspaceReq.Workspace, targetOptions)
}

func (g *GCPProvider) DestroyWorkspace(workspaceReq *provider.WorkspaceRequest) (*util.Empty, error) {
	logWriter, cleanupFunc := g.getWorkspaceLogWriter(workspaceReq.Workspace.Id)
	defer cleanupFunc()

	targetOptions, err := types.ParseTargetOptions(workspaceReq.TargetOptions)
	if err != nil {
		logWriter.Write([]byte("Failed to parse target options: " + err.Error() + "\n"))
		return nil, err
	}

	return new(util.Empty), gcputil.DeleteWorkspace(workspaceReq.Workspace, targetOptions)
}

func (g *GCPProvider) GetWorkspaceInfo(workspaceReq *provider.WorkspaceRequest) (*workspace.WorkspaceInfo, error) {
	workspaceInfo, err := g.getWorkspaceInfo(workspaceReq)
	if err != nil {
		return nil, err
	}

	var projectInfos []*project.ProjectInfo
	for _, project := range workspaceReq.Workspace.Projects {
		projectInfo, err := g.GetProjectInfo(&provider.ProjectRequest{
			TargetOptions: workspaceReq.TargetOptions,
			Project:       project,
		})
		if err != nil {
			return nil, err
		}
		projectInfos = append(projectInfos, projectInfo)
	}
	workspaceInfo.Projects = projectInfos

	return workspaceInfo, nil
}

func (g *GCPProvider) CreateProject(projectReq *provider.ProjectRequest) (*util.Empty, error) {
	logWriter, cleanupFunc := g.getProjectLogWriter(projectReq.Project.WorkspaceId, projectReq.Project.Name)
	defer cleanupFunc()
	logWriter.Write([]byte("\033[?25h\n"))

	dockerClient, err := g.getDockerClient(projectReq.Project.WorkspaceId)
	if err != nil {
		logWriter.Write([]byte("Failed to get docker client: " + err.Error() + "\n"))
		return nil, err
	}

	sshClient, err := tailscale.NewSshClient(g.tsnetConn, &ssh.SessionConfig{
		Hostname: projectReq.Project.WorkspaceId,
		Port:     config.SSH_PORT,
	})
	if err != nil {
		logWriter.Write([]byte("Failed to create ssh client: " + err.Error() + "\n"))
		return new(util.Empty), err
	}
	defer sshClient.Close()

	return new(util.Empty), dockerClient.CreateProject(&docker.CreateProjectOptions{
		Project:    projectReq.Project,
		ProjectDir: getProjectDir(projectReq),
		Cr:         projectReq.ContainerRegistry,
		LogWriter:  logWriter,
		Gpc:        projectReq.GitProviderConfig,
		SshClient:  sshClient,
	})
}

func (g *GCPProvider) StartProject(projectReq *provider.ProjectRequest) (*util.Empty, error) {
	if g.DaytonaDownloadUrl == nil {
		return nil, errors.New("DaytonaDownloadUrl not set. Did you forget to call Initialize")
	}
	logWriter, cleanupFunc := g.getProjectLogWriter(projectReq.Project.WorkspaceId, projectReq.Project.Name)
	defer cleanupFunc()

	dockerClient, err := g.getDockerClient(projectReq.Project.WorkspaceId)
	if err != nil {
		logWriter.Write([]byte("Failed to get docker client: " + err.Error() + "\n"))
		return nil, err
	}

	sshClient, err := tailscale.NewSshClient(g.tsnetConn, &ssh.SessionConfig{
		Hostname: projectReq.Project.WorkspaceId,
		Port:     config.SSH_PORT,
	})
	if err != nil {
		logWriter.Write([]byte("Failed to create ssh client: " + err.Error() + "\n"))
		return new(util.Empty), err
	}
	defer sshClient.Close()

	return new(util.Empty), dockerClient.StartProject(&docker.CreateProjectOptions{
		Project:    projectReq.Project,
		ProjectDir: getProjectDir(projectReq),
		Cr:         projectReq.ContainerRegistry,
		LogWriter:  logWriter,
		Gpc:        projectReq.GitProviderConfig,
		SshClient:  sshClient,
	}, *g.DaytonaDownloadUrl)
}

func (g *GCPProvider) StopProject(projectReq *provider.ProjectRequest) (*util.Empty, error) {
	logWriter, cleanupFunc := g.getProjectLogWriter(projectReq.Project.WorkspaceId, projectReq.Project.Name)
	defer cleanupFunc()

	dockerClient, err := g.getDockerClient(projectReq.Project.WorkspaceId)
	if err != nil {
		logWriter.Write([]byte("Failed to get docker client: " + err.Error() + "\n"))
		return nil, err
	}

	return new(util.Empty), dockerClient.StopProject(projectReq.Project, logWriter)
}

func (g *GCPProvider) DestroyProject(projectReq *provider.ProjectRequest) (*util.Empty, error) {
	logWriter, cleanupFunc := g.getProjectLogWriter(projectReq.Project.WorkspaceId, projectReq.Project.Name)
	defer cleanupFunc()

	dockerClient, err := g.getDockerClient(projectReq.Project.WorkspaceId)
	if err != nil {
		logWriter.Write([]byte("Failed to get docker client: " + err.Error() + "\n"))
		return nil, err
	}

	sshClient, err := tailscale.NewSshClient(g.tsnetConn, &ssh.SessionConfig{
		Hostname: projectReq.Project.WorkspaceId,
		Port:     config.SSH_PORT,
	})
	if err != nil {
		logWriter.Write([]byte("Failed to create ssh client: " + err.Error() + "\n"))
		return new(util.Empty), err
	}
	defer sshClient.Close()

	return new(util.Empty), dockerClient.DestroyProject(projectReq.Project, getProjectDir(projectReq), sshClient)
}

func (g *GCPProvider) GetProjectInfo(projectReq *provider.ProjectRequest) (*project.ProjectInfo, error) {
	logWriter, cleanupFunc := g.getProjectLogWriter(projectReq.Project.WorkspaceId, projectReq.Project.Name)
	defer cleanupFunc()

	dockerClient, err := g.getDockerClient(projectReq.Project.WorkspaceId)
	if err != nil {
		logWriter.Write([]byte("Failed to get docker client: " + err.Error() + "\n"))
		return nil, err
	}

	return dockerClient.GetProjectInfo(projectReq.Project)
}

func (g *GCPProvider) getWorkspaceInfo(workspaceReq *provider.WorkspaceRequest) (*workspace.WorkspaceInfo, error) {
	logWriter, cleanupFunc := g.getWorkspaceLogWriter(workspaceReq.Workspace.Id)
	defer cleanupFunc()

	targetOptions, err := types.ParseTargetOptions(workspaceReq.TargetOptions)
	if err != nil {
		logWriter.Write([]byte("Failed to parse target options: " + err.Error() + "\n"))
		return nil, err
	}

	vm, err := gcputil.GetComputeInstance(workspaceReq.Workspace, targetOptions)
	if err != nil {
		return nil, err
	}

	metadata := types.ToWorkspaceMetadata(vm)
	jsonMetadata, err := json.Marshal(metadata)
	if err != nil {
		return nil, err
	}

	return &workspace.WorkspaceInfo{
		Name:             workspaceReq.Workspace.Name,
		ProviderMetadata: string(jsonMetadata),
	}, nil
}

func (g *GCPProvider) getWorkspaceLogWriter(workspaceId string) (io.Writer, func()) {
	logWriter := io.MultiWriter(&logwriters.InfoLogWriter{})
	cleanupFunc := func() {}

	if g.LogsDir != nil {
		loggerFactory := logs.NewLoggerFactory(g.LogsDir, nil)
		wsLogWriter := loggerFactory.CreateWorkspaceLogger(workspaceId, logs.LogSourceProvider)
		logWriter = io.MultiWriter(&logwriters.InfoLogWriter{}, wsLogWriter)
		cleanupFunc = func() { wsLogWriter.Close() }
	}

	return logWriter, cleanupFunc
}

func (g *GCPProvider) getProjectLogWriter(workspaceId string, projectName string) (io.Writer, func()) {
	logWriter := io.MultiWriter(&logwriters.InfoLogWriter{})
	cleanupFunc := func() {}

	if g.LogsDir != nil {
		loggerFactory := logs.NewLoggerFactory(g.LogsDir, nil)
		projectLogWriter := loggerFactory.CreateProjectLogger(workspaceId, projectName, logs.LogSourceProvider)
		logWriter = io.MultiWriter(&logwriters.InfoLogWriter{}, projectLogWriter)
		cleanupFunc = func() { projectLogWriter.Close() }
	}

	return logWriter, cleanupFunc
}

func getWorkspaceDir(workspaceId string) string {
	return fmt.Sprintf("/home/daytona/%s", workspaceId)
}

func getProjectDir(projectReq *provider.ProjectRequest) string {
	return path.Join(
		getWorkspaceDir(projectReq.Project.WorkspaceId),
		fmt.Sprintf("%s-%s", projectReq.Project.WorkspaceId, projectReq.Project.Name),
	)
}
