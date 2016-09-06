package main

import (
	"fmt"
	"os"

	"github.com/Sydsvenskan/concourse"
	"github.com/Sydsvenskan/lambda-resource/resource"
)

func main() {
	context, err := concourse.NewContext(os.Args, os.Stdin, os.Stdout, os.Stderr)
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed to create command context:", err.Error())
		os.Exit(1)
		return
	}

	context.Handle(&concourse.Resource{
		Check: &resource.CheckCommand{},
		In:    &resource.InCommand{},
		Out:   &resource.OutCommand{},
	})
}
