package resource

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

// Version is what we use to version the Lambda resource
type Version struct {
	// CodeSha is the sha256 hash of the uploaded function code.
	CodeSha string `json:"codesha,omitempty"`
	// Version is the function version assigned by lambda on publish.
	Version int `json:"version,omitempty"`
	// Timestamp is used as version result on function invokes.
	Timestamp int64 `json:"timestamp,omitempty"`
}

// LambdaCommand can return a lambda source
type LambdaCommand interface {
	LambdaSource() *Source
}
