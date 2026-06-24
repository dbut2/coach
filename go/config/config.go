package config

import (
	"github.com/caarlos0/env/v11"

	"naomi.run/service"
	"naomi.run/telemetry"
)

type Config struct {
	DatabaseDSN   string `env:"DATABASE_DSN,required"`
	MigrationsDir string `env:"MIGRATIONS_DIR" envDefault:"/migrations"`
	Service       service.Config
	Telemetry     telemetry.Config
}

func Load() (Config, error) {
	return env.ParseAs[Config]()
}
