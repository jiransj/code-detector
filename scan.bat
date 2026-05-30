@echo off
chcp 65001 >nul
cd /d "%~dp0"

:: 运行扫描（运行时不需要 MinGW/CGO，已在构建时静态链接）
code-detector.exe -verbose -skip-dirs build,testdata,testdata_extreme,tests,__tests__,node_modules,mock,mocks .

pause
