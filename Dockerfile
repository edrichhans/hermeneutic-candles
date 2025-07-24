FROM golang:1.24

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o candles-server ./cmd/server/main.go

EXPOSE 8080

CMD ["./candles-server"]
