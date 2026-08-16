---
id: c3-3
c3-seal: 1b95628ef0607c445c154ff1c8f96323061391729dc31f888e85358dd7d36927
title: npm @cuongtran001/c3x-cli
type: container
parent: c3-0
goal: Install, manage, and run the c3x binary from npm — a thin client that resolves verified GitHub Release runtimes and forwards normal commands.
---

# npm @cuongtran001/c3x-cli

## Goal

Install, manage, and run the c3x binary from npm — a thin client that resolves verified GitHub Release runtimes and forwards normal commands.

## Components

| ID | Name | Category | Status | Goal Contribution |
| --- | --- | --- | --- | --- |
| c3-301 | binary-downloader |  | active | Resolve, install, verify, select, run, uninstall, and prune c3x release runtimes while forwarding normal commands to the selected binary. |

## Responsibilities

Publish the `@cuongtran001/c3x-cli` package; serve root help and version locally without runtime resolution; list and install verified `cuongtranba/c3-skill` GitHub Release runtimes; report progress when real commands must fetch runtime assets; store safe per-project runtime selection metadata; cache platform assets by version; explicitly uninstall or prune cache entries; and forward normal commands to the selected Go binary. Holds no architecture behavior — distribution only.

## Complexity Assessment

Medium-low: a TypeScript runtime manager that owns release lookup, version selection, checksum-verified install, project metadata validation, explicit cache cleanup, and passthrough exec. The main risk surfaces are platform/version resolution and preventing project metadata from becoming executable authority.
