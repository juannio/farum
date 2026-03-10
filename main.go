package main

import (
	"fmt"
	"os"

	"github.com/juannio/gocker/cmd"
	"github.com/spf13/cobra"
)

func main() {
	root := &cobra.Command{
		Use:   "gocker",
		Short: "A simple container runtime.",
	}

	root.AddCommand(cmd.PullCmd())
	root.AddCommand(cmd.RunCmd())

	if err := root.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
