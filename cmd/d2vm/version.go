package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"go.linka.cloud/d2vm"
)

var (
	cmdVersion = &cobra.Command{
		Use: "version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(d2vm.Version)
			fmt.Println(d2vm.BuildDate)
		},
	}
)

func init() {
	rootCmd.AddCommand(cmdVersion)
}
