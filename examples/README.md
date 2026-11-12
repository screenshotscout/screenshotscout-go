# Examples

Set `SCREENSHOTSCOUT_ACCESS_KEY`, then run an example from the repository root:

```sh
go run ./examples/binary
go run ./examples/json
go run ./examples/capture-url
```

If `SCREENSHOTSCOUT_SECRET_KEY` is also set, the client signs its requests and the URL that `capture-url` prints.
