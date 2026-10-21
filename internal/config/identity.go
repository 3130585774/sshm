// Identity setup changes only the selected host and preserves its other directives.
package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SetHostIdentity adds the selected key first and retains existing identity fallbacks.
func SetHostIdentity(configPath, hostName, identity string) error {
	if !isConcreteHostName(hostName) || strings.ContainsAny(hostName+identity, "\r\n\x00") {
		return fmt.Errorf("invalid host or identity path")
	}
	hosts, err := ParseSSHConfigFile(configPath)
	if err != nil {
		return err
	}
	var target *SSHHost
	for i := range hosts {
		if hosts[i].Name == hostName {
			target = &hosts[i]
			break
		}
	}
	if target == nil {
		return fmt.Errorf("host no longer exists in config")
	}
	if target.Identity == identity {
		return nil
	}
	configMutex.Lock()
	defer configMutex.Unlock()
	content, err := os.ReadFile(target.SourceFile)
	if err != nil {
		return err
	}
	lines := strings.Split(string(content), "\n")
	index := target.LineNumber - 1
	if index < 0 || index >= len(lines) {
		return fmt.Errorf("host configuration changed; retry setup")
	}
	names, ok := hostLineNames(lines[index])
	if !ok {
		return fmt.Errorf("host configuration changed; retry setup")
	}
	found := false
	var remaining []string
	for _, name := range names {
		if name == hostName {
			found = true
		} else {
			remaining = append(remaining, name)
		}
	}
	if !found {
		return fmt.Errorf("host configuration changed; retry setup")
	}
	directive := "    IdentityFile " + formatSSHConfigValue(identity)
	if len(names) == 1 {
		lines = append(lines[:index+1], append([]string{directive}, lines[index+1:]...)...)
	} else {
		// Split only this alias. Copy the complete body, including blank lines and
		// comments, rather than reconstructing directives from the parsed model.
		end := index + 1
		for end < len(lines) && !isBlockStart(lines[end]) {
			end++
		}
		body := append([]string{}, lines[index+1:end]...)
		block := []string{}
		if len(target.Tags) > 0 {
			block = append(block, "# Tags: "+strings.Join(target.Tags, ", "))
		}
		block = append(block, hostLineKeepingNegations([]string{hostName}, names), directive)
		block = append(block, body...)
		// Exclude the selected alias even if the original declaration has a wildcard.
		remaining = append(remaining, "!"+hostName)
		block = append(block, "Host "+formatHostNames(remaining...))
		block = append(block, body...)
		updated := append([]string{}, lines[:index]...)
		updated = append(updated, block...)
		lines = append(updated, lines[end:]...)
	}
	if err = backupConfig(target.SourceFile); err != nil {
		return err
	}
	return replaceIdentityConfig(target.SourceFile, content, []byte(strings.Join(lines, "\n")))
}

// replaceIdentityConfig writes completely before replacing the original. Resolve
// symlinks first so a config symlink continues to refer to the same file.
func replaceIdentityConfig(path string, original, updated []byte) error {
	target, err := filepath.EvalSymlinks(path)
	if err != nil {
		return fmt.Errorf("resolve config file: %w", err)
	}
	temp, err := os.CreateTemp(filepath.Dir(target), ".sshm-key-setup-*")
	if err != nil {
		return fmt.Errorf("create temporary config: %w", err)
	}
	tempPath := temp.Name()
	defer temp.Close()
	defer os.Remove(tempPath)
	if _, err = temp.Write(updated); err != nil {
		return fmt.Errorf("write temporary config: %w", err)
	}
	if err = temp.Sync(); err != nil {
		return fmt.Errorf("sync temporary config: %w", err)
	}
	if err = temp.Close(); err != nil {
		return fmt.Errorf("close temporary config: %w", err)
	}
	current, err := os.ReadFile(target)
	if err != nil {
		return fmt.Errorf("recheck config file: %w", err)
	}
	if !bytes.Equal(current, original) {
		return fmt.Errorf("SSH config changed during setup; retry instead of overwriting newer settings")
	}
	if err = os.Rename(tempPath, target); err != nil {
		return fmt.Errorf("replace config file: %w", err)
	}
	return nil
}
