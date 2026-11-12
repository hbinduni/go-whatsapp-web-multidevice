# v2.0.22 dependency security update

This release updates the Go toolchain and affected dependencies while retaining the current Whatsmeow version, REST API and session database schema.

| Component | Previous | Updated |
| --- | --- | --- |
| Go requirement and container builder | 1.27.3 | 1.27.4 |
| `golang.org/x/net` | 0.61.0 | 0.62.0 |
| `golang.org/x/crypto` | 0.58.0 | 0.59.0 |
| Required companion modules: `x/sync`, `x/sys`, `x/text` | 0.24.0, 0.49.0, 0.43.0 | 0.25.0, 0.50.0, 0.44.0 |

The updates address [GO-2026-6859](https://pkg.go.dev/vuln/GO-2026-6859), [GO-2026-6860](https://pkg.go.dev/vuln/GO-2026-6860), [GO-2026-6866](https://pkg.go.dev/vuln/GO-2026-6866) and the module-only [GO-2026-5932](https://pkg.go.dev/vuln/GO-2026-5932). The baseline source scan reported two symbol matches, one imported-package finding and one required-module finding.

The release workflow now checks both source reachability and the complete module graph with Govulncheck 1.9.0 before building the image. Module scanning also fails on advisories in packages the application does not import. Use the Go version declared in `src/go.mod`; an older locally installed scanner may fail to parse Go 1.27 sources.

Local validation on 12 November 2026:

- Source and module scans report no vulnerabilities.
- `go test -race ./...`, `go fmt ./...`, `go vet ./...`, `go build ./...` and `go mod verify` pass.
- `actionlint` passes for the modified release workflow.
- `make check-all` reaches existing lint failures. Unrestricted lint reports have the same 109 findings before and after this change. Automatic lint edits were reverted to keep this release limited to dependencies and the release gate.

The container build still pulls its base images and upgrades Alpine packages. Container audit and deployment verification are separate from the source audit above.
