#!/bin/sh
set -eu
project=$(cd "$(dirname "$0")/.." && pwd -P)
fixture_root=$(mktemp -d "$project/.live-install-tests.XXXXXX")
case "$fixture_root" in "$project"/.live-install-tests.*) ;; *) exit 1 ;; esac
trap 'rm -rf -- "$fixture_root"' 0
script="$project/module/live-install.sh"
. "$script"
ui_print() { printf '%s\n' "$*"; }
set_perm() { chmod "$4" "$1"; }
set_perm_recursive() {
  find "$1" -type d -exec chmod "$4" {} +
  find "$1" -type f -exec chmod "$5" {} +
}
fail() { printf 'FAIL: %s\n' "$*" >&2; exit 1; }
count=0
fixture() {
  count=$((count+1))
  adb="$fixture_root/case-$count/adb"
  stage="$adb/modules_update/filemanager"
  active="$adb/modules/filemanager"
  data="$adb/dupeguard"
  mkdir -p "$stage/bin" "$stage/webroot" "$adb/modules/other" "$data/trash" "$data/repairs"
  printf 'preserve unrelated module' > "$adb/modules/other/user.txt"
  printf 'restore records' > "$data/trash/record.json"
  printf 'repair records' > "$data/repairs/record.json"
  printf 'id=filemanager\nversion=v1.7.0\n' > "$stage/module.prop"
  printf '#!/bin/sh\nprintf '\''{"ok":true,"data":{"version":"v1.7.0"}}\\n'\''\n' > "$stage/bin/filemanager"
  chmod 755 "$stage/bin/filemanager"
  printf 'new-page' > "$stage/webroot/index.html"
  printf 'new-js' > "$stage/webroot/app.js"
  for item in action.sh skip_mount LICENSE GO-LICENSE; do printf 'new' > "$stage/$item"; done
}
old_version() {
  mkdir -p "$active/bin" "$active/webroot"
  printf 'old-backend' > "$active/bin/filemanager"
  printf 'old-page' > "$active/webroot/index.html"
  printf 'user-extra' > "$active/custom.txt"
  printf 'old action' > "$active/action.sh"
}
kept_records() {
  [ "$(cat "$data/trash/record.json")" = 'restore records' ] || fail 'trash records changed'
  [ "$(cat "$data/repairs/record.json")" = 'repair records' ] || fail 'repair records changed'
  [ "$(cat "$adb/modules/other/user.txt")" = 'preserve unrelated module' ] || fail 'other module changed'
}
published() {
  [ "$(cat "$active/webroot/index.html")" = new-page ] || fail 'page not activated'
  "$active/bin/filemanager" info e30= | grep -Fq '"version":"v1.7.0"' || fail 'backend not activated'
  [ ! -d "$data/lock" ] || fail 'installer lock leaked'
  [ ! -e "$adb/modules/.filemanager-live-$$" ] || fail 'prepared directory leaked'
  [ ! -e "$data/.filemanager-backup-$$" ] || fail 'backup directory leaked'
  cmp "$active/bin/filemanager" "$stage/bin/filemanager" || fail 'staged backend changed'
  cmp "$active/webroot/app.js" "$stage/webroot/app.js" || fail 'staged page changed'
  [ ! -e "$active/action.sh" ] || fail 'old action.sh survived upgrade'
  kept_records
}

fixture
filemanager_install_live "$stage" "$active" "$data" || fail 'first install'
published
printf 'PASS first install is immediately usable\n'

fixture; old_version
filemanager_install_live "$stage" "$active" "$data" || fail 'upgrade'
published
[ "$(cat "$active/custom.txt")" = user-extra ] || fail 'unrecognised module-root file lost'
printf 'PASS upgrade publishes both backend and WebUI, preserves data\n'

fixture; old_version
mkdir "$data/lock"; printf '%s' "$$" > "$data/lock/pid"
if filemanager_install_live "$stage" "$active" "$data"; then fail 'accepted busy upgrade'; fi
[ "$(cat "$active/bin/filemanager")" = old-backend ] || fail 'busy install changed backend'
[ "$(cat "$data/lock/pid")" = "$$" ] || fail 'busy lock was removed'
kept_records
printf 'PASS active operation blocks installation without changing old module\n'

fixture; old_version
mkdir "$data/lock"; printf '99999999' > "$data/lock/pid"
filemanager_install_live "$stage" "$active" "$data" || fail 'stale lock recovery'
published
printf 'PASS dead-process lock recovery\n'

fixture; old_version
printf '#!/bin/sh\nprintf '\''{"ok":true,"data":{"version":"v0.0.0"}}\\n'\''\n' > "$stage/bin/filemanager"
if filemanager_install_live "$stage" "$active" "$data"; then fail 'accepted wrong backend version'; fi
[ "$(cat "$active/webroot/index.html")" = old-page ] || fail 'health failure replaced old page'
[ ! -d "$data/lock" ] || fail 'failed preflight leaked lock'
kept_records
printf 'PASS backend version failure preserves old installation\n'

fixture; old_version
cp() { case "$2" in */modules_update/filemanager/webroot) return 1 ;; esac; command cp "$@"; }
if filemanager_install_live "$stage" "$active" "$data"; then fail 'accepted failed copy'; fi
unset -f cp
[ "$(cat "$active/webroot/index.html")" = old-page ] || fail 'copy failure replaced old module'
[ ! -d "$data/lock" ] || fail 'failed copy leaked lock'
kept_records
printf 'PASS preparation failure preserves old installation\n'

fixture; old_version
mv() { case "$2" in */modules/.filemanager-live-*) return 1 ;; esac; command mv "$@"; }
if filemanager_install_live "$stage" "$active" "$data"; then fail 'accepted failed publication'; fi
unset -f mv
[ "$(cat "$active/webroot/index.html")" = old-page ] || fail 'failed publication did not restore old version'
[ ! -d "$data/lock" ] || fail 'failed publication leaked lock'
kept_records
printf 'PASS publication failure rolls back old installation\n'

fixture; old_version
if filemanager_install_live "$stage" "$adb/modules/other" "$data"; then fail 'accepted wrong target'; fi
[ "$(cat "$active/webroot/index.html")" = old-page ] || fail 'invalid target changed original module'
kept_records
printf 'PASS path guard refuses another module\n'

fixture; old_version
mkdir "$adb/modules/.filemanager-live-$$"; printf 'do not delete' > "$adb/modules/.filemanager-live-$$/marker"
if filemanager_install_live "$stage" "$active" "$data"; then fail 'accepted temporary path collision'; fi
[ "$(cat "$adb/modules/.filemanager-live-$$/marker")" = 'do not delete' ] || fail 'deleted unowned temporary directory'
kept_records
printf 'PASS temporary collision never deletes an unowned directory\n'

fixture
mkdir "$data/.filemanager-backup-$$"; printf 'do not restore' > "$data/.filemanager-backup-$$/marker"
if filemanager_install_live "$stage" "$active" "$data"; then fail 'accepted backup collision'; fi
[ ! -e "$active" ] || fail 'restored unowned backup'
[ "$(cat "$data/.filemanager-backup-$$/marker")" = 'do not restore' ] || fail 'deleted unowned backup'
kept_records
printf 'PASS backup collision never restores or deletes an unowned backup\n'
printf 'Live installer: 10 scenarios passed (Git Bash simulation; no Android/SELinux validation).\n'
