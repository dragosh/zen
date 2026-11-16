#!/bin/sh
# .zen/plugins/note/note.sh — renders :::note and :::warning containers
esc() { sed -e 's/&/\&amp;/g' -e 's/</\&lt;/g' -e 's/>/\&gt;/g'; }
case "$ZEN_NAME" in
  warning) kind=warning title=Warning ;;
  *) kind=note title=Note ;;
esac
title="${ZEN_ATTR_TITLE:-$title}"
printf '<div class="%s"><strong>' "$kind"
printf '%s' "$title" | esc
printf '</strong>\n'
esc                                  # the body arrives on stdin
printf '</div>'
