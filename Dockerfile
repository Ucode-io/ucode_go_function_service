# Stage 1: Builder for Go
FROM golang:1.21.1 as builder

# Create directory for Go source
RUN mkdir -p $GOPATH/src/gitlab.udevs.io/ucode/ucode_go_object_builder_service 
WORKDIR $GOPATH/src/gitlab.udevs.io/ucode/ucode_go_object_builder_service

# Copy the local package files to the container's workspace.
COPY . ./

# Install Go dependencies and build the Go binary
RUN export CGO_ENABLED=0 && \
    export GOOS=linux && \
    go mod vendor && \
    make build && \
    mv ./bin/ucode_go_object_builder_service /

# Stage 2: Node.js
FROM node:18-alpine as node_builder

# Set the working directory
RUN mkdir -p /js/pkg/js_parser
WORKDIR /js/pkg/js_parser

# Copy package.json and package-lock.json
COPY ./pkg/js_parser/package*.json ./

# Install Node.js dependencies
RUN npm install

# Copy the rest of the application files
COPY /pkg/js_parser/ ./

RUN npm install hot-formula-parser
#RUN npm run build
# Build the Node.js application if necessary (uncomment if you have a build step)
# RUN npm run build

# Stage 3: Final image
FROM alpine

RUN apk add --no-cache nodejs npm
# Copy the Go binary from the builder stage
COPY --from=builder /ucode_go_object_builder_service /ucode_go_object_builder_service

# Copy Node.js application files if needed
COPY --from=node_builder /js/pkg/js_parser /js/pkg/js_parser
# Copy migrations
COPY migrations/postgres ./migrations/postgres 

# Set entrypoint for the container
ENTRYPOINT ["/ucode_go_object_builder_service"]



