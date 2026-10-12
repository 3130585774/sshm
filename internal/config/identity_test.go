package config

import (
	"os"
	"path/filepath"
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
