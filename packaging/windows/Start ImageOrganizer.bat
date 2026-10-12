@echo off
rem Starter ImageOrganizer og aabner det i browseren.
rem Ligger der en embedding-model i llama\models, startes llama-server ogsaa,
rem saa soegning efter indhold virker.
cd /d "%~dp0"
set MODEL=
set MMPROJ=
for %%f in (llama\models\*.gguf) do (
  echo %%~nf | findstr /i "mmproj" >nul && (set "MMPROJ=%%f") || (set "MODEL=%%f")
)
if defined MODEL if defined MMPROJ (
  echo Starter llama-server med %MODEL% ...
  start "llama-server (soegning efter indhold)" /min llama\llama-server.exe -m "%MODEL%" --mmproj "%MMPROJ%" --embeddings --port 8095 --host 127.0.0.1
)
if defined MODEL if not defined MMPROJ echo BEMAERK: projector-filen (mmproj*.gguf) mangler i llama\models - soegning efter indhold startes ikke.
start "" "http://127.0.0.1:8420"
echo ImageOrganizer koerer. Luk dette vindue for at stoppe programmet.
imageorganizer.exe %*
pause
