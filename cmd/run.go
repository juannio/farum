package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/juannio/farum/container"
	"github.com/juannio/farum/image"
	"github.com/spf13/cobra"
)

func RunCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "run <image> <command>",
		Short: "Run a command in container.",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Split args
			parts := strings.Split(args[0], ":")
			if len(parts) != 2 {
				return fmt.Errorf("invalid image format, use <image>:<tag>")
			}

			imageName := parts[0]
			tag := parts[1]
			command := args[1:]

			img := image.New(imageName, tag)

			// --->> Check if image exists locally, pull if not
			if !imageExists(img.ImageDir) {
				fmt.Printf("image %s:%s not found locally, pulling...\n", img.Name, img.Tag)
				if err := img.Pull(); err != nil {
					return fmt.Errorf("failed to pull image: %w", err)
				}
			} else {
				fmt.Printf("image <<%s:%s>> found locally\n", img.Name, img.Tag)
			} // <<---

			// --->> Create and setup container fs
			c, err := container.New(img)
			if err != nil {
				return fmt.Errorf("failed to create container: %w", err)
			}

			fmt.Printf("container created --->> %s\n", c.ID)

			if err := c.Setup(); err != nil {
				return fmt.Errorf("failed to setup container %s: %w", c.ID, err)
			} // <<---
			// Cleanup container on any case after Setup
			defer c.CleanUp()

			// --->> Run command into container
			fmt.Printf("running %v in container %s\n", command, c.ID)

			if err := c.Run(command); err != nil {
				return fmt.Errorf("failed to run container: %w\n", err)
			}
			return nil
		},
	}
}

func imageExists(imageDir string) bool {
	entries, err := os.ReadDir(imageDir)
	if err != nil {
		return false
	}
	return len(entries) > 0
}
