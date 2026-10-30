#!/bin/sh
set -eu
project=$(cd "$(dirname "$0")/.." && pwd -P)
root=$(mktemp -d "$project/.migration-tests.XXXXXX")
case "$root" in "$project"/.migration-tests.*) ;; *) exit 1 ;; esac
trap 'rm -rf -- "$root"' 0
. "$project/module/hot-install.sh"
count=0
fixture() {
  count=$((count+1)); adb="$root/case-$count/adb"; data="$adb/dupeguard"; active="$adb/modules/filemanager"; old="$adb/modules/dupeguard"
  mkdir -p "$data/repairs" "$active/bin" "$active/webroot" "$adb/modules/other"
  printf 'id=filemanager\nversion=v1.9.0\n' > "$active/module.prop"
  printf 'private-record' > "$data/repairs/record"
  printf 'unrelated' > "$adb/modules/other/keep"
}
legacy() { mkdir -p "$old/bin"; printf 'id=dupeguard\nversion=v1.6.0\n' > "$old/module.prop"; printf legacy-program > "$old/bin/dupeguard"; }
preserved() { [ "$(cat "$data/repairs/record")" = private-record ]; [ "$(cat "$adb/modules/other/keep")" = unrelated ]; }
rejected() { if filemanager_retire_legacy "$active" "$data"; then echo 'Unexpected migration' >&2; exit 1; fi; [ -f "$old/bin/dupeguard" ]; preserved; }
fixture
filemanager_retire_legacy "$active" "$data"
preserved
echo 'PASS clean installation has no legacy migration'
fixture; legacy
filemanager_retire_legacy "$active" "$data"
[ ! -e "$old" ]; [ "$(cat "$data/legacy-module-dupeguard/bin/dupeguard")" = legacy-program ]; [ ! -e "$data/lock" ]
filemanager_retire_legacy "$active" "$data"
preserved
echo 'PASS legacy module is backed up after readiness; repeat installation preserves data'
fixture; legacy; mkdir "$data/lock"; printf '%s' "$$" > "$data/lock/pid"
rejected
echo 'PASS running legacy operation blocks migration without moving the module'
fixture; legacy; printf 'id=unrelated\n' > "$old/module.prop"
rejected
echo 'PASS identity mismatch leaves the old directory untouched'
fixture; legacy; mkdir "$data/legacy-module-dupeguard"; printf keep > "$data/legacy-module-dupeguard/keep"
rejected
[ "$(cat "$data/legacy-module-dupeguard/keep")" = keep ]
echo 'PASS existing backup is never overwritten'
fixture; legacy; mkdir -p "$adb/modules_update/dupeguard"
rejected
echo 'PASS pending legacy installation blocks migration'
fixture; legacy
printf '#!/bin/sh\nprintf '\''{"ok":true,"data":{"version":"wrong"}}\\n'\''\n' > "$active/bin/filemanager"
chmod 755 "$active/bin/filemanager"; printf page > "$active/webroot/index.html"
if filemanager_ready "$active" "$data" v1.9.0; then exit 1; fi
[ -f "$old/bin/dupeguard" ]; preserved
echo 'PASS new backend version failure never retires the old module'
fixture; legacy; mkdir "$data/lock"; printf 99999999 > "$data/lock/pid"
filemanager_retire_legacy "$active" "$data"
[ ! -e "$old" ]; [ ! -e "$data/lock" ]; preserved
echo 'PASS verified dead-process lock is recovered for native migration'
fixture; legacy
printf '#!/bin/sh\nprintf '\''{"ok":true,"data":{"version":"v1.9.0"}}\\n'\''\n' > "$active/bin/filemanager"
chmod 755 "$active/bin/filemanager"; printf page > "$active/webroot/index.html"
filemanager_ready "$active" "$data" v1.9.0
[ "$(head -n 1 "$data/install-status")" = ready ]; [ ! -e "$old" ]; [ -f "$data/legacy-module-dupeguard/bin/dupeguard" ]; preserved
echo 'PASS readiness callback backs up the old module before reporting ready'
fixture
if filemanager_check_existing "$adb/modules_update/filemanager" "$active"; then exit 1; fi
printf 'QiSun8023/FileManager' > "$active/filemanager-origin"
filemanager_check_existing "$adb/modules_update/filemanager" "$active"
preserved
echo 'PASS foreign module identity is rejected; project origin marker permits upgrades'
echo 'Legacy migration: 10 scenarios passed (simulation, no Android device).' 
