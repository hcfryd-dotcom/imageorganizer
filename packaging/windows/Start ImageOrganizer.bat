@echo off
rem Starter ImageOrganizer og aabner det i browseren.
cd /d "%~dp0"
start "" "http://127.0.0.1:8420"
echo ImageOrganizer koerer. Luk dette vindue for at stoppe programmet.
imageorganizer.exe %*
pause
