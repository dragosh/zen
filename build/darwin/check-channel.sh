#!/bin/sh
# Checks the inputs of a release channel before anything is built or signed.
#   sh build/darwin/check-channel.sh sign   # inputs for signing
#   sh build/darwin/check-channel.sh pkg    # plus the installer identity
# Environment (nothing is stored in the repository):
#   CHANNEL             devid (default) | appstore
#   SIGN_IDENTITY       "Developer ID Application: ..." | "Apple Distribution: ..." | - (ad hoc)
#   TEAM_ID             Apple Team ID (appstore)
#   PROFILE             path of the Mac App Store provisioning profile (appstore)
#   INSTALLER_IDENTITY  "3rd Party Mac Developer Installer: ..." (appstore pkg)
#   ADHOC=1             local dry run of the appstore build: ad-hoc signature, no profile, no .pkg
step="${1:-sign}"
channel="${CHANNEL:-devid}"
fail() { echo "zen: CHANNEL=$channel: $*" >&2; exit 1; }

case "$channel" in
devid) exit 0 ;;
appstore) ;;
*) fail "CHANNEL must be devid or appstore" ;;
esac

if [ "$ADHOC" = 1 ]; then
	echo "zen: ADHOC=1: ad-hoc App Store build (sandboxed, no profile, no .pkg); it cannot be uploaded" >&2
	exit 0
fi
case "$SIGN_IDENTITY" in
"Apple Distribution"*) ;;
*) fail "SIGN_IDENTITY must be an \"Apple Distribution: ...\" identity (got \"${SIGN_IDENTITY:--}\"); ADHOC=1 makes a local dry run" ;;
esac
[ -n "$TEAM_ID" ] || fail "TEAM_ID is not set (your Apple Developer Team ID)"
[ -n "$PROFILE" ] || fail "PROFILE is not set: the path of the Mac App Store provisioning profile (.provisionprofile)"
[ -f "$PROFILE" ] || fail "PROFILE=$PROFILE does not exist"
if [ "$step" = pkg ]; then
	case "$INSTALLER_IDENTITY" in
	"3rd Party Mac Developer Installer"* | "Apple Distribution"* | "Mac Installer Distribution"*) ;;
	*) fail "INSTALLER_IDENTITY must be your \"3rd Party Mac Developer Installer: ...\" identity" ;;
	esac
fi
exit 0
