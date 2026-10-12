// Package keysetup implements an interactive OpenSSH public-key installation wizard.
package keysetup

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Gu1llaum-3/sshm/internal/config"
	"golang.org/x/crypto/ssh"
)

// Wizard implements Bubble Tea's ExecCommand and can also run directly from the CLI.
type Wizard struct {
	Host, ConfigFile string
	stdin            io.Reader
	stdout, stderr   io.Writer
}

// New creates a wizard using the current terminal.
func New(host, configFile string) *Wizard {
	return &Wizard{Host: host, ConfigFile: configFile, stdin: os.Stdin, stdout: os.Stdout, stderr: os.Stderr}
}

// SetStdin sets the terminal input.
func (w *Wizard) SetStdin(r io.Reader) { w.stdin = r }

// SetStdout sets the terminal output.
func (w *Wizard) SetStdout(r io.Writer) { w.stdout = r }

// SetStderr sets the diagnostic output.
func (w *Wizard) SetStderr(r io.Writer) { w.stderr = r }

func (w *Wizard) run(name string, args ...string) error {
	c := exec.Command(name, args...)
	c.Stdin, c.Stdout, c.Stderr = w.stdin, w.stdout, w.stderr
	return c.Run()
}

func (w *Wizard) args() []string {
	a := []string{"-o", "StrictHostKeyChecking=ask", "-o", "RemoteCommand=none", "-o", "RequestTTY=no", "-o", "ClearAllForwardings=yes", "-o", "ControlMaster=no", "-o", "ControlPath=none"}
	if w.ConfigFile != "" {
		a = append(a, "-F", w.ConfigFile)
	}
	return a
}

// Run selects or generates a key, installs its public half, verifies login and saves the identity.
func (w *Wizard) Run() error {
	for _, tool := range []string{"ssh", "ssh-keygen"} {
		if _, err := exec.LookPath(tool); err != nil {
			return fmt.Errorf("%s is required: %w", tool, err)
		}
	}
	configPath := w.ConfigFile
	if configPath == "" {
		var err error
		configPath, err = config.GetDefaultSSHConfigPath()
		if err != nil {
			return err
		}
	}
	hosts, err := config.ParseSSHConfigFile(configPath)
	if err != nil {
		return err
	}
	found := false
	for _, h := range hosts {
		if h.Name == w.Host {
			found = true
			break
		}
	}
	if !found || strings.HasPrefix(w.Host, "-") {
		return fmt.Errorf("select a concrete host from your SSH configuration")
	}
	reader := bufio.NewReader(w.stdin)
	prompt := func(label string) (string, error) {
		fmt.Fprint(w.stdout, label)
		line, err := reader.ReadString('\n')
		if err != nil {
			return "", fmt.Errorf("input cancelled: %w", err)
		}
		return strings.TrimSpace(line), nil
	}
	fmt.Fprintf(w.stdout, "Configure key login for %s (Linux target account).\nOnly the public key is uploaded. Existing server access is preserved.\n", w.Host)
	key, err := prompt("Private key path, or 'new' to generate an Ed25519 key (empty cancels): ")
	if err != nil || key == "" {
		return err
	}
	if key == "new" {
		key, err = prompt("New private key path (must not exist; empty cancels): ")
		if err != nil || key == "" {
			return err
		}
		key, err = expandPath(key)
		if err != nil {
			return err
		}
		for _, p := range []string{key, key + ".pub"} {
			if _, e := os.Lstat(p); !os.IsNotExist(e) {
				return fmt.Errorf("refusing to overwrite %s", p)
			}
		}
		if err = os.MkdirAll(filepath.Dir(key), 0700); err != nil {
			return err
		}
		fmt.Fprintln(w.stdout, "Choose a passphrase when prompted. Use ssh-agent to unlock once per session; an empty passphrase leaves the key unencrypted.")
		if err = w.run("ssh-keygen", "-t", "ed25519", "-f", key, "-C", "sshm"); err != nil {
			return fmt.Errorf("generate key: %w", err)
		}
	} else {
		key, err = expandPath(key)
		if err != nil {
			return err
		}
	}
	if info, e := os.Stat(key); e != nil || !info.Mode().IsRegular() {
		return fmt.Errorf("private key must be an existing regular file")
	}
	// Derive from the private key instead of trusting a possibly stale .pub file.
	c := exec.Command("ssh-keygen", "-y", "-f", key)
	c.Stdin, c.Stderr = w.stdin, w.stderr
	pubBytes, err := c.Output()
	if err != nil {
		return fmt.Errorf("read private key (unlock it when prompted): %w", err)
	}
	pub, _, _, rest, err := ssh.ParseAuthorizedKey(pubBytes)
	if err != nil || len(bytes.TrimSpace(rest)) != 0 {
		return fmt.Errorf("invalid public key")
	}
	publicLine := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(pub)))
	// ssh -G reports OpenSSH's effective user/hostname/port including Include and Match.
	resolve := exec.Command("ssh", append(w.args(), "-G", w.Host)...)
	effective, err := resolve.Output()
	if err != nil {
		return fmt.Errorf("resolve target: %w", err)
	}
	for _, line := range strings.Split(string(effective), "\n") {
		if strings.HasPrefix(line, "user ") || strings.HasPrefix(line, "hostname ") || strings.HasPrefix(line, "port ") {
			fmt.Fprintln(w.stdout, line)
		}
	}
	fmt.Fprintf(w.stdout, "Public key: %s\n", ssh.FingerprintSHA256(pub))
	answer, err := prompt("Install this public key in the target account's authorized_keys? [y/N]: ")
	if err != nil {
		return err
	}
	if !strings.EqualFold(answer, "y") {
		return nil
	}
	installArgs := append(w.args(), "-T", w.Host, installScript(publicLine))
	if err = w.run("ssh", installArgs...); err != nil {
		return fmt.Errorf("public-key installation failed; local config unchanged: %w", err)
	}
	fmt.Fprintln(w.stdout, "Public key installed. Verifying public-key login (you may need to unlock your private key)...")
	verifyArgs := append(w.args(), "-v", "-T", "-i", key, "-o", "IdentitiesOnly=yes", "-o", "PreferredAuthentications=publickey", "-o", "PasswordAuthentication=no", "-o", "KbdInteractiveAuthentication=no", w.Host, "true")
	verify := exec.Command("ssh", verifyArgs...)
	var diagnostics bytes.Buffer
	verify.Stdin, verify.Stdout, verify.Stderr = w.stdin, w.stdout, &diagnostics
	if err = verify.Run(); err != nil {
		fmt.Fprint(w.stderr, diagnostics.String())
		return fmt.Errorf("key installed, but public-key login verification failed; local config unchanged (check server policy or unlock with ssh-add): %w", err)
	}
	if !acceptedKey(diagnostics.String(), ssh.FingerprintSHA256(pub)) {
		return fmt.Errorf("key installed, but SSH authenticated with a different key; local config unchanged")
	}
	if err = config.SetHostIdentity(configPath, w.Host, key); err != nil {
		return fmt.Errorf("key installed and login verified, but saving local config failed: %w", err)
	}
	fmt.Fprintln(w.stdout, "Key login configured. Press Enter on this host to connect. Encrypted keys may still need ssh-agent unlocking.")
	return nil
}

func expandPath(p string) (string, error) {
	if p == "~" {
		return os.UserHomeDir()
	}
	if strings.HasPrefix(p, "~/") || strings.HasPrefix(p, "~\\") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		p = filepath.Join(home, p[2:])
	}
	return filepath.Abs(p)
}

func shellQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'" }

// The canonical key contains no user-controlled shell syntax or comment. Preserve
// existing entries, compare key type/blob (ignoring comments), and normalize modes.
func installScript(publicLine string) string {
	fields := strings.Fields(publicLine)
	return `set -eu
[ "$(uname -s)" = Linux ] || { echo 'Only Linux target accounts are supported.' >&2; exit 1; }
umask 077
mkdir -p "$HOME/.ssh"
chmod 700 "$HOME/.ssh"
auth="$HOME/.ssh/authorized_keys"
[ ! -L "$auth" ] || { echo 'Refusing symlink authorized_keys.' >&2; exit 1; }
touch "$auth"
chmod 600 "$auth"
if ! awk -v type=` + shellQuote(fields[0]) + ` -v blob=` + shellQuote(fields[1]) + ` '{ for (i=1;i<NF;i++) if ($i==type && $(i+1)==blob) found=1 } END { exit !found }' "$auth"; then
 printf '\n%s\n' ` + shellQuote(publicLine+" sshm") + ` >> "$auth"
fi`
}

// OpenSSH may try configured fallback identities even with IdentitiesOnly. Check
// the last accepted identity so another working key cannot mask a setup failure.
func acceptedKey(diagnostics, fingerprint string) bool {
	last := ""
	for _, line := range strings.Split(diagnostics, "\n") {
		if strings.Contains(line, "Server accepts key:") {
			last = line
		}
	}
	return strings.Contains(last, " "+fingerprint+" ") || strings.HasSuffix(last, " "+fingerprint)
}
