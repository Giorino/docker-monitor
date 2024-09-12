package utils

import (
	"archive/tar"
	"bytes"
	"context"
	"flag"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/fatih/color"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

func PrintHeader(action string) {
	switch action {
	case "container":
		fmt.Printf("%-46s | %7s | %26s | %12s\n", "CONTAINER ID / NAME", "CPU %", "MEM USAGE / LIMIT", "NET I/O")
		fmt.Println(strings.Repeat("-", 100)) // Print a separator line
	case "image":
		fmt.Printf("%-40s %-20s %-15s\n", "REPOSITORY", "TAG", "IMAGE ID")
		fmt.Println(strings.Repeat("-", 100)) // Print a separator line
	}

}

func PrintHelp() {
	fmt.Println("Usage: docker-monitor [OPTIONS] [CONTAINER]")
	fmt.Println("Display a live stream of container(s) resource usage statistics")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  -v, --vertical   Print stats vertically")
	fmt.Println("  -h, --help       Print help message")
	fmt.Println("  -s, --stopped    List stopped containers")
	fmt.Println("  -i, --images     List images")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  docker-monitor")
	fmt.Println("  docker-monitor my_container")
	fmt.Println("  docker-monitor $[container_id]")
	fmt.Println("  docker-monitor -v")
	fmt.Println("  docker-monitor -v my_container")
	fmt.Println("  docker-monitor -s")
	fmt.Println("  docker-monitor -i")
	fmt.Println()
	fmt.Println("To start, stop or remove a container:")
	fmt.Println("  docker-monitor start --id $[container_id]")
	fmt.Println("  docker-monitor stop --id $[container_id]")
	fmt.Println("  docker-monitor remove --id $[container_id]")
	fmt.Println("They could be used with --name flag as well.")
	fmt.Println()
	fmt.Println("To run a container:")
	fmt.Println("  docker-monitor run $[image_name] $[container_name]")
	fmt.Println()
	fmt.Println("To build an image:")
	fmt.Println("  docker-monitor build $[image_tag]")
}

func ClearScreen(verticalFlag bool) {
	if !verticalFlag {
		// Move cursor to second line (preserving header)
		fmt.Print("\033[2;1H")
		// Clear from cursor to end of screen
		fmt.Print("\033[J")
	} else {
		// Clear the entire screen
		fmt.Print("\033[2J")
		// Move cursor to top-left corner
		fmt.Print("\033[H")
	}
}

func PrintStatsVertically(c types.Container, stats struct {
	CPUPercentage float64
	MemoryUsage   float64
	MemoryLimit   float64
	NetworkRx     float64
	NetworkTx     float64
}) {
	red := color.New(color.FgRed).Add(color.Bold).SprintFunc()
	green := color.New(color.FgGreen).Add(color.Bold).SprintFunc()

	fmt.Printf(red("Container: ")+"%s\n", c.Names[0])
	fmt.Printf(green("  CPU: ")+"%.2f%%\n", stats.CPUPercentage)
	fmt.Printf(green("  Memory: ")+"%.2f MB / %.2f MB\n", stats.MemoryUsage, stats.MemoryLimit)
	fmt.Printf(green("  Network I/O: ")+"%.2f MB / %.2f MB\n", stats.NetworkRx, stats.NetworkTx)
	fmt.Println()
}

func ListStoppedContainers(ctx context.Context, cli *client.Client) {
	containers, err := cli.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		fmt.Printf("Error listing containers: %v", err)
	}
	if IsThereStoppedContainers(ctx, cli) {
		PrintHeader("container")
		for _, c := range containers {
			if c.State == "exited" {
				PrintStatsHorizontally(c, struct {
					CPUPercentage float64
					MemoryUsage   float64
					MemoryLimit   float64
					NetworkRx     float64
					NetworkTx     float64
				}{})
			}
		}
	} else {
		fmt.Println("No stopped containers found")
	}

}

func PrintStatsHorizontally(c types.Container, stats struct {
	CPUPercentage float64
	MemoryUsage   float64
	MemoryLimit   float64
	NetworkRx     float64
	NetworkTx     float64
}) {
	green := color.New(color.FgGreen).Add(color.Bold).SprintFunc()

	maxIdLength := 12
	maxCombinedLength := 60

	containerID := c.ID[:maxIdLength]
	containerName := c.Names[0]

	// Combine ID and Name, ensuring the total length doesn't exceed maxCombinedLength
	combinedField := green(containerID) + " " + containerName
	if len(combinedField) > maxCombinedLength {
		combinedField = combinedField[:maxCombinedLength-3] + "..."
	}

	// Format each field
	idNameField := fmt.Sprintf("%-60s", combinedField)
	cpuField := fmt.Sprintf("%6s%%", ReturnCpuText(stats.CPUPercentage))
	memField := fmt.Sprintf("%9.2f MB /%9.2f MB", stats.MemoryUsage, stats.MemoryLimit)
	netField := fmt.Sprintf("%5.2f MB /%5.2f MB", stats.NetworkRx, stats.NetworkTx)

	// Combine all fields into the output string
	output := fmt.Sprintf("%s | %s | %s | %s", idNameField, cpuField, memField, netField)

	fmt.Println(output)
}

// ReturnCpuText It helps to print the CPU usage in red if it exceeds 100%
func ReturnCpuText(usage float64) string {
	text := fmt.Sprintf("%.2f", usage)
	if len(text) >= 6 {
		red := color.New(color.FgRed).Add(color.Bold).SprintFunc()
		return red(text)
	} else {
		return text
	}
}

func TarDirectory(dockerfilePath string) io.Reader {
	buf := new(bytes.Buffer)
	tw := tar.NewWriter(buf)
	defer func(tw *tar.Writer) {
		_ = tw.Close()
	}(tw)

	// Use the provided Dockerfile path
	if err := addFileToTarWriter(dockerfilePath, tw, "Dockerfile"); err != nil {
		log.Printf("Error adding Dockerfile to tar: %v", err)
		return nil
	}

	// Add the context (current directory)
	currentDir, err := os.Getwd()
	if err != nil {
		log.Printf("Error getting current directory: %v", err)
		return nil
	}

	err = filepath.Walk(currentDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Skip the Dockerfile itself as we've already added it
		if path == dockerfilePath {
			return nil
		}

		// Create a relative path for the file within the tar
		relPath, err := filepath.Rel(currentDir, path)
		if err != nil {
			return err
		}

		// Add the file to the tar
		return addFileToTarWriter(path, tw, relPath)
	})

	if err != nil {
		log.Printf("Error walking directory: %v", err)
		return nil
	}

	return buf
}

// Helper function to add files to the tarball
func addFileToTarWriter(filePath string, tw *tar.Writer, tarPath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer func(file *os.File) {
		_ = file.Close()
	}(file)

	info, err := file.Stat()
	if err != nil {
		return err
	}

	header, err := tar.FileInfoHeader(info, "")
	if err != nil {
		return err
	}
	header.Name = tarPath

	if err := tw.WriteHeader(header); err != nil {
		return err
	}

	_, err = io.Copy(tw, file)
	return err
}

func ListImages(ctx context.Context, cli *client.Client) error {
	images, err := cli.ImageList(ctx, image.ListOptions{})
	if err != nil {
		return fmt.Errorf("error listing images: %v", err)
	}

	for _, i := range images {
		for _, tag := range i.RepoTags {
			repo, tagName := splitRepoTag(tag)
			fmt.Printf("%-40s %-20s %-15s\n", repo, tagName, i.ID[7:19])
		}
	}

	return nil
}

// Helper function to split repository and tag
func splitRepoTag(repoTag string) (string, string) {
	if i := len(repoTag) - 1; i >= 0 && repoTag[i] == ':' {
		return repoTag[:i], "latest"
	}
	if i := strings.LastIndex(repoTag, ":"); i >= 0 {
		return repoTag[:i], repoTag[i+1:]
	}
	return repoTag, "latest"
}

func HandleFlags() ([]bool, bool) {
	verticalFlag := flag.Bool("v", false, "Print stats vertically")
	flag.BoolVar(verticalFlag, "vertical", false, "Print stats vertically")
	helpFlag := flag.Bool("h", false, "Print help message")
	flag.BoolVar(helpFlag, "help", false, "Print help message")
	listStoppedFlag := flag.Bool("s", false, "List stopped containers")
	flag.BoolVar(listStoppedFlag, "stopped", false, "List stopped containers")
	listImages := flag.Bool("i", false, "List images")
	flag.BoolVar(listImages, "images", false, "List images")
	flag.Parse()
	flagEntered := (*helpFlag || *listStoppedFlag || *listImages) == true
	return []bool{*verticalFlag, *helpFlag, *listStoppedFlag, *listImages}, flagEntered
}

func HandleFlagsForManipulation(ctx context.Context, cli *client.Client) []bool {
	// stores the flags
	flags, flagEntered := HandleFlags()

	if flagEntered {
		if flags[1] {
			PrintHelp()
		}
		if flags[2] {
			ListStoppedContainers(ctx, cli)
		}
		if flags[3] {
			PrintHeader("image")
			err := ListImages(ctx, cli)
			if err != nil {
				return nil
			}
		}
		os.Exit(0)
	}
	return flags
}

func GetDockerPlatform() *v1.Platform {
	platform := &v1.Platform{
		OS:           "linux", // Docker images are typically Linux-based
		Architecture: runtime.GOARCH,
	}

	// Adjust architecture naming if necessary
	switch platform.Architecture {
	case "amd64":
		platform.Architecture = "amd64"
	case "arm64":
		platform.Architecture = "arm64"
		// Add other cases as needed
	}

	return platform
}

func IsThereStoppedContainers(ctx context.Context, cli *client.Client) bool {
	containers, err := cli.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		fmt.Printf("Error listing containers: %v", err)
	}

	for _, c := range containers {
		if c.State == "exited" {
			return true
		}
	}
	return false
}
