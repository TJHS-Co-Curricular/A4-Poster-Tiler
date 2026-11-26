@echo off
rem Allow colleagues to reach A4-Poster-Tiler (port 5030-5039) through Windows Firewall.
rem Only the "Private" network profile is opened. Right-click this file and choose "Run as administrator".
net session >nul 2>nul
if errorlevel 1 (
  echo [ERROR] Please right-click this file and choose "Run as administrator".
  pause
  exit /b 1
)
netsh advfirewall firewall delete rule name="A4-Poster-Tiler" >nul 2>nul
netsh advfirewall firewall add rule name="A4-Poster-Tiler" dir=in action=allow protocol=TCP localport=5030-5039 profile=private
if errorlevel 1 (
  echo [ERROR] Could not add the firewall rule.
  pause
  exit /b 1
)
echo [OK] Firewall rule "A4-Poster-Tiler" added (TCP 5030-5039, private networks only).
pause
