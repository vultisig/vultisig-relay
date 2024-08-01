package main

import (
	"flag"

	"github.com/vultisig/vultisig-router/config"
	"github.com/vultisig/vultisig-router/server"
	"github.com/vultisig/vultisig-router/storage"
)

func main() {
	var cfgFile string
	flag.StringVar(&cfgFile, "config", "config.json", "config file")
	flag.Parse()

	cfg, err := config.LoadConfig(cfgFile)
	if err != nil {
		panic(err)
	}
	store, err := storage.NewRedisStorage(cfg.RedisServer)
	if err != nil {
		panic(err)
	}
	s := server.NewServer(cfg.Port, store)
	if err := s.StartServer(); err != nil {
		panic(err)
	}

	err = store.Close()
	if err != nil {
		panic(err)
	}
}
