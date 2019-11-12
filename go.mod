module github.com/aurorasolar/go-service-nr-base

go 1.13

require (
	github.com/aws/aws-sdk-go v1.25.32 // indirect
	github.com/aws/aws-sdk-go-v2 v0.15.0
	github.com/deepmap/oapi-codegen v1.3.0
	github.com/getkin/kin-openapi v0.2.1-0.20190729060947-8785b416cb32
	github.com/inconshreveable/mousetrap v1.0.0 // indirect
	github.com/jmoiron/sqlx v1.2.0
	github.com/labstack/echo/v4 v4.1.6
	github.com/labstack/gommon v0.2.9
	github.com/lib/pq v1.1.0 // indirect
	github.com/mattn/go-sqlite3 v1.11.0 // indirect
	github.com/newrelic/go-agent v2.15.0+incompatible
	github.com/newrelic/newrelic-telemetry-sdk-go v0.1.0
	github.com/spf13/cobra v0.0.3
	github.com/spf13/pflag v1.0.3
	github.com/stretchr/testify v1.3.0
	go.uber.org/multierr v1.4.0 // indirect
	go.uber.org/zap v1.10.0
	google.golang.org/appengine v1.6.5 // indirect
)

replace github.com/labstack/echo/v4 v4.1.6 => github.com/Cyberax/echo/v4 v4.1.6-fork
