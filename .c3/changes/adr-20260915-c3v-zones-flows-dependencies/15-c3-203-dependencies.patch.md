---
target: c3-203
scope: insert
base: c3-203@v1:sha256:b1f0b9c3f91a8639fa1f46cf98fd3dfd50b6322c4a14071823c6c655504f7be5
---
## Dependencies

| Depends on | Interaction | Contract | Evidence |
| --- | --- | --- | --- |
| c3-1 | Execs the selected platform binary with the caller's arguments | exec c3x-<version>-<os>-<arch> <args> | skills/c3/bin/c3x.sh binary resolution + exec |
| c3-3 | Falls back to the npm thin client when no local binary is available | npm exec @cuongtran001/c3x-cli@<VERSION> | skills/c3/bin/c3x.sh npm fallback |

