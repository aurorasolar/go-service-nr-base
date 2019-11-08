module github.com/aurorasolar/go-service-base

go 1.13

require (
	github.com/aws/aws-sdk-go-v2 v0.10.0
	github.com/getkin/kin-openapi v0.2.1-0.20190729060947-8785b416cb32
	github.com/inconshreveable/mousetrap v1.0.0 // indirect
	github.com/jackc/pgx/v4 v4.0.0-pre1
	github.com/labstack/echo/v4 v4.1.6
	github.com/labstack/gommon v0.2.9
	github.com/mattn/go-sqlite3 v1.11.0
	github.com/newrelic/go-agent v2.15.0+incompatible
	github.com/newrelic/newrelic-telemetry-sdk-go v0.1.0
	github.com/spf13/cobra v0.0.3
	github.com/spf13/pflag v1.0.3
	github.com/stretchr/testify v1.3.0
	go.uber.org/zap v1.10.0
	golang.org/x/crypto v0.0.0-20190701094942-4def268fd1a4 // indirect
	golang.org/x/net v0.0.0-20190724013045-ca1201d0de80 // indirect
	golang.org/x/sys v0.0.0-20190712062909-fae7ac547cb7 // indirect
	golang.org/x/xerrors v0.0.0-20190717185122-a985d3407aa7 // indirect
)

replace github.com/labstack/echo/v4 v4.1.6 => github.com/Cyberax/echo/v4 v4.1.6-fork
