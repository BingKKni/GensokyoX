#requires -Version 7.0
<#
.SYNOPSIS
    在 Windows pwsh 下交叉编译 Gensokyo 为 Linux amd64 二进制。

.EXAMPLE
    pwsh -ExecutionPolicy Bypass -File .\build-linux-amd64.ps1

.EXAMPLE
    pwsh -ExecutionPolicy Bypass -File .\build-linux-amd64.ps1 -OutputName gensokyo11 -Force
#>

[CmdletBinding()]
param(
    # 不填写时，自动按 output/gensokyoN 的最大编号递增。
    [string]$OutputName = "",

    # 国内网络默认使用 goproxy.cn；如需禁用可传空字符串：-GoProxy ""。
    [string]$GoProxy = "https://goproxy.cn,direct",

    # 指定的输出文件已存在时允许覆盖。
    [switch]$Force,

    # 不执行 go mod download，直接构建。
    [switch]$SkipModuleDownload,

    # 构建完成后不自动更新 start.sh 中的 BIN=./gensokyoN。
    [switch]$NoUpdateStartSh
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

function Set-ChineseFriendlyConsole {
    try {
        $utf8NoBom = [System.Text.UTF8Encoding]::new($false)
        [Console]::InputEncoding = $utf8NoBom
        [Console]::OutputEncoding = $utf8NoBom
        $script:OutputEncoding = $utf8NoBom

        if ($IsWindows) {
            & chcp.com 65001 > $null 2>&1
        }
    }
    catch {
        Write-Warning "切换控制台到 UTF-8 失败，继续构建：$($_.Exception.Message)"
    }
}

function Write-Step {
    param([Parameter(Mandatory)][string]$Message)
    Write-Host "[构建] $Message" -ForegroundColor Cyan
}

function Write-Success {
    param([Parameter(Mandatory)][string]$Message)
    Write-Host "[完成] $Message" -ForegroundColor Green
}

function Invoke-Go {
    param([Parameter(Mandatory)][string[]]$Arguments)

    & go @Arguments
    if ($LASTEXITCODE -ne 0) {
        throw "go $($Arguments -join ' ') 执行失败，退出码：$LASTEXITCODE"
    }
}

function Get-NextOutputName {
    param([Parameter(Mandatory)][string]$OutputDir)

    $maxNumber = 0
    Get-ChildItem -LiteralPath $OutputDir -File -Filter "gensokyo*" | ForEach-Object {
        if ($_.Name -match '^gensokyo(\d+)$') {
            $number = [int]$Matches[1]
            if ($number -gt $maxNumber) {
                $maxNumber = $number
            }
        }
    }

    return "gensokyo$($maxNumber + 1)"
}

function Test-ValidFileName {
    param([Parameter(Mandatory)][string]$Name)

    return $Name.IndexOfAny([System.IO.Path]::GetInvalidFileNameChars()) -lt 0
}

Set-ChineseFriendlyConsole

$outputDir = $PSScriptRoot
if ([string]::IsNullOrWhiteSpace($outputDir)) {
    $outputDir = Split-Path -Parent $MyInvocation.MyCommand.Path
}

$repoRoot = (Resolve-Path -LiteralPath (Join-Path -Path $outputDir -ChildPath "..")).Path
$goModPath = Join-Path -Path $repoRoot -ChildPath "go.mod"

if (-not (Test-Path -LiteralPath $goModPath -PathType Leaf)) {
    throw "未找到 go.mod：$goModPath。请确认脚本位于仓库的 output 文件夹内。"
}

if ($null -eq (Get-Command go -ErrorAction SilentlyContinue)) {
    throw "未找到 go 命令。请先安装 Go，并确保 go.exe 已加入 PATH。"
}

if ([string]::IsNullOrWhiteSpace($OutputName)) {
    $OutputName = Get-NextOutputName -OutputDir $outputDir
}

if (-not (Test-ValidFileName -Name $OutputName)) {
    throw "输出文件名包含非法字符：$OutputName"
}

$outputPath = Join-Path -Path $outputDir -ChildPath $OutputName
if ((Test-Path -LiteralPath $outputPath -PathType Leaf) -and -not $Force) {
    throw "输出文件已存在：$outputPath。如需覆盖，请添加 -Force。"
}

$env:GO111MODULE = "on"
$env:GOOS = "linux"
$env:GOARCH = "amd64"
$env:CGO_ENABLED = "0"

if (-not [string]::IsNullOrWhiteSpace($GoProxy)) {
    $env:GOPROXY = $GoProxy
}

Write-Step "仓库目录：$repoRoot"
Write-Step "输出文件：$outputPath"
Write-Step "目标平台：GOOS=$env:GOOS GOARCH=$env:GOARCH CGO_ENABLED=$env:CGO_ENABLED"
Write-Step "$(go version)"

Push-Location -LiteralPath $repoRoot
try {
    if (-not $SkipModuleDownload) {
        Write-Step "下载 Go 模块依赖"
        Invoke-Go -Arguments @("mod", "download")
    }

    Write-Step "开始编译 Linux amd64 二进制"
    Invoke-Go -Arguments @("build", "-trimpath", "-ldflags", "-s -w", "-o", $outputPath, ".")
}
finally {
    Pop-Location
}

if (-not (Test-Path -LiteralPath $outputPath -PathType Leaf)) {
    throw "构建命令已结束，但未找到输出文件：$outputPath"
}

$artifact = Get-Item -LiteralPath $outputPath
$sizeMb = [Math]::Round($artifact.Length / 1MB, 2)
Write-Success "已生成 Linux amd64 文件：$($artifact.FullName) ($sizeMb MB)"

if (-not $NoUpdateStartSh) {
    $startShPath = Join-Path -Path $outputDir -ChildPath "start.sh"
    if (Test-Path -LiteralPath $startShPath -PathType Leaf) {
        $startSh = Get-Content -LiteralPath $startShPath -Raw -Encoding UTF8
        $binLine = "BIN=`"./$OutputName`" #这里后面编号代表第X次编译，每次编译后都要重命名给后面加一个数字编号，方便启动失败时回退到上一个编号的版本"

        $binRegex = [regex]'(?m)^BIN="\./[^\r\n"]+".*$'
        if ($binRegex.IsMatch($startSh)) {
            $updatedStartSh = $binRegex.Replace($startSh, $binLine, 1)
            Set-Content -LiteralPath $startShPath -Value $updatedStartSh -Encoding UTF8 -NoNewline
            Write-Success "已更新 start.sh：BIN=./$OutputName"
        }
        else {
            Write-Warning '未在 start.sh 中找到 BIN="./..."，已跳过自动更新。'
        }
    }
}
