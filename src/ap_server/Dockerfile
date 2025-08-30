# minimal, two-stage
FROM golang:1.21-alpine AS builder
WORKDIR /app

# create a throwaway module name (can be anything)
RUN go mod init local/app
COPY main.go .
RUN go build -o main ./main.go

FROM alpine:3.19
WORKDIR /app
COPY --from=builder /app/main .
EXPOSE 8989
CMD ["./main"]
