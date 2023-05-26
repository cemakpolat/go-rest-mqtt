# Build stage
FROM golang:1.17-alpine AS build

WORKDIR /app

# Copy the Go module files
COPY go.mod go.sum ./

# Download the Go module dependencies
RUN go mod download

# Copy the rest of the source code
COPY . .

# Build the Go application
RUN go build -o app

# Final stage
FROM alpine:latest

WORKDIR /app

# Copy the built executable from the build stage
COPY --from=build /app/app .

# Expose the application port
EXPOSE 8080

# Run the Go application
CMD ["./app"]

