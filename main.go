package main

import (
	"fmt"
	"os"

	"github.com/juannio/farum/cmd"
	"github.com/spf13/cobra"
)

func main() {
	root := &cobra.Command{
		Use:   "farum",
		Short: "A simple container runtime.",
	}

	root.AddCommand(cmd.PullCmd())
	root.AddCommand(cmd.RunCmd())
	root.AddCommand(cmd.InitCmd())

	if err := root.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
