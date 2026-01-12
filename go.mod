module github.com/go-anyway/framework-xxljob

go 1.25.4

require (
	github.com/go-anyway/framework-log v1.0.0
	github.com/go-anyway/framework-metrics v1.0.0
	github.com/go-anyway/framework-trace v1.0.0
	github.com/xxl-job/xxl-job-executor-go v1.2.0
)

replace (
	github.com/go-anyway/framework-log => ../core/log
	github.com/go-anyway/framework-metrics => ../metrics
	github.com/go-anyway/framework-trace => ../trace
)
