@echo off
setlocal

set "ENV_FILE=.env"
set "PRIVATE_KEY_VALUE=YourPrivateKeyHere"

:: Check if the .env file already exists
if exist "%ENV_FILE%" (
    echo %ENV_FILE% exists, appending to it.
) else (
    echo %ENV_FILE% does not exist, creating it.
    type nul > "%ENV_FILE%"
)

:: Append the PRIVATE_KEY variable to the .env file
echo PRIVATE_KEY= >> "%ENV_FILE%"
echo JITO_ROC= >> "%ENV_FILE%"
echo GEYSER_RPC= >> "%ENV_FILE%"
echo PRIVATE_KEY_WITH_FUNDS= >> "%ENV_FILE%

endlocal
