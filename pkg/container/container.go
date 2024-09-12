package container

import (
	"context"
	"fmt"
	"github.com/Giorino/docker-monitor/pkg/utils"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"strings"
)

func FindContainer(ctx context.Context, cli *client.Client, containerSpecifier string) (types.Container, error) {
	containers, err := cli.ContainerList(ctx, container.ListOptions{
		All: true,
	})
	if err != nil {
		return types.Container{}, err
	}

	for _, c := range containers {
		// Check if the containerSpecifier matches the start of the container ID
		if strings.HasPrefix(c.ID, containerSpecifier) {
			return c, nil
		}

		// Check if the containerSpecifier matches the container name (including the leading '/')
		for _, name := range c.Names {
			if name == containerSpecifier || name == "/"+containerSpecifier {
				return c, nil
			}
		}
	}

	return types.Container{}, fmt.Errorf("container not found: %s", containerSpecifier)
}

func ReturnListOptions() container.ListOptions {
	return container.ListOptions{}
}

func StartContainer(ctx context.Context, cli *client.Client, containerID string) error {
	return cli.ContainerStart(ctx, containerID, container.StartOptions{})

}

func StopContainer(ctx context.Context, cli *client.Client, containerID string) error {
	return cli.ContainerStop(ctx, containerID, container.StopOptions{})
}

func RemoveContainer(ctx context.Context, cli *client.Client, containerID string) error {
	return cli.ContainerRemove(ctx, containerID, container.RemoveOptions{})
}

func ImageBuild(ctx context.Context, cli *client.Client, tag string, dockerfilePath string) (types.ImageBuildResponse, error) {
	tarReader := utils.TarDirectory(dockerfilePath)
	if tarReader == nil {
		return types.ImageBuildResponse{}, fmt.Errorf("failed to create tar archive")
	}

	return cli.ImageBuild(ctx, tarReader, types.ImageBuildOptions{
		Tags:       []string{tag},
		Dockerfile: "Dockerfile", // This should match the name we used in TarDirectory
	})
}

func DistributeContainerManipulation(command string, flag string, ctx context.Context, cli *client.Client, containerSpecifier string) {
	if flag == "--id" {
		GetGivenManipulation(ctx, cli, command, containerSpecifier)
	} else if flag == "--name" {
		c, _ := FindContainer(ctx, cli, containerSpecifier)
		GetGivenManipulation(ctx, cli, command, c.ID)
	} else {
		fmt.Print("Error manipulating container, please give --id or --name flag")
	}
}

func GetGivenManipulation(ctx context.Context, cli *client.Client, command string, containerID string) {
	switch command {
	case "start":
		err := StartContainer(ctx, cli, containerID)
		if err != nil {
			fmt.Printf("Error starting container: %v", err)
		}
	case "stop":
		err := StopContainer(ctx, cli, containerID)
		if err != nil {
			fmt.Printf("Error stopping container: %v", err)
		}
	case "remove":
		err := RemoveContainer(ctx, cli, containerID)
		if err != nil {
			fmt.Printf("Error removing container: %v", err)
		}
	default:
		fmt.Print("Error manipulating container, please give start, stop or remove command")
	}
}

func RunContainer(ctx context.Context, cli *client.Client, image string, name string) error {
	platform := utils.GetDockerPlatform()
	_, err := cli.ContainerCreate(ctx, &container.Config{
		Image: image,
		Cmd:   []string{"/bin/sh", "-c", "while :; do sleep 1; done"},
	}, &container.HostConfig{}, nil, platform, name)
	if err != nil {
		return err
	}
	c, _ := FindContainer(ctx, cli, name)
	err = StartContainer(ctx, cli, c.ID)
	return err
}
