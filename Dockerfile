FROM golang:1.22-alpine AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN apk add --no-cache tzdata
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /tcc-monitor .

FROM gcr.io/distroless/static-debian12
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /tcc-monitor /tcc-monitor
EXPOSE 8080
ENTRYPOINT ["/tcc-monitor"]
