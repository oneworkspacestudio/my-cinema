param([string]$Go='go',[string]$Version='0.1.0-preview.3',[string]$Out='dist',[Parameter(Mandatory)][string]$RuntimeZip)
$ErrorActionPreference='Stop'
$root=Split-Path $PSScriptRoot -Parent
$expected='5de2eb4e17fe77132c3f681fcc80cd63846f9ce78d8e7b75d7f9ed7e292b16b3'
if((Get-FileHash -LiteralPath $RuntimeZip -Algorithm SHA256).Hash.ToLowerInvariant() -cne $expected){throw 'Pinned Jellyfin runtime checksum mismatch'}
Push-Location (Join-Path $root 'web')
try{npm ci --no-audit --no-fund;if($LASTEXITCODE){throw 'Dependency install failed'};npm run build;if($LASTEXITCODE){throw 'Frontend build failed'}}finally{Pop-Location}
New-Item -ItemType Directory -Force (Join-Path $root $Out) | Out-Null
Push-Location $root
try{
 $target=Join-Path $root "$Out/MyCinema.exe"
 # Go may reuse an existing executable with the same build ID, including an
 # earlier data overlay. Always build a clean launcher before appending once.
 if(Test-Path -LiteralPath $target){Remove-Item -LiteralPath $target -Force}
 & $Go build -buildvcs=false -trimpath -ldflags "-s -w -H=windowsgui -X main.version=$Version" -o (Join-Path $Out 'MyCinema.exe') ./cmd/mycinema
 if($LASTEXITCODE){throw 'Launcher build failed'}
 $exe=[IO.File]::Open((Join-Path $root "$Out/MyCinema.exe"),[IO.FileMode]::Append)
 $runtime=[IO.File]::OpenRead($RuntimeZip)
 try{$runtime.CopyTo($exe);$magic=[Text.Encoding]::ASCII.GetBytes('MYCINEMA-RUNTIME');$exe.Write($magic,0,$magic.Length);$size=[BitConverter]::GetBytes([uint64]$runtime.Length);$exe.Write($size,0,$size.Length)}finally{$runtime.Dispose();$exe.Dispose()}
}finally{Pop-Location}
