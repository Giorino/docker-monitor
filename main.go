package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"github.com/Giorino/docker-monitor/pkg/container"
	"github.com/Giorino/docker-monitor/pkg/stats"
	"github.com/Giorino/docker-monitor/pkg/utils"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"io"
	"log"
	"os"
	"time"
)

// DONE: Update PrintHelp function.
// DONE: add port to docker run (done with "CMD" field of the ContainerStart).
// DONE: docker run function will be added.
// TODO: Main function will be refactored.
// DONE: list images
// DONE: list stopped containers
// DONE: Docker stop function will be added.
// DONE: Docker start function will be added.
// DONE: Docker remove function will be added.
// DONE: Docker build function will be added with given path of Dockerfile.
// TODO: There is a delay between showing two different docker containers stats. It should be lowered.
// DONE: If the containers CPU usage exceeds 100%, the percentage will be shown red.
// TODO: clearScreen function will be edited. It just scrolls to top of the screen, it doesn't clear the screen.
// FIXED: Containers cannot be found with their short-ids. They can only be found with their names or full-ids.
// DONE: Functions will be distributed to different modules.
// FIXED: Images could not be built.
// FIXED: -v flag will be fixed.

func main() {
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		log.Fatalf("Error creating Docker client: %v", err)
	}
	defer func(cli *client.Client) {
		err := cli.Close()
		if err != nil {
			log.Printf("Error closing Docker client: %v", err)
		}
	}(cli)
	flags := utils.HandleFlagsForManipulation(ctx, cli)

	if len(os.Args) > 2 {
		if os.Args[1] == "build" {
			tag := os.Args[2]
			response, err := container.ImageBuild(ctx, cli, tag, "./Dockerfile")
			if err != nil {
				log.Printf("Error building image: %v", err)
			}
			defer func(Body io.ReadCloser) {
				err = Body.Close()
				if err != nil {
					log.Printf("Error closing build response body: %v", err)
				}
			}(response.Body)

			// Print the build output
			scanner := bufio.NewScanner(response.Body)
			for scanner.Scan() {
				fmt.Println(scanner.Text())
			}

			if err := scanner.Err(); err != nil {
				log.Printf("Error reading build output: %v", err)
			}
			os.Exit(0)
		}
		if os.Args[1] == "run" {
			image := os.Args[2]
			name := os.Args[3]
			err := container.RunContainer(ctx, cli, image, name)
			if err != nil {
				print("Error running container: %v", err)
			}
			os.Exit(0)
		}
		container.DistributeContainerManipulation(os.Args[1], os.Args[2], ctx, cli, os.Args[3])
	} else {
		for {
			var containers []types.Container
			if len(flag.Args()) > 0 {
				containerSpecifier := flag.Args()[0]
				c, err := container.FindContainer(ctx, cli, containerSpecifier)
				if err != nil {
					log.Printf("Error finding container: %v", err)
					continue
				}
				containers = append(containers, c)
			} else {
				listOfContainers, err := cli.ContainerList(ctx, container.ReturnListOptions())
				if err != nil {
					log.Printf("Error listing containers: %v", err)
					continue
				}
				if len(listOfContainers) == 0 {
					log.Printf("No containers found")
					time.Sleep(1 * time.Second)
					continue
				}
				containers = listOfContainers
			}

			// Fetch stats for all containers simultaneously
			s := stats.FetchContainerStatsParallel(ctx, cli, containers)

			// Move cursor and print stats
			utils.ClearScreen(flags[0])
			if !flags[0] {
				utils.PrintHeader("container")
			}

			for _, stat := range s {
				if flags[0] {
					utils.PrintStatsVertically(stat.Container, stat.Stats)
				} else {
					utils.PrintStatsHorizontally(stat.Container, stat.Stats)
				}
			}
			time.Sleep(1 * time.Second)
		}
	}
}
