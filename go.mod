module github.com/puppetlabs/relay-pls

go 1.14

require (
	cloud.google.com/go/bigquery v1.14.0
	github.com/golang/mock v1.4.4
	github.com/golang/protobuf v1.4.3
	github.com/google/tink/go v1.5.0
	github.com/google/uuid v1.1.2
	github.com/hashicorp/vault/api v1.0.4
	github.com/pelletier/go-toml v1.8.1 // indirect
	github.com/puppetlabs/leg/encoding v0.1.0
	github.com/spf13/viper v1.7.1
	github.com/stretchr/testify v1.6.1
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.14.0
	go.opentelemetry.io/otel v0.14.0
	go.opentelemetry.io/otel/exporters/stdout v0.14.0
	go.opentelemetry.io/otel/sdk v0.14.0
	google.golang.org/api v0.36.0
	google.golang.org/grpc v1.34.0
	google.golang.org/protobuf v1.25.0
)
