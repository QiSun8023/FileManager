"""Test and build an ARM64 Android module ZIP from this repository."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import shutil
import struct
import subprocess
import sys
import zipfile
from set_version import ROOT, REPO, manifest, props


def run(command, cwd=ROOT, env=None):
    print('Running:', command[0], *command[1:3], flush=True)
    subprocess.run(command, cwd=cwd, env=env, check=True)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--go', default='go')
    parser.add_argument('--node', default='node')
    parser.add_argument('--bash', default='bash')
    args = parser.parse_args()
    run([sys.executable, str(ROOT/'tools/build_companion.py')])
    prop = props()
    version, code = prop['version'], int(prop['versionCode'])
    update = manifest(version, code)
    assert prop['id'] == 'filemanager'
    assert prop['updateJson'] == f'https://raw.githubusercontent.com/{REPO}/main/update-filemanager.json'
    assert f'const version = "{version}"' in (ROOT/'engine/version.go').read_text(encoding='utf8')
    assert f"'{version}'" in (ROOT/'module/webroot/app.js').read_text(encoding='utf8')
    assert not (ROOT/'module/action.sh').exists()
    env = os.environ.copy()
    # Run tests for this computer before switching the target to Android.
    env.pop('GOOS', None)
    env.pop('GOARCH', None)
    env['CGO_ENABLED'] = '0'
    run([sys.executable, str(ROOT/'tests/test-release.py')])
    run([args.go, 'test', './...'], ROOT/'engine', env)
    run([args.go, 'vet', './...'], ROOT/'engine', env)
    for test in ('test-bridge.mjs', 'test-theme.mjs', 'test-watch-ui.mjs', 'test-helper-ui.mjs'):
        run([args.node, str(ROOT/'tests'/test)])
    for test in ('test-live-install.sh', 'test-ksu-install.sh', 'test-legacy-install.sh', 'test-uninstall.sh', 'test-companion-install.sh'):
        run([args.bash, str(ROOT/'tests'/test)])
    for path in (ROOT/'module/webroot').glob('*.js'):
        run([args.node, '--check', str(path)])
    for path in (ROOT/'module').glob('*.sh'):
        run([args.bash, '-n', str(path)])
    env.update(GOOS='android', GOARCH='arm64', CGO_ENABLED='0')
    binary = ROOT/'module/bin/filemanager'
    binary.parent.mkdir(exist_ok=True)
    run([args.go, 'build', '-trimpath', '-ldflags', '-s -w -buildid=', '-o', str(binary), '.'], ROOT/'engine', env)
    elf = binary.read_bytes()
    assert elf[:4] == b'\x7fELF' and struct.unpack_from('<HH', elf, 16) == (3, 183)
    assert version.encode() in elf and b'/system/bin/linker64\0' in elf
    dist = ROOT/'dist'
    dist.mkdir(exist_ok=True)
    package = dist/f'FileManager-{version}-arm64.zip'
    with zipfile.ZipFile(package, 'w') as archive:
        for path in sorted((ROOT/'module').rglob('*')):
            if not path.is_file():
                continue
            name = path.relative_to(ROOT/'module').as_posix()
            data = path.read_bytes()
            if path.suffix in ('.sh', '.prop', '.html', '.js', '.css'):
                assert b'\r' not in data, f'LF required: {name}'
            entry = zipfile.ZipInfo(name, date_time=(2026, 10, 29, 12, 0, 0))
            entry.create_system = 3
            entry.external_attr = (0o100755 if name.endswith('.sh') or name == 'bin/filemanager' else 0o100644) << 16
            entry.compress_type = zipfile.ZIP_DEFLATED
            archive.writestr(entry, data)
    with zipfile.ZipFile(package) as archive:
        assert archive.testzip() is None
        assert 'module.prop' in archive.namelist() and 'action.sh' not in archive.namelist()
    (dist/'update-filemanager.json').write_text(json.dumps(update, ensure_ascii=False, indent=2)+'\n', encoding='utf8')
    shutil.copy2(ROOT/'CHANGELOG.md', dist/'CHANGELOG.md')
    digest = hashlib.sha256(package.read_bytes()).hexdigest()
    (dist/'SHA256SUMS.txt').write_text(f'{digest}  {package.name}\n', encoding='utf8')
    print(package)
    print('SHA256:', digest)
    print('Built locally; no files have been published to GitHub.')


if __name__ == '__main__':
    main()
