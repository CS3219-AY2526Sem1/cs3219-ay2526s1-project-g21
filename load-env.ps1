Write-Host "Loading AWS credentials from .env file..."

Get-Content .env | ForEach-Object {
    if ($_ -match '^AWS_ACCESS_KEY_ID=(.*)$') {
        $env:AWS_ACCESS_KEY_ID = $matches[1]
        Write-Host "Set AWS_ACCESS_KEY_ID"
    }
    elseif ($_ -match '^AWS_SECRET_ACCESS_KEY=(.*)$') {
        $env:AWS_SECRET_ACCESS_KEY = $matches[1]
        Write-Host "Set AWS_SECRET_ACCESS_KEY"
    }
    elseif ($_ -match '^AWS_DEFAULT_REGION=(.*)$') {
        $env:AWS_DEFAULT_REGION = $matches[1]
        Write-Host "Set AWS_DEFAULT_REGION"
    }
}

Write-Host "AWS credentials loaded!"