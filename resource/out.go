package resource

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strconv"

	"github.com/Zipcar/lambda-resource/concourse"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/lambda"
	"github.com/pkg/errors"
)

// OutCommand out-command payload
type OutCommand struct {
	// Source definition
	Source Source `json:"source"`
	// Params passed to the resource
	Params PutParams `json:"params"`
}

// PutParams is the params used when put:ing a resource.
type PutParams struct {
	// ZipFile is a path to a zip archive containing the function code.
	ZipFile *string `json:"zip_file"`
	// CodeDirectory is a path to a directory containing the function
	// implementation
	CodeDirectory *string `json:"code_dir"`
	// CodeFile is a path to the file implementing the function
	CodeFile *string `json:"code_file"`
	// Alias is used to "tag" a function with f.ex. a "PROD" or "TEST" alias.
	Alias *string `json:"alias"`
	// Version can be used together with "Alias" to tag a specific version
	// without updating the function code.
	Version *string `json:"version"`
	// VersionFile is a file to read the version number from
	VersionFile *string `json:"version_file"`
}

// HandleCommand runs the in command
func (cmd *OutCommand) HandleCommand(ctx *concourse.CommandContext) (
	*concourse.CommandResponse, error,
) {
	version := cmd.Params.Version
	if cmd.Params.VersionFile != nil {
		versionData, err := ioutil.ReadFile(*cmd.Params.VersionFile)
		if err != nil {
			return nil, errors.Wrapf(
				err, "failed to read version file %q", *cmd.Params.VersionFile,
			)
		}
		loadedVersion := string(bytes.TrimRight(versionData, "\n\r"))
		version = &loadedVersion
	}

	// Version number sanity check
	if version != nil {
		if len(*version) == 0 {
			return nil, errors.New("empty version string")
		}
		if _, err := strconv.Atoi(*version); err != nil {
			return nil, errors.Wrapf(err, "%q is not a valid version integer", *version)
		}
	}

	api := LambdaClient(cmd.Source)
	resp := &concourse.CommandResponse{}

	if hasCodePayload(cmd.Params) {
		data, err := codePayload(cmd.Params)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get code payload data")
		}

		config, err := api.UpdateFunctionCode(&lambda.UpdateFunctionCodeInput{
			FunctionName: &cmd.Source.FunctionName,
			ZipFile:      data,
			Publish:      aws.Bool(true),
		})
		if err != nil {
			return nil, errors.Wrap(err, "failed to update function code")
		}

		fmt.Fprintf(ctx.Log,
			"successfully updated function to version %s (sha256: %s)\n",
			*config.Version, *config.CodeSha256)

		// Store the version so that it can be used by the alias "tagging"
		version = config.Version

		resp.Version = concourse.ResourceVersion{
			"version": *config.Version,
		}

		if err := ctx.File("version", []byte(*version)); err != nil {
			return nil, errors.Wrap(err,
				"failed to persist function configuration")
		}

		// Add some nice-to-have metadata
		resp.AddMeta("arn", *config.FunctionArn)
		resp.AddMeta("runtime", *config.Runtime)
		resp.AddMeta("timeout", strconv.FormatInt(*config.Timeout, 10))
		resp.AddMeta("memory", strconv.FormatInt(*config.MemorySize, 10))
	}

	// Tag the version with an alias
	if cmd.Params.Alias != nil && version != nil {

		aliasConfig, err := api.UpdateAlias(&lambda.UpdateAliasInput{
			FunctionName:    &cmd.Source.FunctionName,
			FunctionVersion: version,
			Name:            cmd.Params.Alias,
		})
		if err != nil {
			return resp, errors.Wrapf(err, "failed to set alias %q for the version %q",
				*cmd.Params.Alias, *version)
		}

		fmt.Fprintf(ctx.Log,
			"successfully set the alias %s to version %s\n",
			*aliasConfig.Name, *aliasConfig.FunctionVersion)

		if resp.Version == nil {
			resp.Version = concourse.ResourceVersion{
				"alias":   *cmd.Params.Alias,
				"version": *version,
			}
		} else {
			resp.Version["alias"] = *cmd.Params.Alias
		}
	}

	return resp, nil
}

func hasCodePayload(p PutParams) bool {
	return p.ZipFile != nil ||
		p.CodeDirectory != nil ||
		p.CodeFile != nil
}

func codePayload(p PutParams) ([]byte, error) {
	if p.ZipFile != nil {
		data, err := ioutil.ReadFile(*p.ZipFile)
		if err != nil {
			return nil, errors.Wrap(err, "could not read the function zip file")
		}
		return data, nil
	}

	if p.CodeDirectory != nil {
		dirPath, err := filepath.Abs(*p.CodeDirectory)
		if err != nil {
			return nil, errors.Wrapf(
				err, "failed to resolve absolute path for %q", *p.CodeDirectory,
			)
		}

		rootInfo, err := os.Stat(dirPath)
		if err != nil {
			return nil, errors.Wrapf(
				err, "could not get code directory %q info", dirPath,
			)
		}

		var buf bytes.Buffer
		w := zip.NewWriter(&buf)
		if err := zipRecurse(w, dirPath, "", rootInfo); err != nil {
			return nil, errors.Wrap(err, "failed to create zip payload")
		}
		_ = w.Close()

		return buf.Bytes(), nil
	}

	if p.CodeFile != nil {
		baseName := filepath.Base(*p.CodeFile)
		var buf bytes.Buffer
		w := zip.NewWriter(&buf)
		if err := zipHandleFile(w, *p.CodeFile, baseName); err != nil {
			return nil, errors.Wrap(err, "failed to create zip payload")
		}
		_ = w.Close()

		return buf.Bytes(), nil
	}

	return nil, nil
}

func zipRecurse(
	w *zip.Writer, dirPath string, archivePath string, directory os.FileInfo,
) error {
	if !directory.IsDir() {
		return fmt.Errorf("%q is not a directory", dirPath)
	}
	files, err := listDir(dirPath)
	if err != nil {
		return err
	}

	for _, info := range files {
		osFilePath := path.Join(dirPath, info.Name())
		zipFilePath := archivePath + info.Name()

		if info.IsDir() {
			if err := zipRecurse(w, osFilePath, zipFilePath+"/", info); err != nil {
				return err
			}
		} else {
			if err := zipHandleFile(w, osFilePath, zipFilePath); err != nil {
				return err
			}
		}
	}

	return nil
}

func zipHandleFile(w *zip.Writer, osFilePath, zipFilePath string) error {
	file, err := os.Open(osFilePath)
	if err != nil {
		return errors.Wrapf(err, "failed to open %q", osFilePath)
	}
	defer file.Close()

	fw, err := w.Create(zipFilePath)
	if err != nil {
		return errors.Wrapf(
			err, "failed to create archive file %q", zipFilePath,
		)
	}

	if _, err := io.Copy(fw, file); err != nil {
		return errors.Wrapf(err, "failed to write %q to archive", osFilePath)
	}

	return nil
}

func listDir(dir string) ([]os.FileInfo, error) {
	directory, err := os.Open(dir)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to open directory %q", dir)
	}
	defer directory.Close()

	files, err := directory.Readdir(0)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to list contents of %q", dir)
	}

	return files, nil
}
