package config

import (
	"encoding/json"
	"os"
	"strconv"
)

type Config struct {
	Port        int64       `json:"port"`
	RedisServer RedisServer `json:"redis_server"`
	RedisURI    string      `json:"redis_uri"`
}

type RedisServer struct {
	Addr     string `json:"addr"`
	User     string `json:"user"`
	Password string `json:"password"`
	DB       int    `json:"db"`
}

func LoadConfig(file string) *Config {
	cfg := &Config{
		Port: 80,
	}

	f, err := os.Open(file)
	if err == nil {
		defer func() {
			_ = f.Close()
		}()
		_ = json.NewDecoder(f).Decode(cfg)
	}

	if portStr := os.Getenv("PORT"); portStr != "" {
		p, err := strconv.ParseInt(portStr, 10, 64)
		if err == nil {
			cfg.Port = p
		}
	}

	if redisURI := os.Getenv("REDIS_URI"); redisURI != "" {
		cfg.RedisURI = redisURI
	}

	return cfg
}
