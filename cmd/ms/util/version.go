package util

// Version is the ms release version: semver, without a leading "v". It is the
// single source of truth for releases — bump it with script/release, and the
// .githooks/reference-transaction hook refuses any vX.Y.Z tag that does not
// match it. Overridable at link time with
// -ldflags "-X github.com/rubiojr/meliafts/cmd/ms/util.Version=...".
var Version = "0.8.2"
