//go:build production

package desktop

// DevBuild is false in shipped builds (`-tags production`): no Develop menu.
const DevBuild = false
