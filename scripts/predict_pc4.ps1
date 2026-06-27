$body = @{
  features = @{
    hour = 20
    day_of_week = 6
    month = 12
    is_weekend = 1
    voltage = 240.8
    global_reactive_power = 0.15
    global_intensity = 7.2
    sub_metering_1 = 0
    sub_metering_2 = 1
    sub_metering_3 = 18
    other_consumption = 12.5
  }
} | ConvertTo-Json -Depth 4

Invoke-RestMethod `
  -Method Post `
  -Uri http://127.0.0.1:8080/v1/predict `
  -ContentType "application/json" `
  -Body $body | ConvertTo-Json -Depth 10
