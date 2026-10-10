// Package credentials stores SSH passwords in the operating system credential
// manager (macOS Keychain, Windows Credential Manager, Linux Secret Service)
// and feeds them to ssh through the SSH_ASKPASS mechanism.
package credentials

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/zalando/go-keyring"
)

const (
	service = "sshm"
	// hostEnv carries the host whose password the askpass helper should return.
	hostEnv = "SSHM_ASKPASS_HOST"
)

// Set stores the password for host in the OS credential manager.
func Set(host, password string) error {
	if err := keyring.Set(service, host, password); err != nil {
		return fmt.Errorf("saving password for %q: %w", host, err)
	}
	return nil
}

// Get returns the saved password for host. ok is false when none is saved
// (or the credential manager is unavailable).
func Get(host string) (password string, ok bool) {
	p, err := keyring.Get(service, host)
	if err != nil {
		return "", false
	}
	return p, true
}

// Has reports whether a password is saved for host.
func Has(host string) bool {
	_, ok := Get(host)
	return ok
}

// Delete removes the saved password for host. Deleting a missing entry is not an error.
func Delete(host string) error {
	if err := keyring.Delete(service, host); err != nil && !errors.Is(err, keyring.ErrNotFound) {
		return fmt.Errorf("deleting password for %q: %w", host, err)
	}
	return nil
}

// Env returns environment variables that make ssh obtain the saved password
// for host from this executable (requires OpenSSH 8.4+ for SSH_ASKPASS_REQUIRE).
// It returns nil when no password is saved or the executable cannot be located.
func Env(host string) []string {
	if !Has(host) {
		return nil
	}
	exe, err := os.Executable()
	if err != nil {
		return nil
	}
	return []string{
		"SSH_ASKPASS=" + exe,
		"SSH_ASKPASS_REQUIRE=force",
		hostEnv + "=" + host,
	}
}

// MaybeRunAskpass handles the invocation of this binary as an SSH_ASKPASS
// helper. It returns true when it handled the call and the process should exit;
// the returned code is the exit status.
func MaybeRunAskpass(args []string) (handled bool, code int) {
	host := os.Getenv(hostEnv)
	if host == "" {
		return false, 0
	}
	prompt := ""
	if len(args) > 0 {
		prompt = strings.ToLower(args[0])
	}
	// Only answer password prompts; never passphrases or host key confirmations.
	if !strings.Contains(prompt, "password") || strings.Contains(prompt, "passphrase") {
		return true, 1
	}
	p, ok := Get(host)
	if !ok {
		return true, 1
	}
	fmt.Println(p)
	return true, 0
}
