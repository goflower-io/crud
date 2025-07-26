module github.com/goflower-io/crud

go 1.22.3

toolchain go1.22.5

require (
	github.com/pganalyze/pg_query_go/v5 v5.1.0
	github.com/pingcap/parser v0.0.0-20220622031236-3bca03d3057b
	github.com/rqlite/sql v0.0.0-20221103124402-8f9ff0ceb8f0
	golang.org/x/mod v0.17.0
)

require google.golang.org/protobuf v1.34.0 // indirect

require (
	github.com/cznic/mathutil v0.0.0-20181122101859-297441e03548 // indirect
	github.com/go-sql-driver/mysql v1.8.1 // indirect
	github.com/google/go-cmp v0.6.0 // indirect
	github.com/pingcap/errors v0.11.5-0.20210425183316-da1aaba5fb63 // indirect
	github.com/pingcap/log v1.1.0 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	github.com/stretchr/testify v1.9.0 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.27.0 // indirect
	golang.org/x/text v0.3.6 // indirect
	gopkg.in/natefinch/lumberjack.v2 v2.2.1 // indirect
)

retract [v1.0.0, v1.0.1]
