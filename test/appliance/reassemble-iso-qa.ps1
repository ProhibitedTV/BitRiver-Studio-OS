# BitRiver Studio OS — experimental QA ISO reconstruction (Windows PowerShell).
# No admin privileges, network, or physical disks are required.
$ErrorActionPreference = "Stop"
$base = $PSScriptRoot
$chunks = Join-Path $base "chunks"
$files = @(Get-ChildItem -LiteralPath $chunks -File -Filter "bitriver-studio-os-qa.iso.part*" | Sort-Object Name)
if ($files.Count -lt 2) { throw "Missing ISO parts. Extract the whole Actions QA artifact first." }
for ($i = 0; $i -lt $files.Count; $i++) {
    $expected = "bitriver-studio-os-qa.iso.part{0:D3}" -f $i
    if ($files[$i].Name -cne $expected) { throw "Missing or unexpected chunk: expected $expected" }
}
$line = @(Get-Content -LiteralPath (Join-Path $base "ISO-SHA256SUMS") |
    Where-Object { $_ -cmatch '^[0-9a-f]{64}  bitriver-studio-os-qa\.iso$' })
if ($line.Count -ne 1) { throw "Missing or invalid complete-ISO SHA256 manifest" }
$expectedHash = $line[0].Substring(0, 64)
$target = Join-Path $base "bitriver-studio-os-qa.iso"
if (Test-Path -LiteralPath $target) { throw "Refusing to overwrite an existing ISO: $target" }
$out = [System.IO.File]::Open($target, [System.IO.FileMode]::CreateNew)
try {
    foreach ($part in $files) {
        $source = [System.IO.File]::OpenRead($part.FullName)
        try { $source.CopyTo($out) }
        finally { $source.Dispose() }
    }
} finally { $out.Dispose() }
$actualHash = (Get-FileHash -LiteralPath $target -Algorithm SHA256).Hash.ToLowerInvariant()
if ($actualHash -cne $expectedHash) {
    Remove-Item -LiteralPath $target -Force
    throw "QA ISO SHA256 mismatch. Corrupt reconstruction removed."
}
Write-Host "SHA256 verified: $actualHash"
Write-Host "NOT A RELEASE. Test ONLY in an isolated VM with a disposable virtual disk."
