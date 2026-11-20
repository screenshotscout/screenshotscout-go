# Examples

Set `SCREENSHOTSCOUT_ACCESS_KEY`, then run an example from the repository root:

```sh
go run ./examples/binary
go run ./examples/json
go run ./examples/capture-url
go run ./examples/html
```

If `SCREENSHOTSCOUT_SECRET_KEY` is also set, the client signs its requests and the URL that `capture-url` prints. `html` renders an invoice to PDF and sends the HTML in a POST request, which can't be signed, so run it without `SCREENSHOTSCOUT_SECRET_KEY`.
