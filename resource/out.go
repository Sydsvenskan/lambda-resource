package resource

import (
	"archive/zip"
	"bytes"
	"crypto/md5"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"strconv"
	"time"

	"github.com/Sydsvenskan/concourse"
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

// LambdaSource returns the lambda source information
func (cmd *OutCommand) LambdaSource() *Source {
	return &cmd.Source
}

// PutParams is the params used when put:ing a resource.
type PutParams struct {
	// ZipFile is a path to a zip archive containing the function code.
	ZipFile *string `json:"zip_file"`
	// CodeDirectory is a path
	CodeDirectory *string `json:"code_dir"`
	// Alias is used to "tag" a function with f.ex. a "PROD" or "TEST" alias.
	Alias *string `json:"alias"`
	// Version can be used together with "Alias" to tag a specific version
	// without updating the function code.
	Version     *string `json:"version"`
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

		fmt.Fprintf(os.Stderr,
			"successfully updated function to version %s (sha256: %s)\n",
			*config.Version, *config.CodeSha256)

		if err := ctx.JSON("function.json", config); err != nil {
			return nil, errors.Wrap(err,
				"failed to persist function configuration")
		}

		// Store the version so that it can be used by the alias "tagging"
		version = config.Version

		// Parse the version so that we can make it part of our output
		versionNumber, err := strconv.Atoi(*config.Version)
		if err != nil {
			return nil, errors.Wrap(err,
				"could not parse function version")
		}
		resp.Version = Version{
			Version: versionNumber,
			CodeSha: *config.CodeSha256,
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
	} else {
		resp.Version = Version{Timestamp: time.Now().Unix()}
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

		fmt.Fprintf(os.Stderr,
			"successfully set the alias %s to version %s\n",
			*aliasConfig.Name, *aliasConfig.FunctionVersion)

		if err := ctx.JSON("alias.json", aliasConfig); err != nil {
			return resp, errors.Wrap(err,
				"failed to persist function configuration")
		}
	}

	return resp, nil
}

func hasCodePayload(p PutParams) bool {
	return p.ZipFile != nil || p.CodeDirectory != nil
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
		rootInfo, err := os.Stat(*p.CodeDirectory)
		if err != nil {
			return nil, errors.Wrap(err, "could not get code directory info")
		}

		var buf bytes.Buffer
		w := zip.NewWriter(&buf)
		if err := zipRecurse(w, *p.CodeDirectory, "", rootInfo); err != nil {
			return nil, errors.Wrap(err, "failed to create zip payload")
		}
		_ = w.Close()

		fmt.Fprintf(os.Stderr, "Zip file hash %x", md5.Sum(buf.Bytes()))
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
