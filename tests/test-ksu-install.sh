#!/bin/sh
set -eu
project=$(cd "$(dirname "$0")/.." && pwd -P)
test_root=$(mktemp -d "$project/.ksu-tests.XXXXXX")
case "$test_root" in "$project"/.ksu-tests.*) ;; *) exit 1 ;; esac
trap 'rm -rf -- "$test_root"' 0
. "$project/module/live-install.sh"
. "$project/module/hot-install.sh"
ui_print() { printf '%s\n' "$*"; }
set_perm() { chmod "$4" "$1"; }
set_perm_recursive() { find "$1" -type d -exec chmod "$4" {} +; find "$1" -type f -exec chmod "$5" {} +; }
fail() { echo "FAIL: $*" >&2; exit 1; }
count=0
fixture() {
  count=$((count+1)); adb="$test_root/case-$count/adb"; data="$adb/dupeguard"; active="$adb/modules/filemanager"; stage="$adb/modules_update/filemanager"
  mkdir -p "$data/trash" "$data/repairs" "$stage/bin" "$stage/webroot" "$active/bin" "$active/webroot" "$adb/modules_update/other"
  printf 'restore-data' > "$data/trash/keep"; printf 'repair-data' > "$data/repairs/keep"
  printf 'other-module' > "$adb/modules_update/other/keep"
  printf 'QiSun8023/FileManager' > "$stage/filemanager-origin"
  printf 'QiSun8023/FileManager' > "$active/filemanager-origin"
  printf 'id=filemanager\nversion=v1.7.0\n' > "$stage/module.prop"
  printf '#!/bin/sh\nprintf '\''{"ok":true,"data":{"version":"v1.7.0"}}\\n'\''\n' > "$stage/bin/filemanager"; chmod 755 "$stage/bin/filemanager"
  printf new-page > "$stage/webroot/index.html"; printf new-code > "$stage/webroot/app.js"
  for n in action.sh skip_mount LICENSE GO-LICENSE; do printf new > "$stage/$n"; done
  for n in hot-install.sh live-install.sh update.sh post-install.sh; do cp "$project/module/$n" "$stage/$n"; done
  printf token-one > "$stage/.filemanager-install-token"
  printf old-code > "$active/bin/filemanager"; printf old-page > "$active/webroot/index.html"
  # Emulate metadata that KernelSU writes after customize.sh returns.
  cp "$stage/module.prop" "$active/module.prop"; touch "$active/update"
}
kept() {
  [ "$(cat "$data/trash/keep")" = restore-data ] || fail 'restore records changed'
  [ "$(cat "$data/repairs/keep")" = repair-data ] || fail 'repair records changed'
  [ "$(cat "$adb/modules_update/other/keep")" = other-module ] || fail 'another pending module changed'
}
ready() {
  [ "$(head -n 1 "$data/install-status")" = ready ] || fail 'not ready'
  [ "$(cat "$active/webroot/index.html")" = new-page ] || fail 'page not published'
  [ ! -e "$active/update" ] || fail 'own update flag remains'
  [ ! -e "$active/action.sh" ] || fail 'action entry remains'
  [ ! -e "$stage" ] || fail 'own staged module remains'
  kept
}

KSU=true; KSU_HAS_HOT_INSTALL=true
[ "$(filemanager_mode)" = native ] || fail 'native detection'
unset KSU_HAS_HOT_INSTALL
[ "$(filemanager_mode)" = deferred ] || fail 'older KernelSU was skipped'
KSU=''; [ "$(filemanager_mode)" = deferred ] || fail 'Magisk fallback'
printf 'PASS native KernelSU, older KernelSU and Magisk mode selection\n'

fixture
filemanager_deferred_update "$stage" "$active" "$data" v1.7.0 token-one 0 0 || fail 'deferred update'
ready
printf 'PASS older KernelSU publishes WebUI and clears only its own pending files\n'

fixture
sleep 0.8 & installer=$!
sleep 1.4 & parent=$!
filemanager_deferred_update "$stage" "$active" "$data" v1.7.0 token-one "$installer" "$parent" > "$test_root/wait.log" 2>&1 & updater=$!
sleep 0.3
[ "$(cat "$active/webroot/index.html")" = old-page ] || fail 'published before installer finished'
wait "$installer"; sleep 0.2
[ "$(cat "$active/webroot/index.html")" = old-page ] || fail 'published before ksud parent finished'
wait "$parent"; wait "$updater" || fail 'updater after parent exit'
ready
printf 'PASS waits for both installer shell and finishing ksud process\n'

fixture
printf 'version=v1.2.0\n' > "$active/module.prop"
if filemanager_deferred_update "$stage" "$active" "$data" v1.7.0 token-one 0 0; then fail 'unfinished installer accepted'; fi
[ "$(cat "$active/webroot/index.html")" = old-page ] || fail 'unfinished install replaced old page'
[ -d "$stage" ] || fail 'unfinished install removed staging'
kept
printf 'PASS failed/incomplete installer is not falsely activated\n'

fixture
if filemanager_deferred_update "$stage" "$active" "$data" v1.7.0 wrong-token 0 0; then fail 'stale token accepted'; fi
[ "$(cat "$active/webroot/index.html")" = old-page ] || fail 'stale update changed active module'
kept
printf 'PASS stale worker cannot publish a different installation\n'

fixture
mkdir "$data/lock"; printf '%s' "$$" > "$data/lock/pid"
if filemanager_deferred_update "$stage" "$active" "$data" v1.7.0 token-one 0 0; then fail 'busy update accepted'; fi
[ "$(cat "$active/webroot/index.html")" = old-page ] || fail 'busy update changed active module'
kept
printf 'PASS busy file operation remains protected\n'

fixture
cp -a "$stage/." "$active/"
filemanager_ready "$active" "$data" v1.7.0 || fail 'native callback'
[ "$(head -n 1 "$data/install-status")" = ready ] || fail 'native status'
[ ! -e "$active/update" ] || fail 'native own flag not cleared'
[ ! -e "$active/action.sh" ] || fail 'native callback kept old action entry'
kept
printf 'PASS native hot-install callback validates backend and WebUI\n'

fixture
cp -a "$stage/." "$active/"
filemanager_deferred_update "$active" "$active" "$data" v1.7.0 token-one 0 0 || fail 'direct install'
[ -f "$active/webroot/index.html" ] && [ ! -e "$active/update" ] || fail 'direct install damaged live files'
kept
printf 'PASS manager installing directly into live directory is supported\n'

# Run the real customize.sh with only its fixed adb paths mapped to a fixture.
for native in true false; do
  fixture
  mkdir -p "$adb/ksu/bin"
  printf '#!/bin/sh\nprintf '\''%%s\\n'\'' "$*" > '\''%s/launch-args'\''\n' "$adb" > "$adb/ksu/bin/busybox"
  chmod 755 "$adb/ksu/bin/busybox"
  sed "s#/data/adb#$adb#g" "$project/module/customize.sh" > "$test_root/customize-test.sh"
  (
    ARCH=arm64; API=33; BOOTMODE=true; MODPATH=$stage; KSU=true; KSU_HAS_HOT_INSTALL=$native
    abort() { echo "ABORT: $*"; exit 1; }
    cat() { if [ "$1" = /proc/sys/kernel/random/uuid ]; then printf token-custom; else command cat "$@"; fi; }
    . "$test_root/customize-test.sh"
    if [ "$native" = true ]; then
      [ "$MODULE_HOT_INSTALL_REQUEST" = true ] && [ "$MODULE_HOT_RUN_SCRIPT" = post-install.sh ] || exit 1
    else
      wait
      grep -Fq setsid "$adb/launch-args" || exit 1
      grep -Fq update.sh "$adb/launch-args" || exit 1
    fi
  ) || fail "real customize.sh KernelSU branch native=$native"
done
printf 'PASS real customize.sh requests native hot install and queues older-KSU updater\n'
printf 'KernelSU installer: 10 scenarios passed (simulation, no Android device).\n'
