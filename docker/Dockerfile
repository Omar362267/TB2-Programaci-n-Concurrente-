FROM golang:1.22-alpine AS build
WORKDIR /app
COPY go.mod ./
COPY . .
RUN go build -o /pc3 ./cmd/pc3

FROM alpine:3.20
WORKDIR /app
COPY --from=build /pc3 /usr/local/bin/pc3
CMD ["pc3", "-input", "data/raw/household_power_consumption.txt", "-workers", "4"]
