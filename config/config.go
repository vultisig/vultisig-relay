package config

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
