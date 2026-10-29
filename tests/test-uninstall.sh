#!/bin/sh
set -eu
project=$(cd "$(dirname "$0")/.." && pwd -P)
root=$(mktemp -d "$project/.uninstall-tests.XXXXXX")
case "$root" in "$project"/.uninstall-tests.*) ;; *) exit 1 ;; esac
trap 'rm -rf -- "$root"' 0
# Load functions only. Never execute the production /data/adb entry point here.
sed '$d' "$project/module/uninstall.sh" > "$root/functions.sh"
. "$root/functions.sh"
n=0
fixture() {
 n=$((n+1)); adb="$root/case-$n/adb"; storage="$root/case-$n/storage"; active="$adb/modules/filemanager"; data="$adb/dupeguard"
 mkdir -p "$active" "$data/repairs" "$data/legacy-module-dupeguard" "$storage/Android/FileManager/Trash/files/id" "$storage/Android/FileManager/Trash/records" "$storage/.dupeguard-trash" "$storage/Download" "$adb/modules/other"
 printf 'id=filemanager\n' > "$active/module.prop"; printf QiSun8023/FileManager > "$active/filemanager-origin"
 printf record > "$data/repairs/record"; printf backup > "$data/legacy-module-dupeguard/keep"
 printf audio > "$storage/Android/FileManager/Trash/files/id/content"; printf record > "$storage/Android/FileManager/Trash/records/id.json"
 printf legacy > "$storage/.dupeguard-trash/keep"; printf original > "$storage/Download/keep"; printf other > "$adb/modules/other/keep"
 printf sibling > "$storage/Android/FileManager/keep"
}
unrelated() { [ "$(cat "$storage/Download/keep")" = original ]; [ "$(cat "$adb/modules/other/keep")" = other ]; [ "$(cat "$storage/Android/FileManager/keep")" = sibling ]; }
reject() { if filemanager_uninstall "$active" "$adb" "$storage"; then exit 1; fi; [ -f "$data/repairs/record" ]; [ -f "$storage/Android/FileManager/Trash/records/id.json" ]; unrelated; }
fixture; filemanager_uninstall "$active" "$adb" "$storage"
[ ! -e "$data" ]; [ ! -e "$storage/Android/FileManager/Trash" ]; [ ! -e "$storage/.dupeguard-trash" ]; unrelated
filemanager_uninstall "$active" "$adb" "$storage"
echo 'PASS all owned records, backups and recycled files are removed; originals and sibling files survive; repeated cleanup works'
fixture; printf foreign > "$active/filemanager-origin"; reject
echo 'PASS foreign origin blocks cleanup'
fixture; printf 'id=other\n' > "$active/module.prop"; reject
echo 'PASS wrong module identity blocks cleanup'
fixture; mkdir -p "$adb/modules/dupeguard"; reject
echo 'PASS an active legacy module prevents deleting its shared records'
fixture; mkdir "$data/lock"; printf '%s' "$$" > "$data/lock/pid"; reject
echo 'PASS an unrelated live process lock is never killed or erased'
fixture; mkdir "$data/lock"; printf unknown > "$data/lock/pid"; reject
echo 'PASS ambiguous lock preserves all data'
fixture; if filemanager_uninstall "$active" "$adb" "$storage/unavailable"; then exit 1; fi
[ -f "$data/repairs/record" ]; unrelated
echo 'PASS unavailable shared storage preserves recovery records'
# Capture detached launch rather than starting any production background worker.
filemanager_busybox() { printf /bin/sh; }
filemanager_detach_cleanup() { printf '%s' "$2" > "$data/launched"; }
fixture; cp "$project/module/uninstall.sh" "$active/uninstall.sh"; printf first > "$data/install-generation"
filemanager_uninstall "$active" "$adb" "$storage/unavailable"
[ -f "$data/uninstall-cleanup/uninstall.sh" ]; [ "$(cat "$data/uninstall-cleanup/generation")" = first ]; [ ! -e "$data/lock" ]; unrelated
echo 'PASS unavailable storage queues an owned helper and retains records'
sed "s#/data/adb#$adb#g;s#/storage/emulated/0#$storage#g" "$project/module/uninstall.sh" > "$data/uninstall-cleanup/uninstall.sh"
printf changed > "$data/install-generation"
sh "$data/uninstall-cleanup/uninstall.sh" --deferred
[ -f "$data/repairs/record" ]; unrelated
echo 'PASS a newer installation generation cancels deferred cleanup'
printf first > "$data/install-generation"; mv "$active" "$data/retired-test-module"
mkdir -p "$root/fakebin"
printf '#!/bin/sh\ncase "$1" in sys.user.0.ce_available) printf true ;; *) printf 1 ;; esac\n' > "$root/fakebin/getprop"
chmod 755 "$root/fakebin/getprop"
PATH="$root/fakebin:$PATH" sh "$data/uninstall-cleanup/uninstall.sh" --deferred
[ ! -e "$data" ]; [ ! -e "$storage/Android/FileManager/Trash" ]; unrelated
echo 'PASS deferred worker cleans up its own helper and records after storage unlock'
echo 'Uninstall: 10 scenarios passed (isolated filesystem simulation).'
