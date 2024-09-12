package stats

import (
	"context"
	"encoding/json"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"io"
	"log"
	"sync"
)

type ContainerStat struct {
	Container types.Container
	Stats     struct {
		CPUPercentage float64
		MemoryUsage   float64
		MemoryLimit   float64
		NetworkRx     float64
		NetworkTx     float64
	}
}

func GetContainerStats(ctx context.Context, cli *client.Client, containerID string) (struct {
	CPUPercentage float64
	MemoryUsage   float64
	MemoryLimit   float64
	NetworkRx     float64
	NetworkTx     float64
}, error) {
	stats, err := cli.ContainerStats(ctx, containerID, false)
	if err != nil {
		return struct {
			CPUPercentage float64
			MemoryUsage   float64
			MemoryLimit   float64
			NetworkRx     float64
			NetworkTx     float64
		}{}, err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.Printf("Error closing stats body: %v", err)
		}
	}(stats.Body)

	var statsJSON container.StatsResponse
	if err := json.NewDecoder(stats.Body).Decode(&statsJSON); err != nil {
		return struct {
			CPUPercentage float64
			MemoryUsage   float64
			MemoryLimit   float64
			NetworkRx     float64
			NetworkTx     float64
		}{}, err
	}

	cpuPercent := calculateCPUPercentage(&statsJSON)
	memoryUsage := float64(statsJSON.MemoryStats.Usage) / 1024 / 1024
	memoryLimit := float64(statsJSON.MemoryStats.Limit) / 1024 / 1024
	networkRx := float64(statsJSON.Networks["eth0"].RxBytes) / 1024 / 1024
	networkTx := float64(statsJSON.Networks["eth0"].TxBytes) / 1024 / 1024

	return struct {
		CPUPercentage float64
		MemoryUsage   float64
		MemoryLimit   float64
		NetworkRx     float64
		NetworkTx     float64
	}{
		CPUPercentage: cpuPercent,
		MemoryUsage:   memoryUsage,
		MemoryLimit:   memoryLimit,
		NetworkRx:     networkRx,
		NetworkTx:     networkTx,
	}, nil
}

func calculateCPUPercentage(stats *container.StatsResponse) float64 {
	perCpuUsageLen := len(stats.CPUStats.CPUUsage.PercpuUsage)
	// If the users OS is not Linux, the perCpuUsage will be empty, So the average CPU usage will be calculated
	if perCpuUsageLen == 0 {
		perCpuUsageLen = 7
	}
	cpuDelta := float64(stats.CPUStats.CPUUsage.TotalUsage) - float64(stats.PreCPUStats.CPUUsage.TotalUsage)
	systemDelta := float64(stats.CPUStats.SystemUsage) - float64(stats.PreCPUStats.SystemUsage)
	cpuPercent := (cpuDelta / systemDelta) * float64(perCpuUsageLen) * 100.0
	return cpuPercent
}

func FetchContainerStatsParallel(ctx context.Context, cli *client.Client, containers []types.Container) []ContainerStat {
	var wg sync.WaitGroup
	statsChan := make(chan struct {
		Index int
		Stat  ContainerStat
	}, len(containers))

	for index, _container := range containers {
		wg.Add(1)
		go func(i int, c types.Container) {
			defer wg.Done()
			stats, err := GetContainerStats(ctx, cli, c.ID)
			if err != nil {
				log.Printf("Error fetching stats for container %s: %v", c.ID, err)
				return
			}
			statsChan <- struct {
				Index int
				Stat  ContainerStat
			}{
				Index: i,
				Stat:  ContainerStat{Container: c, Stats: stats},
			}
		}(index, _container)
	}

	go func() {
		wg.Wait()
		close(statsChan)
	}()

	// Create a slice with pre-defined size based on container count
	results := make([]ContainerStat, len(containers))

	// Insert stats in the correct position
	for result := range statsChan {
		results[result.Index] = result.Stat
	}

	return results
}
