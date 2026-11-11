# 同步上游源码到 internal/hsc
#
# 背景
# - 本机 github.com:443 直连不可用，不能用 git clone；改从 codeload.github.com 下载 tarball
# - 上游 go/ 全部是 package main，按功能白名单复制后改包名为 hsc
# - 不改上游文件逻辑：GUI 适配集中在 internal/hsc/gui.go，重跑本脚本不会覆盖它
#
# 用法
#   pwsh -File scripts/sync-upstream.ps1                # 下载 main 并同步
#   pwsh -File scripts/sync-upstream.ps1 -NoFetch       # 复用 .cache/upstream 里已解压的源码
#   pwsh -File scripts/sync-upstream.ps1 -Ref <分支|标签>
#
# 白名单
# 登录（13 个）：config.go httpc.go jar.go jar_uid.go util.go session.go
#   creds.go cred_cmd.go cas.go rsa512.go captcha.go login_cmd.go logout.go
# 课程与任务点（8 个）：courses.go course.go init_cmd.go args.go parallel.go
#   scan_doc.go video.go video_refresh.go
# 数据
#   go/data/captcha-model.json -> internal/hsc/data/captcha-model.json
#
# 注意
# - 新增 GUI 功能时先扩白名单（例如课程/播放/测验），再重跑本脚本
# - 上游 *_cmd.go 里的打印文案由 gui.go 的编排替换成事件，不改上游逻辑

param(
    [string]$Ref = "main",
    [string]$Repo = "xcdoge/hust-smartcourse-cli",
    [switch]$NoFetch
)

$ErrorActionPreference = "Stop"
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

$root = Split-Path -Parent $PSScriptRoot
$dest = Join-Path $root "internal\hsc"
$cacheRoot = Join-Path $root ".cache\upstream"
$srcRoot = Join-Path $cacheRoot "src"

$whitelist = @(
    "config.go", "httpc.go", "jar.go", "jar_uid.go", "util.go", "session.go",
    "creds.go", "cred_cmd.go", "cas.go", "rsa512.go", "captcha.go",
    "login_cmd.go", "logout.go",
    "courses.go", "course.go", "init_cmd.go", "args.go", "parallel.go",
    "scan_doc.go", "video.go", "video_refresh.go", "quiz.go"
)
$dataFiles = @("captcha-model.json")

if (-not $NoFetch) {
    if (Test-Path $srcRoot) { Remove-Item -LiteralPath $srcRoot -Recurse -Force }
    New-Item -ItemType Directory -Path $srcRoot -Force | Out-Null
    $tarball = Join-Path $cacheRoot "hust-smartcourse-cli-$Ref.tar.gz"
    $url = "https://codeload.github.com/$Repo/tar.gz/refs/heads/$Ref"
    Write-Host "下载上游 tarball: $url"
    Invoke-WebRequest -Uri $url -OutFile $tarball -UseBasicParsing
    Write-Host "解压到: $srcRoot"
    tar -xzf $tarball -C $srcRoot
    if ($LASTEXITCODE -ne 0) { throw "tar 解压失败（exit=$LASTEXITCODE）" }
}

if (-not (Test-Path $srcRoot)) { throw "没有缓存源码，先不带 -NoFetch 跑一次: $srcRoot" }
$repoDir = Get-ChildItem -LiteralPath $srcRoot -Directory | Select-Object -First 1
if (-not $repoDir) { throw "缓存目录里没有解压后的仓库: $srcRoot" }
$goDir = Join-Path $repoDir.FullName "go"
if (-not (Test-Path $goDir)) { throw "上游 go/ 目录不存在: $goDir" }

New-Item -ItemType Directory -Path $dest -Force | Out-Null
New-Item -ItemType Directory -Path (Join-Path $dest "data") -Force | Out-Null

$utf8 = New-Object System.Text.UTF8Encoding($false)
$copied = 0
foreach ($name in $whitelist) {
    $src = Join-Path $goDir $name
    if (-not (Test-Path $src)) { throw "白名单文件不存在: $name" }
    $text = [System.IO.File]::ReadAllText($src, [System.Text.Encoding]::UTF8)
    if ($text -notmatch "(?m)^package main\s*$") {
        throw "$name 不是 package main，检查上游目录结构"
    }
    $text = [regex]::Replace($text, "(?m)^package main\s*$", "package hsc", 1)
    [System.IO.File]::WriteAllText((Join-Path $dest $name), $text, $utf8)
    $copied++
}

foreach ($name in $dataFiles) {
    $src = Join-Path $goDir "data\$name"
    if (-not (Test-Path $src)) { throw "数据文件不存在: go/data/$name" }
    Copy-Item -LiteralPath $src -Destination (Join-Path $dest "data\$name") -Force
}

$refFile = Join-Path $cacheRoot "last-sync.txt"
New-Item -ItemType Directory -Path $cacheRoot -Force | Out-Null
"ref=$Ref`nfetchedAt=$(Get-Date -Format o)`nfiles=$($whitelist.Count)" |
    Set-Content -LiteralPath $refFile -Encoding UTF8

Write-Host "已同步 $copied 个 .go 文件 + $($dataFiles.Count) 个数据文件 -> $dest"
Write-Host "下一步: go build ./internal/hsc"
