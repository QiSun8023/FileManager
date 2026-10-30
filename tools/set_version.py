"""Synchronize version constants and the public update manifest, without publishing."""
import argparse
import json
from pathlib import Path
import re

ROOT = Path(__file__).resolve().parents[1]
REPO = 'QiSun8023/FileManager'


def props():
    return dict(line.split('=', 1) for line in (ROOT/'module/module.prop').read_text(encoding='utf8').splitlines() if '=' in line)


def manifest(version, code):
    return {
        'version': version,
        'versionCode': code,
        'zipUrl': f'https://github.com/{REPO}/releases/download/{version}/FileManager-{version}-arm64.zip',
        'changelog': f'https://raw.githubusercontent.com/{REPO}/{version}/CHANGELOG.md',
    }


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('version', help='Example: v1.5.2')
    parser.add_argument('version_code', type=int, help='Must increase, example: 10502')
    args = parser.parse_args()
    if not re.fullmatch(r'v\d+\.\d+\.\d+', args.version):
        parser.error('Use vMAJOR.MINOR.PATCH')
    previous = props()
    if args.version_code <= int(previous['versionCode']):
        parser.error('versionCode must be greater than the current value')
    if args.version == previous['version']:
        parser.error('Choose a new version name')
    old = previous['version']
    for folder in ('engine', 'module', 'tests'):
        for path in (ROOT/folder).rglob('*'):
            if not path.is_file() or 'bin' in path.relative_to(ROOT/folder).parts:
                continue
            if path.suffix not in ('.go', '.js', '.mjs', '.html', '.css', '.sh', '.md', '.prop'):
                continue
            text = path.read_text(encoding='utf8')
            text = text.replace(old, args.version).replace('v='+old[1:], 'v='+args.version[1:])
            if path.name == 'module.prop':
                text = re.sub(r'^versionCode=\d+$', f'versionCode={args.version_code}', text, flags=re.M)
            if path.name == 'index.html':
                text = text.replace(f'文件管理 {old[1:]}', f'文件管理 {args.version[1:]}')
            path.write_text(text, encoding='utf8', newline='\n')
    companion = ROOT/'companion/AndroidManifest.xml'
    if companion.exists():
        text = companion.read_text(encoding='utf8')
        text = re.sub(r'android:versionCode="[0-9]+"', f'android:versionCode="{args.version_code}"', text)
        text = re.sub(r'android:versionName="[^"]+"', f'android:versionName="{args.version[1:]}"', text)
        companion.write_text(text, encoding='utf8', newline='\n')
    (ROOT/'update-filemanager.json').write_text(json.dumps(manifest(args.version, args.version_code), ensure_ascii=False, indent=2)+'\n', encoding='utf8', newline='\n')
    print(f'Prepared {args.version} ({args.version_code}). Edit CHANGELOG.md before committing and tagging.')


if __name__ == '__main__':
    main()
