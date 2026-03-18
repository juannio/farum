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
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {

			containerId := args[0]

			// This bin is spawned by farum run process,
			// therefore this command is executed in container's own runtime namespace.
			// Prevent mount propagation to host
			syscall.Mount("", "/", "", syscall.MS_SLAVE|syscall.MS_REC, "")
			// Mount procfs in /proc
			if err := syscall.Mount("proc", "/proc", "proc", 0, ""); err != nil {
				return fmt.Errorf("failed to mount procfs at /proc: %w", err)
			}

			// Set hostname
			if err := syscall.Sethostname([]byte(containerId)); err != nil {
				return fmt.Errorf("failed to set hostname: %w", err)
			}

			// Exec command (/bin/sh or any shell used for container)
			if err := syscall.Exec(args[1], args[1:], os.Environ()); err != nil {
				return fmt.Errorf("failed to run command: %s: %w", args[0], err)
			}
			return nil
		},
	}
}
