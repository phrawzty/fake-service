version=v0.5.5

protos:
	protoc -I grpc/protos/ grpc/protos/api.proto --go_out=plugins=grpc:grpc/api

build_linux:
	CGO_ENABLED=0 GOOS=linux go build -o bin/fake-service

build_docker: build_linux
	docker build -t nicholasjackson/fake-service:${version} .

run_downstream:
	TRACING_ZIPKIN=/dev/null NAME=web HTTP_CLIENT_KEEP_ALIVES=false UPSTREAM_WORKERS=2 UPSTREAM_URIS="http://localhost:9091,grpc://localhost:9093" go run main.go

run_downstream_errors:
	TRACING_ZIPKIN=/dev/null NAME=web HTTP_CLIENT_KEEP_ALIVES=false ERROR_RATE=1 ERROR_CODE=500 UPSTREAM_WORKERS=2 UPSTREAM_URIS="http://localhost:9091,grpc://localhost:9093" go run main.go

run_upstream_1:
	NAME=payment MESSAGE="Hello from upstream 1" LISTEN_ADDR=localhost:9091  UPSTREAM_URIS=http://localhost:9092 go run main.go

run_upstream_2:
	NAME=currency MESSAGE="Hello from upstream 2" LISTEN_ADDR=localhost:9092 go run main.go

run_downstream_grpc:
	NAME=api HTTP_CLIENT_KEEP_ALIVES=false TRACING_ZIPKIN=/dev/stderr UPSTREAM_WORKERS=2 LISTEN_ADDR=localhost:9093 UPSTREAM_URIS="grpc://localhost:9094" go run main.go

run_upstream_grpc:
	NAME=accounts SERVER_TYPE=grpc TRACING_ZIPKIN=/dev/stderr MESSAGE="Hello from grpc upstream" LISTEN_ADDR=localhost:9094 go run main.go

call_downstream:
	curl localhost:9090

test:
	filewatcher --idle-timeout 24h gotestsum

run_functional_ddog: build_docker
	cd examples/docker-compose-datadog && docker-compose up

run_functional_ddog_consul: build_docker
	cd examples/docker-compose-datadog && docker-compose -f docker-compose-consul.yml up

run_functional_jaeger: build_docker
	cd examples/docker-compose-jaeger && docker-compose up
