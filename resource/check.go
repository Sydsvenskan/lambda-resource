package resource

import (
	"sort"
	"strconv"

	"github.com/Zipcar/lambda-resource/concourse"
	"github.com/aws/aws-sdk-go/service/lambda"
	"github.com/pkg/errors"
)

// CheckCommand check-command payload
type CheckCommand struct {
	// Source definition
	Source Source `json:"source"`
	// Version information passed to the resource
	Version concourse.ResourceVersion `json:"version"`
}

func getVersionNumber(v concourse.ResourceVersion) *int {
	if v == nil {
		return nil
	}
	if s, ok := v["version"]; ok {
		v, err := strconv.Atoi(s)
		if err == nil {
			return &v
		}
	}
	return nil
}

// HandleCommand runs the command
func (cmd *CheckCommand) HandleCommand(ctx *concourse.CommandContext) (
	*concourse.CommandResponse, error,
) {
	api := LambdaClient(cmd.Source)

	var newVersions []concourse.ResourceVersion
	incomingVersion := getVersionNumber(cmd.Version)

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

				itemVersion, err := strconv.Atoi(*v.Version)
				if err != nil {
					return nil, errors.Wrap(err, "failed to parse function version")
				}

				if incomingVersion == nil || itemVersion > *incomingVersion {
					newVersions = append(newVersions, concourse.ResourceVersion{
						"version": *v.Version,
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

		itemVersion, err := strconv.Atoi(*config.Version)
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse function version")
		}

		if incomingVersion == nil || itemVersion > *incomingVersion {
			newVersions = append(newVersions, concourse.ResourceVersion{
				"version": *config.Version,
				"alias":   *cmd.Source.Alias,
			})
		}
	}

	return &concourse.CommandResponse{
		Versions: newVersions,
	}, nil
}

// ByVersion sorts a slice of Versions by version number
type ByVersion []concourse.ResourceVersion

func (l ByVersion) Len() int      { return len(l) }
func (l ByVersion) Swap(i, j int) { l[i], l[j] = l[j], l[i] }
func (l ByVersion) Less(i, j int) bool {
	iv, err := strconv.Atoi(l[i]["version"])
	if err != nil {
		panic(err.Error())
	}
	jv, err := strconv.Atoi(l[j]["version"])
	if err != nil {
		panic(err.Error())
	}
	return iv < jv
}
