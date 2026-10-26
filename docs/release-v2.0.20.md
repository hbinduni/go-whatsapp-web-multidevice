# v2.0.20 dependency and security update

Prepared on 2026-10-26 for the independently maintained GoWA-SSE fork. This update retains the existing REST API and does not migrate to upstream GOWA v9.

| Component | Previous | Updated |
| --- | --- | --- |
| whatsmeow | `b572e5bcb92b` (2026-06-30) | `b2b1e71d45e2` (2026-10-17) |
| Go requirement and container builder | 1.25 | 1.27.3 |
| Alpine runtime and backup image | 3.20 | 3.24.3 |
| Fiber v2 | 2.52.13 | 2.52.16 |
| fasthttp | 1.71.0 | 1.77.0 |
| SQLite driver | 1.14.45 | 1.14.52 |
| MinIO client | 7.2.0 | 7.4.1 |
| `golang.org/x/image` | 0.41.0 | 0.47.0 |
| `golang.org/x/net` | 0.56.0 | 0.61.0 |
| `golang.org/x/crypto` | 0.53.0 | 0.58.0 |
| `golang.org/x/text` | 0.38.0 | 0.43.0 |
| `github.com/klauspost/compress` | 1.18.6 | 1.20.3 |

Other compatible dependency updates and checksums are recorded in `src/go.mod` and `src/go.sum`. The release workflow pulls current base images, and both Dockerfiles upgrade Alpine packages during the build. Binary release workflows read the Go version from `src/go.mod`.

The whatsmeow update requires new arguments for blocklist operations and a structured input for profile About updates. Blocklist JSON keeps its existing `data.DHash` and `data.JIDs` fields, using phone-number JIDs when available and LIDs otherwise. Empty and nil responses keep their previous JSON forms.

Whatsmeow automatically applies schema migrations 15 and 16 when opening a session database. These add `companion_meta_nonce` to `whatsmeow_device` and `wasa_root_secret_id` to `whatsmeow_chat_settings`. A temporary SQLite database at schema 14 was upgraded through the application's database initializer; its synthetic device and session records were preserved, both columns were present, and a repeated upgrade succeeded. PostgreSQL migration and live WhatsApp behavior were not exercised.

Validation completed locally:

- `go fmt ./...`, `go vet ./...`, `go test ./...`, `go build ./...`, and `go mod verify` passed.
- Blocklist JSON regression tests failed before the compatibility adapter and passed afterward.
- `actionlint` passed for both modified release workflows.
- `govulncheck v1.9.0 -mode=binary -show verbose` on the updated Go 1.27.3 binary reported zero symbol or imported-package vulnerabilities. The baseline binary had nine symbol matches in `x/net` and `x/image`; the update also cleared its package findings in `x/text` and `klauspost/compress`.
- The remaining module-only advisory, [GO-2026-5932](https://pkg.go.dev/vuln/GO-2026-5932), concerns `x/crypto/openpgp`. GOWA does not import that package. Source-mode scanning failed because of the scanner's source-parser incompatibility with Go 1.27, so the binary scan is the recorded evidence.
- `make check-all` remains blocked by existing `errcheck` findings. With output limits disabled, the original commit and updated tree have the same 53 findings, with none added or removed.

Container tags were verified to support linux/amd64. No container was built or scanned, and no live service was changed. The container build and runtime validation remain release steps.

Version sources: [whatsmeow changes](https://github.com/tulir/whatsmeow/compare/b572e5bcb92b...b2b1e71d45e2), [Go releases](https://go.dev/dl/), [Alpine support policy](https://alpinelinux.org/releases/).
