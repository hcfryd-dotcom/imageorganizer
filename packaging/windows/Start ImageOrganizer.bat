@echo off
rem Starter ImageOrganizer og aabner det i browseren.
rem Ligger der en embedding-model i llama\models, startes llama-server ogsaa,
rem saa soegning efter indhold virker.
rem Koerer programmet allerede, aabnes kun browseren: et nyt program kan
rem ikke aabne biblioteket, mens det gamle har det, og det gamle afbrydes ikke.
cd /d "%~dp0"
tasklist /fi "imagename eq imageorganizer.exe" | find /i "imageorganizer.exe" >nul && (
  start "" "http://127.0.0.1:8420"
  exit /b 0
)
set MODEL=
set MMPROJ=
for %%f in (llama\models\*.gguf) do (
  echo %%~nf | findstr /i "mmproj" >nul && (set "MMPROJ=%%f") || (set "MODEL=%%f")
)
rem -ngl 99 runs it on the graphics card when llama has a GPU backend, ggml-vulkan.dll; without one it runs on the CPU as before.
tasklist /fi "imagename eq llama-server.exe" | find /i "llama-server.exe" >nul && set MODEL=
if defined MODEL if defined MMPROJ (
  echo Starter llama-server med %MODEL% ...
  start "llama-server (soegning efter indhold)" /min llama\llama-server.exe -m "%MODEL%" --mmproj "%MMPROJ%" --embeddings --port 8095 --host 127.0.0.1 -ngl 99
)
if defined MODEL if not defined MMPROJ echo BEMAERK: projector-filen (mmproj*.gguf) mangler i llama\models - soegning efter indhold startes ikke.
rem Browseren aabnes et oejeblik efter programmet, som viser en opstartsside
rem med en bjaelke over hvad det laver, til billederne er klar.
start "" /min cmd /c "ping -n 3 127.0.0.1 >nul & explorer http://127.0.0.1:8420"
echo ImageOrganizer koerer. Luk dette vindue for at stoppe programmet.
imageorganizer.exe %*
pause
