package main

import (
	"context"
	"io"
	"log"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/namesgenerator"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/docker/go-connections/nat"
)

func GetContext(filePath string) io.Reader {
	ctx, _ := archive.TarWithOptions(filePath, &archive.TarOptions{})
	return ctx
}

func DockerAPI(deployRequest DeployRequest) {
	log.Print("Hello Docker")
	ctx := context.Background()
	dockerClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		log.Fatal(err)
	}
	defer dockerClient.Close()

	imageName := namesgenerator.GetRandomName(0)

	buildImage(dockerClient, deployRequest, imageName)

	startContainer(dockerClient, ctx, deployRequest, imageName)
}

func buildImage(dockerClient *client.Client, deployRequest DeployRequest, imageName string) {
	dockerBuildContext := GetContext("./Dockerfile")
	// docker build --build-arg GIT_URL=https://github.com/theankitbhardwaj/latest-wayback-snapshot-redis.git --build-arg BUILD_CMD="go build -tags netgo -ldflags '-s -w' -o myService" --build-arg START_CMD="./myService" -t go-webservice .
	buildArgs := make(map[string]*string)

	buildArgs["GIT_URL"] = deployRequest.Git_URL
	buildArgs["PORT"] = deployRequest.Port
	buildArgs["BUILD_CMD"] = deployRequest.Build_CMD
	buildArgs["START_CMD"] = deployRequest.Start_CMD
	buildOptions := types.ImageBuildOptions{
		Tags:      []string{imageName},
		Remove:    true,
		BuildArgs: buildArgs,
		NoCache:   true,
	}
	buildResponse, err := dockerClient.ImageBuild(context.Background(), dockerBuildContext, buildOptions)
	if err != nil {
		log.Fatal(err)
	}
	io.Copy(os.Stdout, buildResponse.Body)

	defer buildResponse.Body.Close()
}

func startContainer(dockerClient *client.Client, ctx context.Context, deployRequest DeployRequest, imageName string) {
	portBindings := make(nat.PortMap)
	var sb strings.Builder
	if deployRequest.Port != nil {
		sb.WriteString(*deployRequest.Port)
		sb.WriteString("/tcp")
	} else {
		sb.WriteString("8080/tcp")
	}

	containerPort := sb.String()

	hostPort, err := GetFreePort()

	if err != nil {
		log.Fatal(err)
	}

	bindings := []nat.PortBinding{
		{HostIP: "", HostPort: hostPort}, //TODO: Get this host port dynamically
	}
	portBindings[nat.Port(containerPort)] = bindings

	resp, err := dockerClient.ContainerCreate(ctx, &container.Config{
		Image: imageName,
	}, &container.HostConfig{
		PortBindings: portBindings,
		NetworkMode:  "bridge",
	}, nil, nil, "")

	if err != nil {
		log.Fatal(err)
	}

	if err := dockerClient.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		log.Fatal(err)
	}

	statusCh, errCh := dockerClient.ContainerWait(ctx, resp.ID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			log.Fatal(err)
		}
	case <-statusCh:
	}

	out, err1 := dockerClient.ContainerLogs(ctx, resp.ID, container.LogsOptions{ShowStdout: true})
	if err1 != nil {
		log.Fatal(err1)
	}

	stdcopy.StdCopy(os.Stdout, os.Stderr, out)
}

// GetFreePort asks the kernel for a free open port that is ready to use.
func GetFreePort() (port string, err error) {
	var a *net.TCPAddr
	if a, err = net.ResolveTCPAddr("tcp", "localhost:0"); err == nil {
		var l *net.TCPListener
		if l, err = net.ListenTCP("tcp", a); err == nil {
			defer l.Close()
			return strconv.Itoa(l.Addr().(*net.TCPAddr).Port), nil
		}
	}
	return
}
