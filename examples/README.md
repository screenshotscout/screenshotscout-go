# Examples

Set `SCREENSHOTSCOUT_ACCESS_KEY`, then run an example from the repository root:

```sh
go run ./examples/binary
go run ./examples/json
go run ./examples/capture-url
```

The `capture-url` example also reads `SCREENSHOTSCOUT_SECRET_KEY` and signs the URL when it is set.

The example applications read their own environment and pass credentials explicitly. The SDK itself never reads credentials from environment variables.
