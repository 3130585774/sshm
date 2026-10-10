package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/Gu1llaum-3/sshm/internal/credentials"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var passwdDelete bool

var passwdCmd = &cobra.Command{
	Use:   "passwd <host>",
	Short: "Save or remove a host password in the OS credential manager",
	Long: `Save the SSH password of a host in the operating system credential manager
(macOS Keychain, Windows Credential Manager, Linux Secret Service).
sshm then supplies it automatically when connecting (requires OpenSSH 8.4+).
Use --delete to remove the saved password.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if passwdDelete {
			return credentials.Delete(args[0])
		}
		fmt.Fprintf(os.Stderr, "Password for %s: ", args[0])
		var pw string
		if term.IsTerminal(int(os.Stdin.Fd())) {
			b, err := term.ReadPassword(int(os.Stdin.Fd()))
			fmt.Fprintln(os.Stderr)
			if err != nil {
				return err
			}
			pw = string(b)
		} else {
			line, err := bufio.NewReader(os.Stdin).ReadString('\n')
			if err != nil && line == "" {
				return err
			}
			pw = strings.TrimRight(line, "\r\n")
		}
		if pw == "" {
			return fmt.Errorf("empty password")
		}
		return credentials.Set(args[0], pw)
	},
}

func init() {
	passwdCmd.Flags().BoolVarP(&passwdDelete, "delete", "d", false, "remove the saved password")
	RootCmd.AddCommand(passwdCmd)
}
