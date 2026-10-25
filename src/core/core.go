// Package core holds Runny's testable business logic: configuration
// (loading, defaults, auto-upgrade, saving), script discovery, and Python
// interpreter resolution. The thin main package in ./src keeps the TUI and
// process plumbing on top of it.
package core

const configFileName = "runny_config.json"
