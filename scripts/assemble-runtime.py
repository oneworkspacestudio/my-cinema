"""Assemble unmodified official Jellyfin and compatible FFmpeg portable binaries.

For the exact published ZIP use the runtime asset on the My Cinema release.
This script records vendor assembly; deflate bytes can vary by zlib version.
"""
import argparse, hashlib, json, zipfile
from pathlib import Path

SERVER_SHA='64b81a7b01b4dfb4ad3380a367cdeaf59878d9070c0d5e0cf84025e58ed3e853'
FFMPEG_SHA='bc2a3b5f0868ff446fd8d472c124fe9430cf5ac972d02433c884170afb2e75e9'
def digest(path):
 h=hashlib.sha256()
 with Path(path).open('rb') as f:
  for block in iter(lambda:f.read(1048576),b''):h.update(block)
 return h.hexdigest()
def assemble(server,encoder,output):
 assert digest(server)==SERVER_SHA,'Jellyfin vendor checksum mismatch'
 assert digest(encoder)==FFMPEG_SHA,'FFmpeg vendor checksum mismatch'
 output=Path(output)
 if output.exists():raise ValueError('Use a new output file')
 with zipfile.ZipFile(server) as base,zipfile.ZipFile(encoder) as ff,zipfile.ZipFile(output,'w',zipfile.ZIP_DEFLATED,compresslevel=5) as result:
  replacements={}
  for name in ['ffmpeg.exe','ffprobe.exe']:
   members=[s for s in ff.namelist() if Path(s).name==name]
   assert len(members)==1
   replacements['jellyfin/'+name]=ff.read(members[0])
  manifest={'schema':1,'jellyfin':'12.3','serverArchiveSHA256':SERVER_SHA,'ffmpeg':'8.1.2-1','encoderArchiveSHA256':FFMPEG_SHA,'vendorBinariesModified':False,'replacements':{k:hashlib.sha256(v).hexdigest() for k,v in replacements.items()}}
  for old in base.infolist():
   assert old.filename.startswith('jellyfin/') and '..' not in old.filename.split('/')
   entry=zipfile.ZipInfo(old.filename,(2026,10,24,0,0,0));entry.compress_type=zipfile.ZIP_DEFLATED;entry.external_attr=old.external_attr
   result.writestr(entry,replacements.get(old.filename,base.read(old.filename)))
  entry=zipfile.ZipInfo('jellyfin/MYCINEMA-ASSEMBLY.json',(2026,10,24,0,0,0));entry.compress_type=zipfile.ZIP_DEFLATED
  result.writestr(entry,json.dumps(manifest,indent=2))
 print(json.dumps({'runtimeSHA256':digest(output),**manifest},indent=2))
if __name__=='__main__':
 parser=argparse.ArgumentParser();parser.add_argument('--server-zip',required=True);parser.add_argument('--ffmpeg-zip',required=True);parser.add_argument('--output',required=True);args=parser.parse_args();assemble(args.server_zip,args.ffmpeg_zip,args.output)
