Invoke-RestMethod `
  -Method Post `
  -Uri http://127.0.0.1:8080/v1/evaluate | ConvertTo-Json -Depth 10
