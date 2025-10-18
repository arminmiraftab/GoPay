package gopay

import (
	"fmt"
)

type Client struct {
	factory *Factory
	config  *Config
}

func NewClient(config *Config) *Client {
	return &Client{
		factory: NewFactory(),
		config:  config,
	}
}

func (c *Client) Register(name string, initializer Initializer) {
	c.factory.RegisterDriver(name, initializer)
}

func (c *Client) GetDriver(driverName string) (Driver, error) {
	driverConfig, ok := c.config.Drivers[driverName]
	if !ok {
		return nil, fmt.Errorf("configuration for driver '%s' not found", driverName)
	}
	return c.factory.New(driverName, driverConfig)
}
