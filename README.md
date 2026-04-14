# masik
My personal gui

## Go PATH

PowerShell command to add Go to user `Path`:

```powershell
$paths = @([Environment]::GetEnvironmentVariable('Path', 'User').Split(';') + 'C:\Program Files\Go\bin', "$env:USERPROFILE\go\bin") | Where-Object { $_ } | Select-Object -Unique; [Environment]::SetEnvironmentVariable('Path', ($paths -join ';'), 'User')
```
