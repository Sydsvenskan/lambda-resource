# Lambda resource

[Concourse](https://concourse.ci/) resource for deploying and running lambda functions.

## Source configuration

* `access_key_id`: *Required*. The AWS access key id.
* `secret_access_key`: *Required*. The AWS access key secret.
* `region_name`: *Required*. The region the function is in.
* `function_name`: *Required*. The name of your function.
* `alias`: *Optional*. Alias to use for the resource, this is useful when you're *check*ing for new versions of an alias.

## Behaviour

### `check`: check for new versions of the function

AWS is polled for new released versions of the function (new version number). If the source configuration includes an alias it checks if the alias has been pointed to a new version.

If check is invoked for the first time the latest version of the function is returned.

### `in`: invoke the function

Invokes the function and stores the result in the destination directory as `result.json` (the response from Lamda) and `result.payload.json` (the result payload from your function). A payload must be specified 

#### Parameters

* `payload`: *Optional*. Arbitrary inline JSON that gets sent as the invocation payload.
* `payload_file`: *Optional*. A file that contains the payload to send to your lambda function.
* `alias`: *Optional*. The alias of the function to invoke.

Either `payload` or `payload_file` must be present.

### `out`: publish a new version of the function

Publishes a new version of the function. `zip_file` or `code_dir` are used to upload new function code. `alias` is used to tag function versions and can be used either when uploading code, or with one of the `version*` parameters to tag an existing version.

#### Parameters

* `zip_file`: *Optional*. A zip file containing the function code.
* `code_dir`: *Optional*. A directory containing the function code.
* `alias`: *Optional*. An alias to tag the new version with. Defaults to the source alias if omitted. If no alias is present here or in source the new version will just be published as is.
* `version`: *Optional*. If no function code has been provided 'version' can be specified together with `alias` to tag an existing version.
* `version_file`: *Optional*. Load a version number from file. If no function code has been provided 'version_file' can be specified together with `alias` to tag an existing version.
