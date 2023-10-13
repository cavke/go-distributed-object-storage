FROM golang:1.21-alpine as builder

# Set the current working directory inside the container
WORKDIR /app

# Copy go modules
COPY go.* ./

# Run go mod
RUN go mod download

# Copy code sources
COPY ./cmd ./cmd
COPY ./internal ./internal

# Build Go
RUN cd cmd && go build -o app

# Docker is used as a base image so you can easily start playing around in the container using the Docker command line client.
FROM docker

WORKDIR /

COPY --from=builder /app /opt/app
RUN apk add bash curl

ENTRYPOINT [ "/opt/app/cmd/app" ]