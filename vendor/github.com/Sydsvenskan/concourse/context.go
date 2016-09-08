package concourse

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"

	"github.com/pkg/errors"
)

// CommandContext is passed to the in, out, and check commands.
type CommandContext struct {
	directory   string
	commandName string
	in          io.Reader
	out         io.Writer
	Log         io.Writer
}

// Resource is the default resource implementation
type Resource struct {
	Check CommandHandler
	In    CommandHandler
	Out   CommandHandler
}

// CheckHandler returns the registered check handler
func (r *Resource) CheckHandler() CommandHandler {
	return r.Check
}

// InHandler returns the registered get handler
func (r *Resource) InHandler() CommandHandler {
	return r.In
}

// OutHandler returns the registered put handler
func (r *Resource) OutHandler() CommandHandler {
	return r.Out
}

// ResourceHandler provides handler implementations
type ResourceHandler interface {
	CheckHandler() CommandHandler
	InHandler() CommandHandler
	OutHandler() CommandHandler
}

// CommandHandler handles a /opt/resource/{in,out,check} concourse command
type CommandHandler interface {
	HandleCommand(ctx *CommandContext) (*CommandResponse, error)
}

// ResourceVersion is arbitrary version info that identifies ar
type ResourceVersion map[string]string

// CommandResponse is what get's returned to Concourse (JSON on Stdout) when
// we've successfully completed the task.
type CommandResponse struct {
	Version  ResourceVersion           `json:"version"`
	Versions []ResourceVersion         `json:"-"`
	Metadata []CommandResponseMetadata `json:"metadata"`
}

// VersionsResponse is a utility function to create a check response
func VersionsResponse(version ...ResourceVersion) []ResourceVersion {
	return version
}

// AddMeta is a helper function for appending new metadata entries
func (cr *CommandResponse) AddMeta(name, value string) {
	cr.Metadata = append(cr.Metadata, CommandResponseMetadata{
		Name:  name,
		Value: value,
	})
}

// CommandResponseMetadata is a metadata entry in our CommandResponse.
type CommandResponseMetadata struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// NewContext creates a new context
func NewContext(
	args []string, in io.Reader, out io.Writer, log io.Writer,
) (*CommandContext, error) {
	ctx := &CommandContext{
		in:  in,
		out: out,
		Log: log,
	}

	ctx.commandName = filepath.Base(args[0])
	if len(args) > 1 {
		ctx.directory = args[1]
	}

	return ctx, nil
}

// Handle the command
func (ctx *CommandContext) Handle(handler ResourceHandler) {
	var cmdHandler CommandHandler
	decoder := json.NewDecoder(ctx.in)

	switch ctx.commandName {
	case "out":
		cmdHandler = handler.OutHandler()
		if handler == nil {
			fmt.Fprintf(ctx.Log,
				"the command %q is not implemented", ctx.commandName,
			)
			_, _ = ctx.out.Write([]byte("{}"))
			return
		}
	case "in":
		cmdHandler = handler.InHandler()
		if handler == nil {
			fmt.Fprintf(ctx.Log,
				"the command %q is not implemented", ctx.commandName,
			)
			_, _ = ctx.out.Write([]byte("{}"))
			return
		}
	case "check":
		cmdHandler = handler.CheckHandler()
		if handler == nil {
			fmt.Fprintf(ctx.Log,
				"the command %q is not implemented", ctx.commandName,
			)
			_, _ = ctx.out.Write([]byte("[]"))
			return
		}
	default:
		fmt.Fprintf(ctx.Log, "unknown command: %q", ctx.commandName)
		os.Exit(1)
	}

	// Decode the input as the selected command
	if err := decoder.Decode(cmdHandler); err != nil {
		fmt.Fprintln(ctx.Log, "failed to decode input json", err.Error())
	}

	// Change directory if specified
	if ctx.directory != "" {
		if err := os.Chdir(ctx.directory); err != nil {
			fmt.Fprintf(ctx.Log, "failed to change directory to %q\n", ctx.directory)
		}
	}

	// Run the command handler
	res, err := cmdHandler.HandleCommand(ctx)
	if err != nil {
		fmt.Fprintln(ctx.Log, "failed to run command", err.Error())
		os.Exit(1)
	}

	// Encode our output, with some special-casing for check
	encoder := json.NewEncoder(ctx.out)
	if ctx.commandName == "check" {
		err = encoder.Encode(res.Versions)
	} else {
		err = encoder.Encode(res)
	}
	if err != nil {
		fmt.Fprintln(ctx.Log, "failed to encode response", err.Error())
		os.Exit(1)
	}
}

// JSON encodes and writes out a JSON result in the output directory.
func (ctx *CommandContext) JSON(path string, obj interface{}) error {
	data, err := json.Marshal(obj)
	if err != nil {
		return errors.Wrapf(err,
			"failed to marshal JSON for writing to %s/%s", ctx.directory, path)
	}
	return ctx.File(path, data)
}

// File writes out a file in the output directory.
func (ctx *CommandContext) File(name string, data []byte) error {
	fullPath := path.Join(ctx.directory, name)
	return errors.Wrapf(
		ioutil.WriteFile(fullPath, data, 0666),
		"failed to write data to %s", fullPath,
	)
}
