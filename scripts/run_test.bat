@echo off
cd /d "%~dp0"
echo Running email extraction test...
echo.
email-extractor.exe --test http://3elrappresentanze.it/
echo.
pause

