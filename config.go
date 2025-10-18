package gopay

type DriverConfig map[string]string

type Config struct {
	Drivers map[string]DriverConfig
}
