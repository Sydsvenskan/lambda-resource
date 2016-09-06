package resource

import (
	"sort"
	"strconv"

	"github.com/Sydsvenskan/concourse"
	"github.com/aws/aws-sdk-go/service/lambda"
	"github.com/pkg/errors"
)

// CheckCommand check-command payload
type CheckCommand struct {
	// Source definition
	Source Source `json:"source"`
	// Params passed to the resource
	Version *Version `json:"version"`
}

// LambdaSource returns the lambda source information
func (cmd *CheckCommand) LambdaSource() *Source {
	return &cmd.Source
}

// HandleCommand runs the command
func (cmd *CheckCommand) HandleCommand(ctx *concourse.CommandContext) (
	*concourse.CommandResponse, error,
) {
	api := LambdaClient(cmd.Source)
	var newVersions []Version

	if cmd.Source.Alias == nil {
		req := lambda.ListVersionsByFunctionInput{
			FunctionName: &cmd.Source.FunctionName,
		}
		for {
			versions, err := api.ListVersionsByFunction(&req)
			if err != nil {
				return nil, errors.Wrap(err, "failed to list versions")
			}

			for _, v := range versions.Versions {
				if *v.Version == "$LATEST" {
					continue
				}

				version, err := strconv.Atoi(*v.Version)
				if err != nil {
					continue
				}

				if cmd.Version == nil || version > cmd.Version.Version {
					newVersions = append(newVersions, Version{
						Version: version,
						CodeSha: *v.CodeSha256,
					})
				}
			}
			if versions.NextMarker == nil {
				break
			}
			req.Marker = versions.NextMarker
		}

		sort.Sort(ByVersion(newVersions))

		if cmd.Version == nil {
			newVersions = newVersions[len(newVersions)-1:]
		}
	} else {
		config, err := api.GetFunctionConfiguration(&lambda.GetFunctionConfigurationInput{
			FunctionName: &cmd.Source.FunctionName,
			Qualifier:    cmd.Source.Alias,
		})
		if err != nil {
			return nil, errors.Wrap(err, "failed to check configuration")
		}

		version, err := strconv.Atoi(*config.Version)
		if err != nil {
			return nil, errors.Wrap(err,
				"could not parse function version")
		}

		if cmd.Version == nil || version != cmd.Version.Version {
			newVersions = append(newVersions, Version{
				Version: version,
				CodeSha: *config.CodeSha256,
			})
		}
	}

	// Some annoying copying here, might have to change the interface for
	// the check command after all... but I'm not sure what I would change it to.
	resp := concourse.CommandResponse{
		Versions: make([]concourse.ResourceVersion, len(newVersions)),
	}
	for i, v := range newVersions {
		resp.Versions[i] = v
	}
	return &resp, nil
}

// ByVersion sorts a slice of Versions by version number
type ByVersion []Version

func (l ByVersion) Len() int           { return len(l) }
func (l ByVersion) Swap(i, j int)      { l[i], l[j] = l[j], l[i] }
func (l ByVersion) Less(i, j int) bool { return l[i].Version < l[j].Version }
