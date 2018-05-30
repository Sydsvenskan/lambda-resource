package resource

import (
	"fmt"
	"strconv"
	"time"

	"github.com/Zipcar/lambda-resource/concourse"
	"github.com/pkg/errors"
)

// InCommand in-command payload
type InCommand struct {
	// Source definition
	Source Source `json:"source"`
	// Params passed to the resource
	Params InParams `json:"params"`
	// Version is used in the implicit post `put` `get`
	Version concourse.ResourceVersion
}

// InParams is the params used when get:ing a resource (invoking a function).
type InParams struct {
	// PayloadSpec is the invoke payload
	PayloadSpec
	// Alias is the alias (if any) of the function that should be invoked
	Alias *string `json:"alias"`
}

// HandleCommand runs the in command
func (cmd *InCommand) HandleCommand(ctx *concourse.CommandContext) (
	*concourse.CommandResponse, error,
) {
	alias := cmd.Source.Alias
	if cmd.Params.Alias != nil {
		alias = cmd.Params.Alias
	}

	if cmd.Params.HasPayload() {
		api := LambdaClient(cmd.Source)

		result, err := InvokeFunction(
			api, cmd.Source, alias,
			cmd.Params.PayloadSpec,
		)
		if err != nil {
			return nil, err
		}
		if result != nil {
			fmt.Fprintln(ctx.Log, "successfully invoked function:")
			if _, err := ctx.Log.Write(result.Payload); err != nil {
				return nil, errors.Wrap(err, "failed to print payload")
			}

			return nil, errors.Wrap(
				PersistResult(ctx, result),
				"failed to persist invoke result",
			)
		}

		return &concourse.CommandResponse{
			Version: concourse.ResourceVersion{
				"timestamp": strconv.FormatInt(time.Now().Unix(), 10),
			},
		}, nil
	}

	if cmd.Version != nil {
		if err := ctx.File("version", []byte(cmd.Version["version"])); err != nil {
			return nil, errors.Wrap(err, "failed to persist version")
		}
	}

	return &concourse.CommandResponse{
		Version: cmd.Version,
	}, nil
}
