FROM golang:1.21.0 as builder

RUN mkdir -p $GOPATH/src/gitlab.udevs.io/ucode/ucode_go_function_service 
WORKDIR $GOPATH/src/gitlab.udevs.io/ucode/ucode_go_function_service

COPY . ./

RUN export CGO_ENABLED=0 && \
    export GOOS=linux && \
    go mod vendor && \
    make build && \
    mv ./bin/ucode_go_function_service /

ENTRYPOINT ["/ucode_go_function_service"]


