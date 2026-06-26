$body = @{ iterations = 10; learning_rate = 1.0; reset_model = $true } | ConvertTo-Json
Invoke-RestMethod -Method Post -Uri http://127.0.0.1:8080/v1/train -ContentType "application/json" -Body $body | ConvertTo-Json -Depth 10
