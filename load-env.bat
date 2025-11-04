@echo off
echo Loading AWS credentials from .env file...
for /f "usebackq delims== tokens=1,2" %%G in (".env") do (
    if "%%G"=="AWS_ACCESS_KEY_ID" (
        set "AWS_ACCESS_KEY_ID=%%H"
        echo Set AWS_ACCESS_KEY_ID
    )
    if "%%G"=="AWS_SECRET_ACCESS_KEY" (
        set "AWS_SECRET_ACCESS_KEY=%%H"
        echo Set AWS_SECRET_ACCESS_KEY
    )
    if "%%G"=="AWS_DEFAULT_REGION" (
        set "AWS_DEFAULT_REGION=%%H"
        echo Set AWS_DEFAULT_REGION
    )
)
echo AWS credentials loaded!