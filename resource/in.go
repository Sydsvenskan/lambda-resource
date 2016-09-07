package resource

import (
	"strconv"
	"time"

	"github.com/Sydsvenskan/concourse"
	"github.com/pkg/errors"
)

// InCommand in-command payload
type InCommand struct {
	// Source definition
	Source Source `json:"source"`
	// Params passed to the resource
	Params InParams `json:"params"`
}

// InParams is the params used when get:ing a resource (invoking a function).
type InParams struct {
	// PayloadSpec is the invoke payload
	PayloadSpec
	// Alias is the alias (if any) of the function that should be invoked
	Alias *string `json:"alias"`
}

// LambdaSource returns the lambda source information
func (cmd *InCommand) LambdaSource() *Source {
	return &cmd.Source
}

// HandleCommand runs the in command
func (cmd *InCommand) HandleCommand(ctx *concourse.CommandContext) (
	*concourse.CommandResponse, error,
) {
	alias := cmd.Source.Alias
	if cmd.Params.Alias != nil {
		alias = cmd.Params.Alias
	}

	api := LambdaClient(cmd.Source)

	result, err := InvokeFunction(
		api, cmd, alias,
		cmd.Params.PayloadSpec,
	)
	if err != nil {
		return nil, err
	}
	if result != nil {
		return nil, errors.Wrap(
			PersistResult(ctx, result),
			"failed to persist invoke result",
		)
	}

	if result == nil {
		return nil, errors.New(
			"function wasn't invoked, did you specify a valid payload?",
		)
	}

	return &concourse.CommandResponse{
		Version: concourse.ResourceVersion{
			"timestamp": strconv.FormatInt(time.Now().Unix(), 10),
		},
	}, nil
}
