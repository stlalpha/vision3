//go:build !release

package main

// Stubbed embed variables for development builds.
var (
	releaseDataTarGz []byte
	publicKeyData    []byte
	releaseSignature []byte
)
