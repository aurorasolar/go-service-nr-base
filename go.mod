module github.com/aurorasolar/go-service-nr-base

go 1.13

require (
	github.com/aws/aws-sdk-go-v2 v0.15.0
	github.com/getkin/kin-openapi v0.2.1-0.20190729060947-8785b416cb32
	github.com/inconshreveable/mousetrap v1.0.0 // indirect
	github.com/labstack/echo/v4 v4.1.6
	github.com/labstack/gommon v0.2.9
	github.com/lib/pq v1.2.0
	github.com/mattn/go-isatty v0.0.9 // indirect
	github.com/newrelic/go-agent v2.15.0+incompatible
	github.com/newrelic/newrelic-telemetry-sdk-go v0.1.0
	github.com/pkg/errors v0.8.1 // indirect
	github.com/spf13/cobra v0.0.3
	github.com/spf13/pflag v1.0.3
	github.com/stretchr/testify v1.4.0
	go.uber.org/multierr v1.4.0 // indirect
	go.uber.org/zap v1.10.0
	golang.org/x/crypto v0.0.0-20190911031432-227b76d455e7 // indirect
	golang.org/x/net v0.0.0-20190813141303-74dc4d7220e7 // indirect
	golang.org/x/sys v0.0.0-20190826190057-c7b8b68b1456 // indirect
)

replace github.com/labstack/echo/v4 v4.1.6 => github.com/Cyberax/echo/v4 v4.1.6-fork
