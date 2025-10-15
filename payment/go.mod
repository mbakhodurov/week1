module github.com/mbakhodurov/homeworks/week1/payment

go 1.24.4

replace github.com/mbakhodurov/homeworks/week1/shared => ../shared

require (
	github.com/google/uuid v1.6.0
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.27.3
	github.com/mbakhodurov/week1/shared v0.0.0-20251014122127-09b21258d792
	google.golang.org/grpc v1.76.0
)

require (
	github.com/envoyproxy/protoc-gen-validate v1.2.1 // indirect
	golang.org/x/net v0.42.0 // indirect
	golang.org/x/sys v0.34.0 // indirect
	golang.org/x/text v0.29.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20251007200510-49b9836ed3ff // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20251002232023-7c0ddcbb5797 // indirect
	google.golang.org/protobuf v1.36.10 // indirect
)
