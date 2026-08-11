---
target: c3-203
scope: block
base: c3-203#n920@v1:sha256:fd320caba48764e69ad9cf87a0a550ad8e97bc8bdf580a91589495c139f4cae8
---
| ref-fat-thin-distribution | ref | The wrapper behavior required by full-fat skill ZIPs, Linux portable fat skill ZIPs, and no-binary skill/plugin artifacts | Full/portable fat artifacts exec bundled binaries; no-binary artifacts delegate to the pinned npm runtime manager | c3x.sh tries full bundled binary, Linux portable bundled binary, source build, then npm exec @cuongtran001/c3x-cli@VERSION. |
