package cmd

import (
	"fmt"
	"os"
	"syscall"

	"github.com/spf13/cobra"
)

func InitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init <command>",
		Short: "Run go binary into child process",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {

			if err := syscall.Mount("proc", "/proc", "proc", 0, ""); err != nil {
				return fmt.Errorf("failed to mount /proc: %w", err)
			}

			if err := syscall.Exec(args[0], args, os.Environ()); err != nil {
				return fmt.Errorf("failed to mount /proc: %w", err)
			}
			//TODO, cleanup init bin passed to container
			return nil
		},
	}
}
