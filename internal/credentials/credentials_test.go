package credentials

import "testing"

func TestMaybeRunAskpass(t *testing.T) {
	t.Setenv(hostEnv, "")
	if handled, _ := MaybeRunAskpass([]string{"x's password: "}); handled {
		t.Fatal("must not handle without host env")
	}
	t.Setenv(hostEnv, "h")
	for _, p := range []string{"Enter passphrase for key 'k': ", "Are you sure (yes/no)?", ""} {
		if handled, code := MaybeRunAskpass([]string{p}); !handled || code == 0 {
			t.Fatalf("prompt %q should be refused, got handled=%v code=%d", p, handled, code)
		}
	}
}
