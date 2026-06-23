@echo off
chcp 65001 >nul
echo ========================================
echo   通讯管理机 - 编译打包
echo ========================================

echo.
echo [1/3] 编译主程序...
go build -o com-manager.exe main.go
if %errorlevel% neq 0 (
    echo 编译主程序失败！
    pause
    exit /b 1
)
echo 编译成功: com-manager.exe

echo.
echo [2/3] 编译看门狗...
go build -o watchdog.exe cmd/watchdog/main.go
if %errorlevel% neq 0 (
    echo 编译看门狗失败！
    pause
    exit /b 1
)
echo 编译成功: watchdog.exe

echo.
echo [3/3] 打包安装程序...
:: 检查 Inno Setup 是否安装
set ISCC="%ProgramFiles(x86)%\Inno Setup 6\ISCC.exe"
if not exist %ISCC% (
    set ISCC="%ProgramFiles%\Inno Setup 6\ISCC.exe"
)
if not exist %ISCC% (
    echo 未找到 Inno Setup 6，请先安装: https://jrsoftware.org/isinfo.php
    echo 安装后重新运行此脚本
    pause
    exit /b 1
)

%ISCC% installer.iss
if %errorlevel% neq 0 (
    echo 打包失败！
    pause
    exit /b 1
)

echo.
echo ========================================
echo   打包完成！安装程序在 release\ 目录
echo ========================================
pause
