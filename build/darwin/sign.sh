#!/bin/sh
# Signs a Zenmark.app bundle for the channel (inner to outer: helpers, then the app).
#   CHANNEL=devid|appstore SIGN_IDENTITY=... [TEAM_ID=...] [PROFILE=...] [ADHOC=1] sh build/darwin/sign.sh bin/Zenmark.app
# devid:    entitlements.plist (no sandbox); with a real identity: hardened runtime + timestamp.
# appstore: entitlements.sandbox.plist for the app, helper.entitlements.sandbox.plist
#           (app-sandbox + inherit) for the helpers, the provisioning profile embedded.
set -e
bundle="$1"
[ -d "$bundle" ] || { echo "sign.sh: $bundle is not a bundle" >&2; exit 1; }
dir="$(cd "$(dirname "$0")" && pwd)"
channel="${CHANNEL:-devid}"
identity="${SIGN_IDENTITY:--}"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

runtime=""
if [ "$identity" != "-" ]; then runtime="--timestamp"; fi
case "$channel" in
devid)
	ent="$dir/entitlements.plist"
	helper_ent="$dir/helper.entitlements.plist"
	[ "$identity" = "-" ] || runtime="--options runtime --timestamp"
	;;
appstore)
	ent="$dir/entitlements.sandbox.plist"
	helper_ent="$dir/helper.entitlements.sandbox.plist"
	if [ -n "$PROFILE" ] && [ "$ADHOC" != 1 ]; then
		cp "$PROFILE" "$bundle/Contents/embedded.provisionprofile"
	fi
	if [ -n "$TEAM_ID" ]; then
		# The application and team identifiers the store checks against the profile.
		id="$(plutil -extract CFBundleIdentifier raw -o - "$bundle/Contents/Info.plist")"
		cp "$ent" "$tmp/app.plist"
		plutil -insert 'com\.apple\.application-identifier' -string "$TEAM_ID.$id" "$tmp/app.plist"
		plutil -insert 'com\.apple\.developer\.team-identifier' -string "$TEAM_ID" "$tmp/app.plist"
		ent="$tmp/app.plist"
	fi
	;;
*) echo "sign.sh: unknown CHANNEL $channel" >&2; exit 1 ;;
esac

xattr -cr "$bundle"
if [ -d "$bundle/Contents/Helpers" ]; then
	for h in "$bundle"/Contents/Helpers/*; do
		[ -f "$h" ] || continue
		codesign --force --sign "$identity" $runtime --entitlements "$helper_ent" "$h"
	done
fi
codesign --force --sign "$identity" $runtime --entitlements "$ent" "$bundle"
codesign --verify --deep --strict "$bundle"
