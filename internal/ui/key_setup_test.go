package ui

import (
	"errors"
	"github.com/Gu1llaum-3/sshm/internal/config"
	tea "github.com/charmbracelet/bubbletea"
	"testing"
)

func TestKeySetupShortcut(t *testing.T) {
	for _, tc := range []struct {
		name           string
		search, delete bool
		want           bool
	}{
		{"table", false, false, true}, {"search", true, false, false}, {"delete", false, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := createTestModel()
			m.searchMode = tc.search
			m.deleteMode = tc.delete
			_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("K")})
			if (cmd != nil) != tc.want {
				t.Fatalf("command present=%v", cmd != nil)
			}
		})
	}
}

func TestKeySetupDone(t *testing.T) {
	for _, failure := range []bool{false, true} {
		m := createTestModel()
		m.searchInput.SetValue("server1")
		var err error
		if failure {
			err = errors.New("verification failed")
		}
		hosts := []config.SSHHost{{Name: "server1", Identity: "/selected"}, {Name: "server2"}}
		updated, _ := m.Update(keySetupDoneMsg{err: err, hosts: hosts})
		got := updated.(Model)
		if got.showingError != failure {
			t.Fatal("incorrect error state")
		}
		if len(got.filteredHosts) != 1 || got.filteredHosts[0].Identity != "/selected" {
			t.Fatal("reload/filter failed")
		}
	}
}
