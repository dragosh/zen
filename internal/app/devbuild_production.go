//go:build production

package app

// devBuild is false in shipped builds (`-tags production`): no Develop menu.
const devBuild = false
