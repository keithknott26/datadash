FROM golang:1.22-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /datadash ./cmd

FROM alpine:3.20
COPY --from=build /datadash /usr/local/bin/datadash
WORKDIR /data
ENTRYPOINT ["datadash"]