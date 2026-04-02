FROM golang:1.25-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /forged-server ./cmd/forged-server

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=build /forged-server /usr/local/bin/forged-server
EXPOSE 8080
CMD ["forged-server"]
