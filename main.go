package main

import (
	"os"

	"github.com/Gu1llaum-3/sshm/cmd"
	"github.com/Gu1llaum-3/sshm/internal/credentials"
)

func main() {
	// ssh may re-invoke this binary as its SSH_ASKPASS helper.
	if handled, code := credentials.MaybeRunAskpass(os.Args[1:]); handled {
		os.Exit(code)
	}
	cmd.Execute()
}
