package cmd

import (
	"fmt"
	"time"

	"github.com/juannio/farum/container"
	"github.com/spf13/cobra"
)

func PsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ps",
		Short: "Display containers status",
		RunE: func(cmd *cobra.Command, args []string) error {

			// Read existing containers metadata
			content, err := container.ReadMetadata()
			if err != nil {
				return fmt.Errorf("failed to read metadata: %w", err)
			}

			// TODO: Improve display data
			fmt.Printf("%-15s %-18s %-10s %-12s %-10s\n",
				"CONTAINER ID", "IMAGE", "STATUS", "COMMAND", "UPTIME")

			for _, metadata := range content {
				fmt.Printf("%-15s %-18s %-10s %-12s %-10s\n",
					metadata.Id,
					metadata.Image,
					metadata.Status,
					metadata.Command,
					getUptime(metadata),
				)
			}

			return nil
		},
	}
}

func getUptime(metadata container.ContainerMetadata) time.Duration {
	if metadata.Status == "RUNNING" {
		return time.Since(metadata.CreatedAt)
	}

	// TODO: Implement uptime of exited/stopped containers
	// using metadata.FinalizedAt
	return 0
}
