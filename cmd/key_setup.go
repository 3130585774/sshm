// Key setup exposes the shared terminal wizard as a CLI command.
package cmd

import (
	"github.com/Gu1llaum-3/sshm/internal/keysetup"
	"github.com/spf13/cobra"
)

func init() {
	RootCmd.AddCommand(&cobra.Command{
		Use:   "key-setup <host>",
		Short: "Install an SSH public key and configure key login (Linux target)",
		Args:  cobra.ExactArgs(1),
		RunE:  func(cmd *cobra.Command, args []string) error { return keysetup.New(args[0], configFile).Run() },
	})
}
