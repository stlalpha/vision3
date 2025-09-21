//go:build release

package main

import "embed"

// Embed the release data tar.gz file (created by build script)
//
//go:embed release-data.tar.gz
var releaseDataTarGz []byte

// Embed the GPG public key for signature verification
//
//go:embed vision3-signing-key.asc
var publicKeyData []byte

// Embed the signature for verification
//
//go:embed release-data.tar.gz.sha256.asc
var releaseSignature []byte

// Ensure the embed import is retained when building with release tag
var _ embed.FS
