package main

import (
	"flag"

	"github.com/vultisig/vultisig-relay/config"
	"github.com/vultisig/vultisig-relay/server"
	"github.com/vultisig/vultisig-relay/storage"
)

func main() {
	var cfgFile string
	flag.StringVar(&cfgFile, "config", "config.json", "config file")
	flag.Parse()

	cfg := config.LoadConfig(cfgFile)

	var store *storage.RedisStorage
	var err error
	if cfg.RedisURI != "" {
		store, err = storage.NewRedisStorageFromURI(cfg.RedisURI)
	} else {
		store, err = storage.NewRedisStorage(cfg.RedisServer)
	}
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
