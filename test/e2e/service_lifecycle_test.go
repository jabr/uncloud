package e2e

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/psviderski/uncloud/internal/ucind"
	"github.com/psviderski/uncloud/pkg/api"
	"github.com/psviderski/uncloud/pkg/client"
	"github.com/stretchr/testify/require"
)

// Helper function to deploy a simple alpine service for testing lifecycle
func deployLifecycleTestService(t *testing.T, ctx context.Context, cli *client.Client, name string, replicas uint) {
	t.Helper()

	spec := api.ServiceSpec{
		Name:     name,
		Replicas: replicas,
		Container: api.ContainerSpec{
			Image:   "alpine:3.20",
			Command: []string{"sleep", "3600"},
		},
	}

	deployment := cli.NewDeployment(spec, nil)
	err := deployment.Validate(ctx)
	require.NoError(t, err)

	_, err = deployment.Run(ctx)
	require.NoError(t, err)

	// Wait for all replicas to be running
	require.Eventually(t, func() bool {
		service, err := cli.InspectService(ctx, name)
		if err != nil {
			return false
		}
		if uint(len(service.Containers)) != replicas {
			return false
		}
		for _, ctr := range service.Containers {
			if ctr.Container.State.Status != "running" {
				return false
			}
		}
		return true
	}, 30*time.Second, 1*time.Second, fmt.Sprintf("service %s should have %d running replicas", name, replicas))
}

func TestServiceStopStartRestart(t *testing.T) {
	t.Parallel()

	clusterName := "ucind-test.lifecycle-ssr"
	ctx := context.Background()
	// Using 2 machines to ensure distributed operations work
	c, _ := createTestCluster(t, clusterName, ucind.CreateClusterOptions{Machines: 2}, true)

	cli, err := c.Machines[0].Connect(ctx)
	require.NoError(t, err)

	serviceName := "lifecycle-test"
	deployLifecycleTestService(t, ctx, cli, serviceName, 2)

	t.Run("stop service", func(t *testing.T) {
		err := cli.StopService(ctx, serviceName)
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			service, err := cli.InspectService(ctx, serviceName)
			if err != nil {
				return false
			}
			for _, ctr := range service.Containers {
				if ctr.Container.State.Status != "exited" && ctr.Container.State.Status != "stopped" {
					// Docker reports 'exited' for stopped containers usually
					return false
				}
			}
			return true
		}, 30*time.Second, 1*time.Second, "service should be stopped")
	})

	t.Run("start service", func(t *testing.T) {
		err := cli.StartService(ctx, serviceName)
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			service, err := cli.InspectService(ctx, serviceName)
			if err != nil {
				return false
			}
			for _, ctr := range service.Containers {
				if ctr.Container.State.Status != "running" {
					return false
				}
			}
			return true
		}, 30*time.Second, 1*time.Second, "service should be running")
	})

	t.Run("restart service", func(t *testing.T) {
		// Get container start time before restart
		service, err := cli.InspectService(ctx, serviceName)
		require.NoError(t, err)

		startTimeByContainerID := make(map[string]string)
		for _, ctr := range service.Containers {
			startTimeByContainerID[ctr.Container.ID] = ctr.Container.State.StartedAt
		}

		err = cli.RestartService(ctx, serviceName)
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			service, err := cli.InspectService(ctx, serviceName)
			if err != nil {
				return false
			}
			// Check if container is running and has a new start time
			for _, ctr := range service.Containers {
				if ctr.Container.State.Status != "running" {
					return false
				}

				oldTime, ok := startTimeByContainerID[ctr.Container.ID]
				if !ok {
					// Container ID changed? Unlikely for restart unless recreated.
					// But stop/start preserves container.
					// If IDs changed, then it was definitely restarted/recreated.
					continue
				}

				if ctr.Container.State.StartedAt == oldTime {
					return false
				}
			}
			return true
		}, 30*time.Second, 1*time.Second, "service should be restarted")
	})
}
