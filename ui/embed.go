// Package ui holds the interface (Svelte, sources in this folder), built into dist/ by "npm run build"
// (the Dockerfile does it) and served by the monitor at the root of the site, with its fonts and map images.
package ui

import "embed"

// Dist is the built interface; without a build it only holds dist/.gitkeep and the site says so.
//
//go:embed all:dist
var Dist embed.FS
