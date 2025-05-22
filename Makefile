
.PHONY: gen
gen:
	mkdir -p gen/stt_service
	protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative --proto_path=. proto/stt_service.proto
