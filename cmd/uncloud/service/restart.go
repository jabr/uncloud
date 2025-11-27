package service

import (
	"context"
	"fmt"

	"github.com/docker/compose/v2/pkg/progress"
	"github.com/psviderski/uncloud/internal/cli"
	"github.com/spf13/cobra"
)

type restartOptions struct {
	services []string
}

func NewRestartCommand() *cobra.Command {
	opts := restartOptions{}
	cmd := &cobra.Command{
		Use:   "restart SERVICE [SERVICE...]",
		Short: "Restart one or more services.",
		Long:  "Restart one or more services.",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			uncli := cmd.Context().Value("cli").(*cli.CLI)
			opts.services = args
			return restart(cmd.Context(), uncli, opts)
		},
	}
	return cmd
}

func restart(ctx context.Context, uncli *cli.CLI, opts restartOptions) error {
	client, err := uncli.ConnectCluster(ctx)
	if err != nil {
		return fmt.Errorf("connect to cluster: %w", err)
	}
	defer client.Close()

	for _, s := range opts.services {
		err = progress.RunWithTitle(ctx, func(ctx context.Context) error {
			if err = client.RestartService(ctx, s); err != nil {
				return fmt.Errorf("restart service '%s': %w", s, err)
			}
			return nil
		}, uncli.ProgressOut(), "Restarting service "+s)
	}

	return err
}
