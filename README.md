# tapas

Read Tapas.io comics and novel series

`tapas` is a single pure-Go binary. It reads public Tapas.io data via the
public XML sitemaps and HTML scraping. No API key, nothing to run alongside it.

The same package is also a [resource-URI driver](#use-it-as-a-resource-uri-driver),
so a host program like [ant](https://github.com/tamnd/ant) can address
tapas as `tapas://` URIs.

## Install

```bash
go install github.com/tamnd/tapas-cli/cmd/tapas@latest
```

Or grab a prebuilt binary from the [releases](https://github.com/tamnd/tapas-cli/releases), or run
the container image:

```bash
docker run --rm ghcr.io/tamnd/tapas:latest --help
```

## Usage

```bash
tapas top                             # list top comic series
tapas top --type novel -n 5           # top 5 novel series
tapas search romance                  # series with "romance" in slug
tapas search action -n 20             # up to 20 action series
tapas series MATCHPOINT               # fetch series details by slug
tapas series 329873                   # fetch series details by id
tapas episodes 329873 -n 10           # list 10 episodes of series 329873
tapas series MATCHPOINT -o json       # as JSON, ready for jq
tapas --help                          # the whole command tree
```

Every command shares one output contract: `-o table|json|jsonl|csv|tsv|url|raw`,
`--fields` to pick columns, `--template` for a custom line, and `-n` to limit.
The default adapts to where output goes (a table on a terminal, JSONL in a
pipe), so the same command reads well by hand and parses cleanly downstream.

## Commands

| Command | Description |
|---------|-------------|
| `tapas top` | List top series from the sitemap (most recently updated) |
| `tapas search <query>` | Search series by slug keyword |
| `tapas series <ref>` | Fetch series details by slug, id, or URL |
| `tapas episodes <id>` | List episodes for a series |

## Serve it

The same operations are available over HTTP and as an MCP tool set for agents,
with no extra code:

```bash
tapas serve --addr :7777    # GET /v1/series/<slug>  returns NDJSON
tapas mcp                   # speak MCP over stdio
```

## Use it as a resource-URI driver

`tapas` registers a `tapas` domain the way a program registers a
database driver with `database/sql`. A host enables it with one blank import:

```go
import _ "github.com/tamnd/tapas-cli/tapas"
```

Then [ant](https://github.com/tamnd/ant) (or any program that links the package)
dereferences `tapas://` URIs without knowing anything about Tapas.io:

```bash
ant get tapas://series/MATCHPOINT      # fetch the series record
ant url tapas://series/MATCHPOINT      # the live https URL
```

## Development

```
cmd/tapas/   thin main: hands cli.NewApp to kit.Run
cli/                 assembles the kit App from the tapas domain
tapas/                the library: HTTP client, data models, and domain.go (the driver)
docs/                tago documentation site
```

```bash
make build      # ./bin/tapas
make test       # go test ./...
make vet        # go vet ./...
```

## Releasing

Push a version tag and GitHub Actions runs GoReleaser, which builds the
archives, Linux packages, the multi-arch GHCR image, checksums, SBOMs, and a
cosign signature:

```bash
git tag v0.1.0
git push --tags
```

The Homebrew and Scoop steps self-disable until their tokens exist, so the first
release works with no extra secrets.

## License

Apache-2.0. See [LICENSE](LICENSE).
