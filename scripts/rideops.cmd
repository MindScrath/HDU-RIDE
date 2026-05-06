@echo off
setlocal
set ROOT=%~dp0..
pushd "%ROOT%\backend" || exit /b 1
go run . ops %*
set CODE=%ERRORLEVEL%
popd
exit /b %CODE%
