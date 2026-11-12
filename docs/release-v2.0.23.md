# v2.0.23 dependency and container security update

This release includes the Go 1.27.4, `x/net` 0.62.0 and `x/crypto` 0.59.0 fixes described in [v2.0.22](release-v2.0.22.md), plus the findings from an authenticated scan of the previous production container.

- The runtime requires libexpat 2.10.0-r0 or newer and zlib 1.4-r0 or newer. These minimums invalidate the old cached installation layer and address the five Alpine-package findings reported for the previous image.
- Imaging is replaced by the pinned `kovidgoyal/imaging` commit `7175a815898331fbdeb77c1a8253b8f5decab77f`. It contains the [palette bounds fix](https://github.com/disintegration/imaging/commit/68f6e7d85d4eded72af15aa66f93a74a7f4b8fcd) for CVE-2023-36308 and the immediately following Go module compatibility update. Later fork changes are not included. The regression test exercises photo cropping and media resizing with valid and invalid palette indices; the original dependency panics on the invalid input.
- Release CI audits the actual linux/amd64 image with Trivy, using its temporary repository token to read the private registry. It checks all severities, includes unfixed findings, and saves the report. The source and module audits remain release gates as well.

Return to the upstream imaging module when a release includes the palette fix and passes `TestImageProcessingPaletteBounds`. The current upstream v1.6.2 release has no patch; the pinned fork supplies the source fix. No advisory is excluded.

No Whatsmeow update, REST API change, or session database migration is included.
