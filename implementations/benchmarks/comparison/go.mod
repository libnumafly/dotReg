module github.com/libnumafly/dotReg/benchmarks/comparison

go 1.22

require (
	github.com/libnumafly/dotReg/implementations/go v0.0.0
	github.com/pelletier/go-toml/v2 v2.4.3
	go.yaml.in/yaml/v3 v3.0.5
	gopkg.in/ini.v1 v1.67.3
)

replace github.com/libnumafly/dotReg/implementations/go => ../../go
