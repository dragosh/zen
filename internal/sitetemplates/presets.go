// Package sitetemplates holds zen's built-in site templates (core/init): a
// folder for each preset, each with a template.yml and the files it copies.
// core/sitetemplates installs them into the user's templates folder.
package sitetemplates

import "embed"

// FS has one folder per preset: techdocs, learning and planner. The "all:"
// prefix keeps dotfiles such as .zen.yml.
//
//go:embed all:techdocs all:learning all:planner
var FS embed.FS
