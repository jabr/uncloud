package e2e

import (
	"bytes"
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	dockerclient "github.com/docker/docker/client"
	"github.com/google/uuid"
	"github.com/psviderski/uncloud/api/pb"
	"github.com/psviderski/uncloud/internal/ucind"
	"github.com/psviderski/uncloud/pkg/client"
	"github.com/psviderski/uncloud/pkg/distlock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func createTestCluster(
	t *testing.T, name string, opts ucind.CreateClusterOptions, waitReady bool,
) (ucind.Cluster, *ucind.Provisioner) {
	dockerCli, err := dockerclient.NewClientWithOpts(dockerclient.FromEnv, dockerclient.WithAPIVersionNegotiation())
	require.NoError(t, err)

	p := ucind.NewProvisioner(dockerCli, nil)
	ctx := context.Background()

	// Use the existing cluster if specified by the environment variable.
	envName := os.Getenv("TEST_CLUSTER_NAME")
	if envName != "" {
		c, err := p.InspectCluster(ctx, envName)
		if err == nil {
			return c, p
		}
		if !errors.Is(err, ucind.ErrNotFound) {
			require.NoError(t, err)
		}
	}

	// Remove the cluster if it already exists. It could be left from a previous interrupted test run.
	require.NoError(t, p.RemoveCluster(ctx, name))

	c, err := p.CreateCluster(ctx, name, opts)
	require.NoError(t, err)
	assert.Equal(t, name, c.Name)
	assert.Len(t, c.Machines, opts.Machines)
	for _, m := range c.Machines {
		assert.NotEmpty(t, m.ID)
	}

	t.Cleanup(func() {
		require.NoError(t, p.RemoveCluster(ctx, name))
	})

	if waitReady {
		require.NoError(t, p.WaitClusterReady(ctx, c, 90*time.Second))
	}

	return c, p
}

func TestClusterLifecycle(t *testing.T) {
	t.Parallel()

	name := "ucind-test.cluster-lifecycle"
	ctx := context.Background()
	c, p := createTestCluster(t, name, ucind.CreateClusterOptions{Machines: 3}, false)

	t.Run("each machine reconciled cluster store", func(t *testing.T) {
		var err error
		// Create a client for each machine and wait for it to be ready.
		clients := make([]*client.Client, len(c.Machines))
		for i, m := range c.Machines {
			clients[i], err = m.Connect(ctx)
			require.NoError(t, err)
			//goland:noinspection GoDeferInLoop
			defer clients[i].Close()
		}

		// Any machine should work as a cluster API endpoint, e.g. be able to list all machines in the cluster.
		for i, cli := range clients {
			// Wait for the machine to reconcile the cluster store.
			require.Eventually(t, func() bool {
				machines, err := cli.ListMachines(ctx, nil)
				if err != nil {
					// Unavailable "machine is not ready to serve cluster requests" is expected until
					// the store is reconciled.
					if s, ok := status.FromError(err); ok && s.Code() == codes.Unavailable {
						return false
					}
					require.NoError(t, err)
				}

				if len(machines) != 3 {
					return false
				}

				for _, m := range machines {
					if pb.MachineMember_UP != m.State {
						return false
					}
				}

				return true
			}, 30*time.Second, 50*time.Millisecond, "cluster store not reconciled on machine #%d", i+1)
		}
	})

	t.Run("inspect", func(t *testing.T) {
		cluster, err := p.InspectCluster(ctx, name)
		require.NoError(t, err)

		assert.Equal(t, name, cluster.Name)
		assert.Len(t, cluster.Machines, 3)
		for _, m := range cluster.Machines {
			assert.NotEmpty(t, m.ID)
			assert.True(t, strings.HasPrefix(m.Name, "machine-"))
		}
	})

	t.Run("distributed lock", func(t *testing.T) {
		cli0, err := c.Machines[0].Connect(ctx)
		require.NoError(t, err)
		t.Cleanup(func() {
			require.NoError(t, cli0.Close())
		})

		cli1, err := c.Machines[1].Connect(ctx)
		require.NoError(t, err)
		t.Cleanup(func() {
			require.NoError(t, cli1.Close())
		})

		firstLocker, err := cli0.NewLocker(distlock.Config{})
		require.NoError(t, err)
		secondLocker, err := cli1.NewLocker(distlock.Config{})
		require.NoError(t, err)

		acquire := func(locker *distlock.Locker) (*distlock.Lease, error) {
			acquireCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
			defer cancel()
			return locker.Acquire(acquireCtx, "e2e-lock")
		}
		release := func(lease *distlock.Lease) {
			t.Helper()
			if lease.Context().Err() != nil {
				return
			}
			releaseCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
			defer cancel()
			require.NoError(t, lease.Release(releaseCtx))
		}

		firstLease, err := acquire(firstLocker)
		require.NoError(t, err)
		t.Cleanup(func() {
			release(firstLease)
		})

		contendingCtx, cancelContending := context.WithTimeout(ctx, 500*time.Millisecond)
		contendingLease, err := secondLocker.Acquire(contendingCtx, "e2e-lock")
		cancelContending()
		require.ErrorIs(t, err, context.DeadlineExceeded)
		require.Nil(t, contendingLease)

		release(firstLease)

		secondLease, err := acquire(secondLocker)
		require.NoError(t, err)
		t.Cleanup(func() {
			release(secondLease)
		})
	})

	t.Run("Caddy storage replication", func(t *testing.T) {
		cli0, err := c.Machines[0].Connect(ctx)
		require.NoError(t, err)
		t.Cleanup(func() {
			require.NoError(t, cli0.Close())
		})

		cli1, err := c.Machines[1].Connect(ctx)
		require.NoError(t, err)
		t.Cleanup(func() {
			require.NoError(t, cli1.Close())
		})

		prefix := "e2e/caddy-storage/" + uuid.NewString()
		key := prefix + "/key/path"

		// Keep reused test clusters clean if an assertion stops the test before its explicit deletes.
		t.Cleanup(func() {
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			_, _ = cli1.CaddyStorage.Delete(client.ProxyMachinesContext(cleanupCtx, nil),
				&pb.DeleteCaddyStorageRequest{Key: prefix})
		})

		// Verify both creation and overwrite, allowing each value to replicate before writing the next.
		var updatedAt time.Time
		for _, value := range [][]byte{[]byte("test-value"), []byte("replacement-value")} {
			_, err = cli0.CaddyStorage.Store(ctx, &pb.StoreCaddyStorageRequest{Key: key, Value: value})
			require.NoError(t, err)

			// A Load through another machine must find the value on the machine that accepted the local write,
			// regardless of whether Corrosion has replicated it to the other machines yet.
			loadResp, err := cli1.CaddyStorage.Load(client.ProxySingleMachineContext(ctx, c.Machines[0].ID),
				&pb.LoadCaddyStorageRequest{Key: key})
			require.NoError(t, err)
			require.Len(t, loadResp.Messages, 1)
			originResult := loadResp.Messages[0]
			require.Nil(t, originResult.Metadata,
				"Proxy to a single machine should not inject metadata into the response")
			require.Equal(t, value, originResult.Value)
			require.NoError(t, originResult.UpdatedAt.CheckValid())
			modified := originResult.UpdatedAt.AsTime()
			require.False(t, modified.IsZero(), "Stored value should have a valid updated_at timestamp")
			if !updatedAt.IsZero() {
				require.True(t, modified.After(updatedAt), "Overwriting a value should advance updated_at")
			}
			updatedAt = modified

			require.Eventually(t, func() bool {
				callCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
				defer cancel()

				resp, err := cli1.CaddyStorage.Load(client.ProxyMachinesContext(callCtx, nil),
					&pb.LoadCaddyStorageRequest{Key: key})
				if err != nil {
					return false
				}

				require.Len(t, resp.Messages, 3)
				for _, m := range resp.Messages {
					require.NotNil(t, m.Metadata)
					if m.Metadata.Error != "" || !bytes.Equal(m.Value,
						value) || !m.UpdatedAt.AsTime().Equal(updatedAt) {
						return false
					}
				}
				return true
			}, 30*time.Second, 100*time.Millisecond, "Caddy storage value %q should replicate to every machine", value)

			// Every machine should report the same Caddy storage key information.
			statResp, err := cli1.CaddyStorage.Stat(client.ProxyMachinesContext(ctx, nil),
				&pb.StatCaddyStorageRequest{Key: key})
			require.NoError(t, err)
			require.Len(t, statResp.Messages, 3)
			for _, m := range statResp.Messages {
				require.NotNil(t, m.Metadata)
				assert.Equal(t, "", m.Metadata.Error)
				assert.Equal(t, key, m.Key)
				assert.True(t, m.UpdatedAt.AsTime().Equal(updatedAt))
				assert.EqualValues(t, len(value), m.Size)
				assert.True(t, m.IsTerminal)
			}
		}

		// A path with descendants should exist as a directory even though no value is stored at that key.
		statResp, err := cli1.CaddyStorage.Stat(client.ProxyMachinesContext(ctx, nil),
			&pb.StatCaddyStorageRequest{Key: prefix + "/key"})
		require.NoError(t, err)
		require.Len(t, statResp.Messages, 3)
		for _, m := range statResp.Messages {
			require.NotNil(t, m.Metadata)
			assert.Equal(t, "", m.Metadata.Error)
			assert.Equal(t, prefix+"/key", m.Key)
			assert.Nil(t, m.UpdatedAt)
			assert.EqualValues(t, 0, m.Size)
			assert.False(t, m.IsTerminal)
		}

		listResp, err := cli1.CaddyStorage.List(client.ProxyMachinesContext(ctx, nil),
			&pb.ListCaddyStorageRequest{Prefix: prefix, Recursive: true})
		require.NoError(t, err)
		require.Len(t, listResp.Messages, 3)
		for _, m := range listResp.Messages {
			require.NotNil(t, m.Metadata)
			assert.Equal(t, "", m.Metadata.Error)
			assert.Equal(t, []string{prefix + "/key", key}, m.Keys)
		}

		// A non-recursive list should only return the immediate child keys.
		listResp, err = cli1.CaddyStorage.List(client.ProxyMachinesContext(ctx, nil),
			&pb.ListCaddyStorageRequest{Prefix: prefix, Recursive: false})
		require.NoError(t, err)
		require.Len(t, listResp.Messages, 3)
		for _, m := range listResp.Messages {
			require.NotNil(t, m.Metadata)
			assert.Equal(t, "", m.Metadata.Error)
			assert.Equal(t, []string{prefix + "/key"}, m.Keys)
		}

		// Delete through one machine. The deletion must reach the other replicas through Corrosion.
		_, err = cli1.CaddyStorage.Delete(ctx, &pb.DeleteCaddyStorageRequest{Key: prefix})
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			callCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()

			resp, err := cli0.CaddyStorage.Load(client.ProxyMachinesContext(callCtx, nil),
				&pb.LoadCaddyStorageRequest{Key: key})
			if err != nil {
				return false
			}

			require.Len(t, resp.Messages, 3)
			for _, m := range resp.Messages {
				require.NotNil(t, m.Metadata)
				if codes.Code(m.Metadata.Status.GetCode()) != codes.NotFound {
					return false
				}
			}
			return true
		}, 30*time.Second, 100*time.Millisecond, "Caddy storage deletion should replicate to every machine")

		statResp, err = cli0.CaddyStorage.Stat(client.ProxyMachinesContext(ctx, nil),
			&pb.StatCaddyStorageRequest{Key: key})
		require.NoError(t, err)
		require.Len(t, statResp.Messages, 3)
		for _, m := range statResp.Messages {
			require.NotNil(t, m.Metadata)
			assert.Equal(t, codes.NotFound, codes.Code(m.Metadata.Status.GetCode()))
		}

		listResp, err = cli0.CaddyStorage.List(client.ProxyMachinesContext(ctx, nil),
			&pb.ListCaddyStorageRequest{Prefix: prefix, Recursive: true})
		require.NoError(t, err)
		require.Len(t, listResp.Messages, 3)
		for _, m := range listResp.Messages {
			require.NotNil(t, m.Metadata)
			assert.Equal(t, codes.NotFound, codes.Code(m.Metadata.Status.GetCode()))
		}

		// Delete is idempotent, so every machine should still return a successful response.
		deleteResp, err := cli1.CaddyStorage.Delete(client.ProxyMachinesContext(ctx, nil),
			&pb.DeleteCaddyStorageRequest{Key: prefix})
		require.NoError(t, err)
		require.Len(t, deleteResp.Messages, 3)
		for _, m := range deleteResp.Messages {
			require.NotNil(t, m.Metadata)
			assert.Equal(t, "", m.Metadata.Error)
		}
	})

	t.Run("remove", func(t *testing.T) {
		err := p.RemoveCluster(ctx, name)
		require.NoError(t, err)

		_, err = p.InspectCluster(ctx, name)
		require.ErrorIs(t, err, ucind.ErrNotFound)
	})
}
