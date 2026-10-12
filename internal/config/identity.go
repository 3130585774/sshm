// Identity setup changes only the selected host and preserves its other directives.
package config

import (
	"fmt"
	"os"
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
	return os.WriteFile(target.SourceFile, []byte(strings.Join(lines, "\n")), 0600)
}
