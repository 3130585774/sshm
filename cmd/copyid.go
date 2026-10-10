package cmd

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

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
		if path, err := exec.LookPath("ssh-copy-id"); err == nil {
			c := exec.Command(path, buildCopyIDArgs(args[0], copyIDKey, configFile)...)
			c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
			return c.Run()
		}
		// ssh-copy-id is unavailable (e.g. Windows): fall back to plain ssh.
		return copyIDViaSSH(args[0], copyIDKey, configFile)
	},
}

// defaultPublicKey returns the first existing default public key in ~/.ssh.
func defaultPublicKey() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	for _, n := range []string{"id_ed25519.pub", "id_ecdsa.pub", "id_rsa.pub"} {
		p := filepath.Join(home, ".ssh", n)
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("no public key found in %s; use -i", filepath.Join(home, ".ssh"))
}

// buildSSHInstallArgs builds ssh arguments that append the key read from stdin
// to the remote authorized_keys (POSIX remote shell).
func buildSSHInstallArgs(host, cfg string) []string {
	var args []string
	if cfg != "" {
		args = append(args, "-F", cfg)
	}
	remote := "umask 077; mkdir -p ~/.ssh && touch ~/.ssh/authorized_keys && " +
		"read -r k && (grep -qxF \"$k\" ~/.ssh/authorized_keys || echo \"$k\" >> ~/.ssh/authorized_keys)"
	return append(args, host, remote)
}

func copyIDViaSSH(host, key, cfg string) error {
	sshPath, err := exec.LookPath("ssh")
	if err != nil {
		return fmt.Errorf("neither ssh-copy-id nor ssh found in PATH: %w", err)
	}
	if key == "" {
		if key, err = defaultPublicKey(); err != nil {
			return err
		}
	}
	data, err := os.ReadFile(key)
	if err != nil {
		return fmt.Errorf("reading public key: %w", err)
	}
	line := strings.TrimSpace(string(data))
	if line == "" || strings.ContainsAny(line, "\r\n") {
		return fmt.Errorf("%s is not a valid single-line public key", key)
	}
	c := exec.Command(sshPath, buildSSHInstallArgs(host, cfg)...)
	c.Stdin = bytes.NewReader([]byte(line + "\n"))
	c.Stdout, c.Stderr = os.Stdout, os.Stderr
	return c.Run()
}

func init() {
	copyIDCmd.Flags().StringVarP(&copyIDKey, "identity", "i", "", "public key file to upload")
	RootCmd.AddCommand(copyIDCmd)
}
