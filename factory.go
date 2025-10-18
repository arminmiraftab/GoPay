package gopay

import "fmt"

type Initializer func(config DriverConfig) (Driver, error)

type Factory struct {
	initializers map[string]Initializer
}

func NewFactory() *Factory {
	return &Factory{
		initializers: make(map[string]Initializer),
	}
}

func (f *Factory) RegisterDriver(name string, initializer Initializer) {
	f.initializers[name] = initializer
}

func (f *Factory) New(driverName string, config DriverConfig) (Driver, error) {
	initializer, ok := f.initializers[driverName]
	if !ok {
		return nil, fmt.Errorf("driver '%s' is not registered", driverName)
	}
	return initializer(config)
}
