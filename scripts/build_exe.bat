@echo off
setlocal EnableExtensions
rem ============================================================
rem  A4-Poster-Tiler - build A4-Poster-Tiler.exe (Go, single file)
rem  Output: A4-Poster-Tiler.exe in the project root.
rem  Version comes from the VERSION file in the project root.
rem  -H windowsgui: no black console window; the web page has a Console button instead.
rem  Needs Go (to build) and Node (to run the JS tests). Users of the exe need neither.
rem  Keep this file pure ASCII + CRLF. Usage: build_exe.bat [--no-pause]
rem ============================================================
pushd "%~dp0.."
set "ROOT=%CD%"
set "MODULE=github.com/TJHS-Co-Curricular/A4-Poster-Tiler"
set "EXE_NAME=A4-Poster-Tiler.exe"
set "APP_ZIP=A4-Poster-Tiler_Portable.zip"
set "NOPAUSE="
if /i "%~1"=="--no-pause" set "NOPAUSE=1"

echo ============================================================
echo  Building %EXE_NAME%
echo ============================================================

echo [1/5] Checking Go and Node ...
call :find_go
if not defined GO_OK (
  echo [ERROR] Go not found. Download windows-amd64.msi from https://go.dev/dl/ , install it, then run this file again.
  goto :fail
)
where node >nul 2>nul
if errorlevel 1 (
  echo [ERROR] Node not found. It is needed for the tests. Install the LTS version from https://nodejs.org/ , then run this file again.
  goto :fail
)
set /p VER=<"%ROOT%\VERSION"
set "VER=%VER: =%"
for /f "tokens=3" %%v in ('go version') do set "GO_VER=%%v"
for /f "delims=" %%v in ('node --version') do set "NODE_VER=%%v"
echo       Go %GO_VER%, Node %NODE_VER%, building v%VER%

echo [2/5] Closing %EXE_NAME% if it is running ...
tasklist /fi "imagename eq %EXE_NAME%" 2>nul | find /i "%EXE_NAME%" >nul
if not errorlevel 1 (
  taskkill /im "%EXE_NAME%" /f >nul 2>nul
  timeout /t 1 /nobreak >nul
)

rem Leftover from a previous build while the exe was open (deleted when no longer locked)
if exist "%ROOT%\%EXE_NAME%~" del /q "%ROOT%\%EXE_NAME%~" >nul 2>nul

echo [3/5] Checking code (gofmt, go vet) ...
set "FMT="
for /f "delims=" %%f in ('gofmt -l cmd internal config web tests') do set "FMT=%%f"
if defined FMT (
  echo [ERROR] Some Go files are not formatted. Run: gofmt -w .
  goto :fail
)
go vet ./...
if errorlevel 1 goto :fail

echo [4/5] Running tests (stop if any fails) ...
go test ./...
if errorlevel 1 goto :fail
node --test tests/
if errorlevel 1 goto :fail

echo [5/5] Compiling ...
set "COMMIT="
where git >nul 2>nul && for /f %%c in ('git -C "%ROOT%" rev-parse --short HEAD 2^>nul') do set "COMMIT=%%c"
for /f %%d in ('powershell -NoProfile -Command "Get-Date -Format yyyy-MM-dd"') do set "BDATE=%%d"
echo       Making exe icon and version info ...
go run ./cmd/winres
if errorlevel 1 goto :fail
set "GOOS=windows"
set "GOARCH=amd64"
set "CGO_ENABLED=0"
go build -trimpath -ldflags "-H windowsgui -s -w -X %MODULE%/internal/buildinfo.Version=%VER% -X %MODULE%/internal/buildinfo.Commit=%COMMIT% -X %MODULE%/internal/buildinfo.BuildDate=%BDATE%" -o "%ROOT%\%EXE_NAME%" ./cmd/a4-poster-tiler
if errorlevel 1 goto :fail

echo       Making portable zip (exe + config + logs) ...
go run ./cmd/portable
if errorlevel 1 goto :fail

for %%f in ("%ROOT%\%EXE_NAME%") do set "SIZE=%%~zf"
set /a SIZE_MB=%SIZE% / 1048576
echo.
echo [OK] %ROOT%\%EXE_NAME%  (v%VER%, about %SIZE_MB% MB)
echo [OK] %ROOT%\dist\%APP_ZIP%
echo      To deploy: give the teacher the zip; unzip it and double-click %EXE_NAME%.
popd
if not defined NOPAUSE pause
exit /b 0

:find_go
set "GO_OK="
where go >nul 2>nul && (set "GO_OK=1" & goto :eof)
if exist "%ProgramFiles%\Go\bin\go.exe" (
  set "PATH=%ProgramFiles%\Go\bin;%PATH%"
  set "GO_OK=1"
)
goto :eof

:fail
echo.
echo [ERROR] Build failed. Read the messages above. The exe was NOT updated.
popd
if not defined NOPAUSE pause
exit /b 1
