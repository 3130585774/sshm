package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestSetHostIdentity(t *testing.T) {
	for _, shared := range []bool{false, true} {
		t.Run(map[bool]string{false: "single", true: "aliases"}[shared], func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv("XDG_CONFIG_HOME", dir)
			main := filepath.Join(dir, "config")
			included := filepath.Join(dir, "hosts.conf")
			names := "target"
			if shared {
				names = "target sibling"
			}
			original := "Host " + names + "\n HostName example.com\n IdentityFile /old/key\n ProxyJump jump\n # keep comment\n ServerAliveInterval 17\n\nMatch host other\n User special\n"
			os.WriteFile(main, []byte("Include "+included+"\nHost jump\n HostName jump.example\n"), 0600)
			os.WriteFile(included, []byte(original), 0600)
			key := "/keys/path with spaces"
			if err := SetHostIdentity(main, "target", key); err != nil {
				t.Fatal(err)
			}
			before, _ := os.ReadFile(included)
			if err := SetHostIdentity(main, "target", key); err != nil {
				t.Fatal(err)
			}
			after, _ := os.ReadFile(included)
			if string(before) != string(after) {
				t.Fatal("repeat changed config")
			}
			hosts, err := ParseSSHConfigFile(main)
			if err != nil {
				t.Fatal(err)
			}
			for _, h := range hosts {
				if h.Name == "target" && (h.Identity != key || h.ProxyJump != "jump" || !strings.Contains(h.Options, "ServerAliveInterval 17")) {
					t.Fatalf("target lost config: %+v", h)
				}
				if h.Name == "sibling" && h.Identity != "/old/key" {
					t.Fatal("sibling identity changed")
				}
			}
			data, _ := os.ReadFile(main)
			if !strings.HasPrefix(string(data), "Include ") {
				t.Fatal("main file changed")
			}
			if !strings.Contains(string(after), "# keep comment") {
				t.Fatal("comment lost")
			}
			if !strings.Contains(string(after), "Match host other\n User special") {
				t.Fatal("Match block lost")
			}
		})
	}
}

func TestReplaceIdentityConfigPreservesSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "actual.conf")
	link := filepath.Join(dir, "config")
	original := []byte("Host target\n HostName example.com\n")
	updated := []byte("Host target\n IdentityFile /new/key\n HostName example.com\n")
	if err := os.WriteFile(target, original, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Skip(err)
	}
	if err := replaceIdentityConfig(link, original, updated); err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(link)
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatal("config symlink was replaced")
	}
	after, err := os.ReadFile(target)
	if err != nil || string(after) != string(updated) {
		t.Fatal("target was not updated")
	}
	info, err = os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0600 {
		t.Fatal("temporary config did not preserve owner-only access")
	}
	leftovers, err := filepath.Glob(filepath.Join(dir, ".sshm-key-setup-*"))
	if err != nil || len(leftovers) != 0 {
		t.Fatal("temporary files remain")
	}
}

func TestReplaceIdentityConfigRejectsConcurrentEdit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config")
	newer := []byte("Host target\n HostName changed.example.com\n")
	if err := os.WriteFile(path, newer, 0600); err != nil {
		t.Fatal(err)
	}
	err := replaceIdentityConfig(path, []byte("stale config"), []byte("replacement"))
	if err == nil || !strings.Contains(err.Error(), "changed during setup") {
		t.Fatalf("expected concurrent edit error, got %v", err)
	}
	after, err := os.ReadFile(path)
	if err != nil || string(after) != string(newer) {
		t.Fatal("newer settings overwritten")
	}
	leftovers, err := filepath.Glob(filepath.Join(dir, ".sshm-key-setup-*"))
	if err != nil || len(leftovers) != 0 {
		t.Fatal("temporary files remain after failure")
	}
}
