---
title: "Installation"
description: "Install imo-shortlist from a release, with go install, or from source."
weight: 20
---

## Prebuilt binaries

Every [release](https://github.com/tamnd/imo-shortlist-cli/releases) carries archives for Linux, macOS,
and Windows on amd64 and arm64, plus deb, rpm, and apk packages for Linux.
Download, unpack, put `imo-shortlist` on your `PATH`, done. The `checksums.txt`
on each release is signed with keyless [cosign](https://docs.sigstore.dev/) if
you want to verify before running.

## With Go

```bash
go install github.com/tamnd/imo-shortlist-cli/cmd/imo-shortlist@latest
```

That puts `imo-shortlist` in `$(go env GOPATH)/bin`, which is `~/go/bin` unless
you moved it. Make sure that directory is on your `PATH`.

## From source

```bash
git clone https://github.com/tamnd/imo-shortlist-cli
cd imo-shortlist-cli
make build        # produces ./bin/imo-shortlist
./bin/imo-shortlist version
```

## Container image

```bash
docker run --rm ghcr.io/tamnd/imo-shortlist:latest --help
```

## Checking the install

```bash
imo-shortlist version
```

prints the version and exits.
