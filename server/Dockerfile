FROM golang:1.27-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /forged-server ./cmd/forged-server

FROM alpine:3.24
RUN apk add --no-cache ca-certificates
COPY --from=build /forged-server /usr/local/bin/forged-server
EXPOSE 8080
CMD ["forged-server"]
