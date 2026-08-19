---
target: ref-cross-compiled-binary
scope: block
base: ref-cross-compiled-binary#n1095@v1:sha256:cd620119a82c67d270cc21fff20f5baeea7295a469515687339e365995c6334a
---
Merging the release PR makes release-please create the `v{VERSION}` tag and the GitHub Release, which chains the release workflow's release build variant: thin plus full-fat binaries for each platform in the matrix, and a `CGO_ENABLED=0` portable binary for each Linux arch. Release assembly publishes the standard binaries for the npm manager and packages full-fat and Linux portable binaries into their matching skill ZIPs, uploading them to the release that already exists.
