package keysetup

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestInstallPreservesAndDeduplicatesKeys(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("installation requires a Linux target")
	}
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("requires POSIX shell")
	}
	home := t.TempDir()
	dir := filepath.Join(home, ".ssh")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "authorized_keys")
	existing := "# existing comment\nssh-ed25519 OLD existing\n"
	if err := os.WriteFile(path, []byte(existing), 0600); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		c := exec.Command("sh", "-c", installScript("ssh-ed25519 NEW"))
		c.Env = append(os.Environ(), "HOME="+home)
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("%v: %s", err, out)
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(data), existing) || strings.Count(string(data), "ssh-ed25519 NEW") != 1 {
		t.Fatalf("unexpected keys: %s", data)
	}
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0600 {
		t.Fatal("incorrect authorized_keys permissions")
	}
}

func TestInstallRefusesSymlink(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("installation requires a Linux target")
	}
	home := t.TempDir()
	dir := filepath.Join(home, ".ssh")
	os.Mkdir(dir, 0700)
	victim := filepath.Join(home, "victim")
	os.WriteFile(victim, []byte("unchanged"), 0600)
	if err := os.Symlink(victim, filepath.Join(dir, "authorized_keys")); err != nil {
		t.Skip(err)
	}
	c := exec.Command("sh", "-c", installScript("ssh-ed25519 NEW"))
	c.Env = append(os.Environ(), "HOME="+home)
	if err := c.Run(); err == nil {
		t.Fatal("accepted symlink")
	}
	data, _ := os.ReadFile(victim)
	if string(data) != "unchanged" {
		t.Fatal("modified symlink target")
	}
}

func TestAcceptedKey(t *testing.T) {
	for _, tc := range []struct {
		log  string
		want bool
	}{
		{"debug1: Server accepts key: /key ED25519 SHA256:chosen explicit\n", true},
		{"debug1: Server accepts key: /key ED25519 SHA256:chosen explicit\ndebug1: Server accepts key: /other ED25519 SHA256:other explicit", false},
		{"", false},
	} {
		if got := acceptedKey(tc.log, "SHA256:chosen"); got != tc.want {
			t.Errorf("%q got %v", tc.log, got)
		}
	}
}

func TestWizardFailuresLeaveConfigUnchanged(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a POSIX executable fixture")
	}
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("requires POSIX shell")
	}
	keygen, err := exec.LookPath("ssh-keygen")
	if err != nil {
		t.Skip(err)
	}
	for _, stage := range []string{"install", "verify", "different-key", "cancel"} {
		t.Run(stage, func(t *testing.T) {
			dir := t.TempDir()
			key := filepath.Join(dir, "key")
			if out, err := exec.Command(keygen, "-q", "-t", "ed25519", "-N", "", "-f", key).CombinedOutput(); err != nil {
				t.Fatalf("%v: %s", err, out)
			}
			configPath := filepath.Join(dir, "config")
			original := "Host target\n HostName example.com\n"
			if err := os.WriteFile(configPath, []byte(original), 0600); err != nil {
				t.Fatal(err)
			}
			script := `#!/bin/sh
for arg do
 if [ "$arg" = -G ]; then printf 'user test\nhostname example.com\nport 22\n'; exit 0; fi
done
for arg do
 if [ "$arg" = -v ]; then
`
			if stage == "verify" {
				script += "exit 1\n"
			} else {
				script += "echo 'debug1: Server accepts key: /other ED25519 SHA256:other explicit' >&2\nexit 0\n"
			}
			script += "fi\ndone\n"
			if stage == "install" {
				script += "exit 1\n"
			} else {
				script += "exit 0\n"
			}
			fake := filepath.Join(dir, "ssh")
			if err := os.WriteFile(fake, []byte(script), 0700); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
			w := New("target", configPath)
			w.Configured = true // Reusing a wizard must not retain a prior success.
			answer := "y"
			if stage == "cancel" {
				answer = "n"
			}
			w.SetStdin(strings.NewReader(key + "\n" + answer + "\n"))
			w.SetStdout(io.Discard)
			w.SetStderr(io.Discard)
			err = w.Run()
			if w.Configured {
				t.Fatal("unsuccessful setup reported configured")
			}
			if stage == "cancel" && err != nil {
				t.Fatal(err)
			}
			if stage != "cancel" && err == nil {
				t.Fatal("failure incorrectly reported success")
			}
			after, err := os.ReadFile(configPath)
			if err != nil {
				t.Fatal(err)
			}
			if string(after) != original {
				t.Fatal("failure changed local config")
			}
		})
	}
}

func TestWizardSuccessSavesIdentityAndReportsConfigured(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a POSIX executable fixture")
	}
	keygen, err := exec.LookPath("ssh-keygen")
	if err != nil {
		t.Skip(err)
	}
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "app"))
	key := filepath.Join(dir, "key")
	if out, err := exec.Command(keygen, "-q", "-t", "ed25519", "-N", "", "-f", key).CombinedOutput(); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	fingerprintOut, err := exec.Command(keygen, "-lf", key, "-E", "sha256").Output()
	if err != nil {
		t.Fatal(err)
	}
	fingerprint := strings.Fields(string(fingerprintOut))[1]
	configPath := filepath.Join(dir, "config")
	original := "Host target\n HostName example.com\n"
	if err := os.WriteFile(configPath, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\nfor arg do\n" +
		"if [ \"$arg\" = -G ]; then printf 'user test\\nhostname example.com\\nport 22\\n'; exit 0; fi\n" +
		"if [ \"$arg\" = -v ]; then echo 'debug1: Server accepts key: /selected ED25519 " + fingerprint + " explicit' >&2; exit 0; fi\n" +
		"done\nexit 0\n"
	if err := os.WriteFile(filepath.Join(dir, "ssh"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	w := New("target", configPath)
	w.SetStdin(strings.NewReader(key + "\ny\n"))
	w.SetStdout(io.Discard)
	w.SetStderr(io.Discard)
	if err := w.Run(); err != nil {
		t.Fatal(err)
	}
	if !w.Configured {
		t.Fatal("verified and saved setup was not reported configured")
	}
	after, err := os.ReadFile(configPath)
	if err != nil || !strings.Contains(string(after), "IdentityFile") {
		t.Fatal("success did not save identity")
	}
}
