param(
    [Parameter(Mandatory=$true)][string]$Source,
    [Parameter(Mandatory=$true)][string]$Spec
)

# Spec format (JSON): { "package": "handlers", "targets": [ { "file": "x.go", "ranges": [[start,end],...] } ] }
$cfg = Get-Content $Spec -Raw | ConvertFrom-Json
$lines = Get-Content $Source
$dir = Split-Path $Source -Parent

$moved = New-Object 'System.Collections.Generic.HashSet[int]'

foreach ($t in $cfg.targets) {
    $out = New-Object System.Collections.Generic.List[string]
    $out.Add("package $($cfg.package)") | Out-Null
    $out.Add("") | Out-Null
    $out.Add("import (") | Out-Null
    $out.Add(")") | Out-Null
    $out.Add("") | Out-Null
    foreach ($r in $t.ranges) {
        $s = [int]$r[0]; $e = [int]$r[1]
        for ($i = $s; $i -le $e; $i++) {
            $out.Add($lines[$i-1]) | Out-Null
            $moved.Add($i) | Out-Null
        }
        $out.Add("") | Out-Null
    }
    $path = Join-Path $dir $t.file
    Set-Content -Path $path -Value $out -Encoding UTF8
    Write-Host "WROTE $path ($($out.Count) lines)"
}

# Rewrite source without moved lines
$remain = New-Object System.Collections.Generic.List[string]
for ($i = 1; $i -le $lines.Count; $i++) {
    if (-not $moved.Contains($i)) { $remain.Add($lines[$i-1]) | Out-Null }
}
Set-Content -Path $Source -Value $remain -Encoding UTF8
Write-Host "REWROTE $Source ($($remain.Count) lines)"
