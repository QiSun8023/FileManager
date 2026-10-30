#!/usr/bin/env bash
set -eu
project=$(cd -- "$(dirname -- "$0")/.." && pwd)
test_root=$(mktemp -d "$project/.helper-tests.XXXXXX")
trap 'rm -rf -- "$test_root"' EXIT
adb="$test_root/adb"; module="$adb/modules/filemanager"; data="$adb/dupeguard"; app="$test_root/app"
mkdir -p "$module" "$data" "$app"
sed "s#/data/adb#$adb#g" "$project/module/companion-install.sh" > "$test_root/helper.sh"
. "$test_root/helper.sh"
printf QiSun8023/FileManager > "$module/filemanager-origin"
printf 'fake signed apk for isolated shell test' > "$module/companion.apk"
sha256sum "$module/companion.apk" | cut -d ' ' -f 1 > "$module/companion.sha256"
calls=0
pm() {
 case "$1" in
 path) [ -f "$app/base.apk" ] && printf 'package:%s\n' "$app/base.apk" || true ;;
 install) printf 1 >> "$test_root/install-calls"; [ ! -f "$test_root/reject" ] || return 1; cp -- "${@: -1}" "$app/base.apk"; printf Success ;;
 *) return 1 ;;
 esac
}
filemanager_install_companion "$module"
[ "$(cat "$data/helper-installed-sha256")" = "$(cat "$module/companion.sha256")" ]
[ "$(cat "$test_root/install-calls")" = 1 ]
printf 'PASS verified helper installs and records exact installed APK hash\n'
filemanager_install_companion "$module"
[ "$(cat "$test_root/install-calls")" = 1 ]
printf 'PASS identical helper does not reinstall or reset application state\n'
printf 'new signed apk' > "$module/companion.apk"
sha256sum "$module/companion.apk" | cut -d ' ' -f 1 > "$module/companion.sha256"
touch "$test_root/reject"
if filemanager_install_companion "$module"; then printf 'FAIL signature rejection accepted\n'; exit 1; fi
[ "$(cat "$app/base.apk")" = 'fake signed apk for isolated shell test' ]
printf 'PASS package manager rejection preserves an existing same-name app\n'
rm "$test_root/reject"
printf tampered >> "$module/companion.apk"
if filemanager_install_companion "$module"; then printf 'FAIL tampered APK accepted\n'; exit 1; fi
[ "$(cat "$test_root/install-calls")" = 11 ]
printf 'PASS APK checksum failure never invokes package installation\n'
printf foreign > "$module/filemanager-origin"
if filemanager_install_companion "$module"; then printf 'FAIL foreign module accepted\n'; exit 1; fi
printf 'PASS foreign module origin never installs helper\n'
printf 'Companion installer: 5 isolated scenarios passed; no actual APK installation.\n'
