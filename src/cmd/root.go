package cmd

import (
	"github.com/spf13/cobra"
	"picket-answersheet-service/src/config"
)

type funcCmd = func(config config.IConfig) *cobra.Command

func GetRoot(config config.IConfig) *cobra.Command {
	cmd := []funcCmd{server}
	root := &cobra.Command{}

	for _, item := range cmd {
		root.AddCommand(item(config))
	}

	return root
}
