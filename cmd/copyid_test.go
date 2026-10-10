package cmd

import (
	"reflect"
	"testing"
)

func TestBuildCopyIDArgs(t *testing.T) {
	got := buildCopyIDArgs("h", "k.pub", "cfg")
	want := []string{"-i", "k.pub", "-F", "cfg", "h"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
	if got := buildCopyIDArgs("h", "", ""); !reflect.DeepEqual(got, []string{"h"}) {
		t.Fatalf("got %v", got)
	}
}
