"""Exercise release metadata synchronization in an isolated source copy."""
import json
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile

root = Path(__file__).resolve().parents[1]
with tempfile.TemporaryDirectory(prefix='filemanager-release-') as directory:
    target = Path(directory)/'FileManager'
    shutil.copytree(root, target, ignore=shutil.ignore_patterns('bin', 'dist', '__pycache__', '.git', 'node_modules', 'build'))
    metadata = dict(line.split('=',1) for line in (target/'module/module.prop').read_text(encoding='utf8').splitlines() if '=' in line)
    current = {'version': metadata['version'], 'versionCode': int(metadata['versionCode'])}
    script = target/'tools/set_version.py'
    before = (target/'module/module.prop').read_bytes()
    result = subprocess.run([sys.executable, str(script), current['version'], str(current['versionCode'])], capture_output=True)
    assert result.returncode != 0 and (target/'module/module.prop').read_bytes() == before
    print('PASS versionCode reuse is rejected without changing files')
    new = 'v99.88.77'
    code = current['versionCode']+1
    subprocess.run([sys.executable, str(script), new, str(code)], check=True)
    update = json.loads((target/'update-filemanager.json').read_text(encoding='utf8'))
    assert update['version'] == new and update['versionCode'] == code
    assert update['zipUrl'].endswith(f'/{new}/FileManager-{new}-arm64.zip')
    assert update['changelog'].endswith(f'/{new}/CHANGELOG.md')
    assert f'versionCode={code}' in (target/'module/module.prop').read_text(encoding='utf8')
    for name in ('engine/version.go', 'module/webroot/app.js', 'module/customize.sh', 'module/update.sh', 'tests/test-bridge.mjs', 'tests/test-ksu-install.sh'):
        assert new in (target/name).read_text(encoding='utf8'), name
    assert 'v=99.88.77' in (target/'module/webroot/index.html').read_text(encoding='utf8')
    assert '文件管理 99.88.77</footer>' in (target/'module/webroot/index.html').read_text(encoding='utf8')
    assert 'id=filemanager\n' in (target/'module/module.prop').read_text(encoding='utf8')
    print('PASS new version synchronizes backend, frontend, installer, tests and pinned update URLs')
print('Release tooling: 2 scenarios passed.')
