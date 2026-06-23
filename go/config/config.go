package config

import (
	"github.com/caarlos0/env/v11"

	"naomi.run/service"
)

type Config struct {
	DatabaseDSN   string `env:"DATABASE_DSN,required"`
	MigrationsDir string `env:"MIGRATIONS_DIR" envDefault:"/migrations"`
	Service       service.Config
}

func Load() (Config, error) {
	return env.ParseAs[Config]()
}
