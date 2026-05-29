@echo off
chcp 65001 >nul
cd /d "%~dp0"

code-detector.exe -verbose -skip-dirs testdata,testdata_extreme,tests,test,__tests__,node_modules,mock,mocks .

pause
