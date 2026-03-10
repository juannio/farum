package cmd

import (
	"fmt"
	"strings"

	"github.com/juannio/farum/image"
	"github.com/spf13/cobra"
)

func PullCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "pull <image>",
		Short: "Pull an image from Docker hub.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			parts := strings.Split(args[0], ":")
			if len(parts) != 2 {
				return fmt.Errorf("invalid image format, use <image>:<tag>")
			}
			return image.Pull(parts[0], parts[1])
		},
	}
}
