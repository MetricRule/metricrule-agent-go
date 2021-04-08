compile_api_proto:
	@protoc -I=api/proto --go_out=api/proto --go_opt=module=github.com/metricrule-sidecar-tfserving/api/proto api/proto/metricrule_metric_configuration.proto
