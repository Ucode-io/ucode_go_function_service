
# About
OpenFaaS (Function as a Service) is an open-source framework for building serverless functions that can run in containers. It allows developers to deploy and manage event-driven functions easily on any cloud or on-premises environment using Kubernetes or Docker. OpenFaaS simplifies scaling, monitoring, and managing functions, enabling a quick and efficient development of microservices and event-driven applications.

# Create your function 
1. First make sure that you have faac-cli installed. You can install it using any of the following methods. 
[link](https://docs.openfaas.com/cli/install/)

2. To initialize function code run the following command.
```
make gen-function
```

# Tools and Packages
1. github.com/golanguzb70/redis-cache [Doc](https://github.com/golanguzb70/redis-cache)
 > This is package you can implement caching easiliy with redis. 
 > To enable caching go to `template/golang-middleware/main.go:33` and set to true.
 > Because we are using http based dog watch type of open faas template. It keeps connection open with redis.
2. github.com/golanguzb70/ucode-sdk [Doc](https://github.com/golanguzb70/ucode-sdk) 
 > This package makes it easy to integrate with ucode apis. 
 > All important ucode APIs are made functions which can be called as just methods.
 > Refer to documentation to learn more.

# Testing in local
To test it local go to `function-name/cmd/main.go` and write the request and run `go run main.go`.

# Logging
1. The Zerolog logger is already configured in the code, so you can easily add log messages.

2. To log messages within your function code, use the Zerolog package like this:

```go
in.Log.Info().Msg("This is an informational log message")
in.Log.Error().Msg("This is an error log message")
```

3. To view the logs from your OpenFaaS function, go to the Ucode Grafana console, find your OpenFaaS service, and check the logs there.

Link: [Link](https://grafana.u-code.io/explore?schemaVersion=1&panes=%7B%22Aj_%22:%7B%22datasource%22:%22loki%22,%22queries%22:%5B%7B%22refId%22:%22A%22,%22expr%22:%22%7Bnamespace%3D%5C%22openfaas-fn%5C%22,%20app%3D%5C%22sodiq-school-zkbio-sync-student-data%5C%22%7D%20%7C%3D%20%60%60%22,%22queryType%22:%22range%22,%22datasource%22:%7B%22type%22:%22loki%22,%22uid%22:%22loki%22%7D,%22editorMode%22:%22builder%22%7D%5D,%22range%22:%7B%22from%22:%22now-1h%22,%22to%22:%22now%22%7D%7D%7D&orgId=1)
```
Login: devs
Password: xoo3aiy3ouS5AeNgig8n
```


**Note:** We switched from using Telegram bots for logging to this method because Telegram often experiences downtime and can be slow. This new logging approach provides more reliable and faster log access.
