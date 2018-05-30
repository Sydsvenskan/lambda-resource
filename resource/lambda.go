package resource

import (
	"encoding/json"
	"fmt"
	"io/ioutil"

	"github.com/Zipcar/lambda-resource/concourse"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/lambda"
	"github.com/pkg/errors"
)

// LambdaErrorType is the type of Lambda invoke errors
type LambdaErrorType string

const (
	// UnhandledError covers all errors that don't get passed to the callback
	// function. I.e. syntax errors and unhandled exceptions.
	UnhandledError LambdaErrorType = "Unhandled"
	// HandledError is what occurs when an error gets passed to the callback.
	HandledError LambdaErrorType = "Handled"
)

// Source is the Lambda resource source definition
type Source struct {
	// KeyID is the AWS access key id
	KeyID string `json:"access_key_id"`
	// AccessKey is the AWS access key
	AccessKey string `json:"secret_access_key"`
	// RegionName is the AWS region that your lambda function is in
	RegionName string `json:"region_name"`
	// FunctionName is the name of your Lambda function
	FunctionName string `json:"function_name"`
	// Alias can be used with in and check to track changes to a specific alias
	// of a function.
	Alias *string `json:"alias"`
}

// PayloadSpec specifies a payload that should be used to invoke the
// lambda function.
type PayloadSpec struct {
	// Payload is used to specify the function payload as inline JSON
	// in params.
	Payload interface{} `json:"payload"`
	// PayloadFile is used to load the payload from an input file.
	PayloadFile *string `json:"payload_file"`
}

// LambdaClient creates a lambda client from the source config
func LambdaClient(s Source) *lambda.Lambda {
	return lambda.New(session.New(&aws.Config{
		Region: &s.RegionName,
		Credentials: credentials.NewStaticCredentials(
			s.KeyID, s.AccessKey, "",
		),
	}))
}

// FunctionError returned by Lambda when something goes wrong during invocation
type FunctionError struct {
	Message    string          `json:"errorMessage"`
	Type       LambdaErrorType `json:"errorType"`
	StackTrace []string        `json:"stackTrace"`
}

// Error returns a description of the error
func (fe FunctionError) Error() string {
	return fmt.Sprintf(
		"function failed to run because of a %q error: %q, %v",
		fe.Type, fe.Message, fe.StackTrace,
	)
}

// LambdaError returns the underlying lambda error
func (fe *FunctionError) LambdaError() *FunctionError {
	return fe
}

// InvokeFunction invokes a lambda function
func InvokeFunction(
	api *lambda.Lambda, source Source, alias *string, payload PayloadSpec,
) (*lambda.InvokeOutput, error) {
	name := source.FunctionName

	data, err := payloadData(payload)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get payload data")
	}

	if len(data) == 0 {
		return nil, nil
	}

	if alias != nil {
		name += ":" + *alias
	}

	result, err := api.Invoke(&lambda.InvokeInput{
		FunctionName: &name,
		Payload:      data,
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to invoke function")
	}

	if result.FunctionError != nil {
		var functionError FunctionError
		if err := json.Unmarshal(result.Payload, &functionError); err != nil {
			return result, errors.Wrapf(err,
				"failed to unmarshal function error %q", *result.FunctionError)
		}

		return result, functionError
	}

	return result, nil
}

// PersistResult writes our a "result.json" and "result.payload.json" to the
// context output directory.
func PersistResult(
	ctx *concourse.CommandContext, result *lambda.InvokeOutput,
) error {
	if err := ctx.JSON("result.json", result); err != nil {
		return errors.Wrap(err,
			"failed to persist invocation result")
	}
	if err := ctx.File("result.payload.json", result.Payload); err != nil {
		return errors.Wrap(err, "failed to persist result payload")
	}
	return nil
}

// HasPayload checks if the
func (spec *PayloadSpec) HasPayload() bool {
	return spec.Payload != nil || spec.PayloadFile != nil
}

func payloadData(spec PayloadSpec) ([]byte, error) {
	if spec.Payload != nil {
		return json.Marshal(spec.Payload)
	}

	if spec.PayloadFile != nil {
		data, err := ioutil.ReadFile(*spec.PayloadFile)
		if err != nil {
			err = errors.Wrap(err, "failed to read payload file")
		}
		return data, err
	}

	return nil, nil
}
