@echo off
rem Builds ImageOrganizer for Windows with face recognition (needs Go and MinGW gcc)
rem and packs dist\ImageOrganizer-windows-x64.zip. onnxruntime.dll is included
rem when it lies next to this script.
setlocal
cd /d "%~dp0"
where go >nul 2>nul || (echo Go blev ikke fundet. Installer Go fra https://go.dev/dl & goto :fail)
where gcc >nul 2>nul || (echo gcc ^(MinGW^) blev ikke fundet. Koer: winget install BrechtSanders.WinLibs.POSIX.UCRT  - og aabn et nyt vindue. & goto :fail)
set CGO_ENABLED=1
set GOOS=windows
set GOARCH=amd64
if exist dist\ImageOrganizer rmdir /s /q dist\ImageOrganizer
mkdir dist\ImageOrganizer
echo Bygger... (foerste gang henter Go de noedvendige pakker)
go build -trimpath -ldflags="-s -w" -o dist\ImageOrganizer\imageorganizer.exe . || goto :fail
copy /y "packaging\windows\Start ImageOrganizer.bat" dist\ImageOrganizer\ >nul
copy /y packaging\windows\LAES-MIG.txt dist\ImageOrganizer\ >nul
copy /y LICENSE dist\ImageOrganizer\LICENSE.txt >nul
if exist onnxruntime.dll (copy /y onnxruntime.dll dist\ImageOrganizer\ >nul) else (echo BEMAERK: onnxruntime.dll ligger ikke ved siden af scriptet - ansigtsgenkendelse kraever den i programmappen.)
if exist dist\ImageOrganizer-windows-x64.zip del dist\ImageOrganizer-windows-x64.zip
powershell -NoProfile -Command "Compress-Archive -Path dist\ImageOrganizer -DestinationPath dist\ImageOrganizer-windows-x64.zip" || goto :fail
echo.
echo Faerdig: dist\ImageOrganizer-windows-x64.zip
pause
exit /b 0
:fail
echo.
echo Bygningen fejlede.
pause
exit /b 1
