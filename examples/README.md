# Examples

Set `SCREENSHOTSCOUT_ACCESS_KEY`, then run an example from the repository root:

```sh
go run ./examples/binary
go run ./examples/json
go run ./examples/capture-url
```

Set `SCREENSHOTSCOUT_SECRET_KEY` as well when signed requests are enabled for the API key.

The example applications read their own environment and pass credentials explicitly. The SDK itself never reads credentials from environment variables.
