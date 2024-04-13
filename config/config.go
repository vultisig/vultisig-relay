package config

import (
	"encoding/json"
	"fmt"
	"os"
)

type Config struct {
	Port        int64       `json:"port"`
	RedisServer RedisServer `json:"redis_server"`
}

type RedisServer struct {
	Addr     string `json:"addr"`
	User     string `json:"user"`
	Password string `json:"password"`
	DB       int    `json:"db"`
}

// LoadConfig loads the configuration from a file.
func LoadConfig(file string) (*Config, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, fmt.Errorf("fail to open config file %s, err: %w", file, err)
	}
	defer func(f *os.File) {
		err := f.Close()
		if err != nil {
			fmt.Println("fail to close file", err)
		}
	}(f)
	var cfg Config
	if err := json.NewDecoder(f).Decode(&cfg); err != nil {
		return nil, fmt.Errorf("fail to decode config file %s, err: %w", file, err)
	}
	return &cfg, nil
}
