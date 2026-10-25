"""Validate a portable release and its Workspace contract before publication."""
import hashlib
import json
from pathlib import Path
import sys
import zipfile

folder = Path(sys.argv[1])
info = json.loads((folder / 'build-info.json').read_text(encoding='utf-8'))
manifest = json.loads((folder / 'workspace-update.json').read_text(encoding='utf-8'))
assert manifest['schema'] == 2 and manifest['integration'] == 3, 'Manager requirements need schema2/integration3'
assert manifest['minimumManager'] == '0.3.0-preview.86'
assert manifest['repository'] == 'oneworkspacestudio/my-cinema'
assert manifest['tag'] == 'v' + info['version']
assert len(manifest['apps']) == 1
app = manifest['apps'][0]
assert app['id'] == 'cinema' and app['version'] == info['version']
assert app['env'] == {'MYCINEMA_DRIVE_ROOT': '${FILES}'}
assert app['health'] == '/cinema-health' and app['route'] == '/cinema'
assert app['source'] == 'https://github.com/' + manifest['repository'] + '/releases/tag/' + manifest['tag']
for key, package in manifest['packages'].items():
    assert key == 'cinema' or key.startswith('cinema-')
    asset = folder / package['asset']
    assert asset.parent == folder and asset.is_file()
    with zipfile.ZipFile(asset) as z:
        with z.open(package['member']) as f:
            digest = hashlib.sha256()
            size = 0
            for block in iter(lambda: f.read(1024 * 1024), b''):
                digest.update(block)
                size += len(block)
    assert size == package['size'] and digest.hexdigest() == package['sha256'], key
assert manifest['packages']['cinema']['sha256'] == info['exeSHA256']
print(json.dumps({'version': info['version'], 'workspaceContractSchema': 2, 'integration': 3,
                  'allPackageMembersAndHashesVerified': True, 'minimumManager': manifest['minimumManager']}))
