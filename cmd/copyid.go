package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

// copyIDKey is the public key file to upload (defaults to ssh-copy-id's choice).
var copyIDKey string

// buildCopyIDArgs builds the ssh-copy-id arguments for the given host.
func buildCopyIDArgs(host, key, cfg string) []string {
	var args []string
	if key != "" {
		args = append(args, "-i", key)
	}
	if cfg != "" {
		args = append(args, "-F", cfg)
	}
	return append(args, host)
}

var copyIDCmd = &cobra.Command{
	Use:   "copy-id <host>",
	Short: "Upload your public key to a configured host",
	Long: `Upload your SSH public key to a host using ssh-copy-id.
The host's settings (HostName, User, Port, ProxyJump...) come from your SSH config.
You will be prompted for the password by ssh; it is never stored.`,
	Example: "  sshm copy-id prod-server\n  sshm copy-id prod-server -i ~/.ssh/id_ed25519.pub",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path, err := exec.LookPath("ssh-copy-id")
		if err != nil {
			return fmt.Errorf("ssh-copy-id not found in PATH: %w", err)
		}
		c := exec.Command(path, buildCopyIDArgs(args[0], copyIDKey, configFile)...)
		c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
		return c.Run()
	},
}

func init() {
	copyIDCmd.Flags().StringVarP(&copyIDKey, "identity", "i", "", "public key file to upload")
	RootCmd.AddCommand(copyIDCmd)
}
