@echo off
rem Start A4-Poster-Tiler in LAN mode (colleagues on the same network can open it).
rem Same as setting access = lan in config\config.ini, but only for this run.
pushd "%~dp0.."
if not exist "%CD%\A4-Poster-Tiler.exe" (
  echo [ERROR] A4-Poster-Tiler.exe not found. Put this file in the scripts folder next to the exe folder, or run build_exe.bat first.
  pause
  popd
  exit /b 1
)
"%CD%\A4-Poster-Tiler.exe" --lan
popd
