package main

import (
	"flag"
	"github.com/voltix-vault/voltix-router/config"
	"github.com/voltix-vault/voltix-router/db"
	"github.com/voltix-vault/voltix-router/server"
	"github.com/voltix-vault/voltix-router/storage"
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
	dbs, err := db.NewDBStorage(cfg.ConnectionString)
	if err != nil {
		panic(err)
	}
	s := server.NewServer(cfg.Port, store, dbs)
	if err := s.StartServer(); err != nil {
		panic(err)
	}
	err = dbs.Close()
	if err != nil {
		panic(err)
	}
	err = store.Close()
	if err != nil {
		panic(err)
	}
}
