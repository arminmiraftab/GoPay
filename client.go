//package gopay
//
//import (
//	"fmt"
//)
//
//type Client struct {
//	factory *Factory
//	config  *Config
//}
//
//func NewClient(config *Config) *Client {
//	return &Client{
//		factory: NewFactory(),
//		config:  config,
//	}
//}
//
//func (c *Client) Register(name string, initializer Initializer) {
//	c.factory.RegisterDriver(name, initializer)
//}
//
//func (c *Client) GetDriver(driverName string) (Driver, error) {
//	driverConfig, ok := c.config.Drivers[driverName]
//	if !ok {
//		return nil, fmt.Errorf("configuration for driver '%s' not found", driverName)
//	}
//	return c.factory.New(driverName, driverConfig)
//}

package gopay

import (
	"fmt"
	"sync"
)

type InitializerFunc func(config DriverConfig) (Driver, error)

type Client struct {
	config  *Config
	drivers map[string]Driver
	mu      sync.RWMutex
}

func NewClient(config *Config) *Client {
	return &Client{
		config:  config,
		drivers: make(map[string]Driver),
	}
}

func (c *Client) Register(name string, initializer InitializerFunc) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.drivers[name]; exists {
		return fmt.Errorf("driver '%s' is already registered", name)
	}

	driverConfig, ok := c.config.Drivers[name]
	if !ok {
		return fmt.Errorf("config for driver '%s' not found", name)
	}

	driver, err := initializer(driverConfig)
	if err != nil {
		return fmt.Errorf("failed to initialize driver '%s': %w", name, err)
	}

	c.drivers[name] = driver
	return nil
}

func (c *Client) GetDriver(name string) (Driver, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	driver, ok := c.drivers[name]
	if !ok {
		return nil, fmt.Errorf("driver '%s' not found or not registered", name)
	}
	return driver, nil
}
