FROM golang:1.21.0 as builder

# 
RUN mkdir -p $GOPATH/src/gitlab.udevs.io/ucode/ucode_go_function_service 
WORKDIR $GOPATH/src/gitlab.udevs.io/ucode/ucode_go_function_service

# Copy the local package files to the container's workspace.
COPY . ./

# installing depends and build
RUN export CGO_ENABLED=0 && \
    export GOOS=linux && \
    go mod vendor && \
    make build && \
    mv ./bin/ucode_go_function_service /

# FROM gitlab.udevs.io:5050/docker/docker:ucode
# COPY --from=builder ucode_go_admin_api_gateway .
ENTRYPOINT ["/ucode_go_function_service"]


