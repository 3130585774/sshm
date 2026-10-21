package ui

import (
	"errors"
	"strings"
	"testing"

	"github.com/Gu1llaum-3/sshm/internal/config"
	tea "github.com/charmbracelet/bubbletea"
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
		updated, _ := m.Update(keySetupDoneMsg{err: err, hosts: hosts, host: "server1", configured: !failure})
		got := updated.(Model)
		if got.showingError != failure {
			t.Fatal("incorrect error state")
		}
		if len(got.filteredHosts) != 1 || got.filteredHosts[0].Identity != "/selected" {
			t.Fatal("reload/filter failed")
		}
	}
}

func TestKeySetupFeedback(t *testing.T) {
	for _, tc := range []struct {
		name       string
		configured bool
		err        error
		want       string
	}{
		{"success", true, nil, "Key login configured for server2"},
		{"cancelled", false, nil, "Key setup cancelled for server2"},
		{"failure", false, errors.New("verification failed"), "verification failed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := createTestModel()
			m.table.SetCursor(3)
			updated, _ := m.Update(keySetupDoneMsg{host: "server2", configured: tc.configured, err: tc.err, hosts: m.hosts})
			got := updated.(Model)
			if !strings.Contains(got.View(), tc.want) {
				t.Fatalf("feedback missing: %s", got.View())
			}
			if selected := got.table.SelectedRow(); len(selected) == 0 || !strings.Contains(selected[0], "server2") {
				t.Fatal("selected host was lost")
			}
			if tc.err != nil && got.noticeMessage != "" {
				t.Fatal("failure displayed success/cancellation notice")
			}
			updated, _ = got.Update(tea.KeyMsg{Type: tea.KeyDown})
			if updated.(Model).noticeMessage != "" {
				t.Fatal("notice was not dismissed")
			}
		})
	}
}
