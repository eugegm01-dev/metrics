package example

// generate:reset
type Config struct {
	Port     int
	Host     string
	Cache    []string
	Settings map[string]bool
	Next     *Config
}
