package main

import (
	"context"
	"net/http"
	"net/url"
	"time"

	"github.com/Xe/ln"
	"github.com/caarlos0/env"
	"github.com/heroku/x/scrub"
	"github.com/horseville/horseville/internal/database"
	"github.com/horseville/horseville/internal/redigo"
	"github.com/jmoiron/sqlx"
)

type config struct {
	DatabaseURL string `env:"DATABASE_URL,required"`
	NatsURL     string `env:"NATS_URL,required"`
	RedisURL    string `env:"REDIS_URL,required"`
	Port        string `env:"PORT" envDefault:"5000"`
}

func (c config) F() ln.F {
	result := ln.F{
		"env_PORT": c.Port,
	}

	u, err := url.Parse(c.DatabaseURL)
	if err != nil {
		result["env_DATABASE_URL_err"] = err
	} else {
		result["env_DATABASE_URL"] = scrub.URL(u)
	}

	u, err = url.Parse(c.NatsURL)
	if err != nil {
		result["env_NATS_URL_err"] = err
	} else {
		result["env_NATS_URL"] = scrub.URL(u)
	}

	u, err = url.Parse(c.RedisURL)
	if err != nil {
		result["env_REDIS_URL_err"] = err
	} else {
		result["env_REDIS_URL"] = scrub.URL(u)
	}

	return result
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ctx = ln.WithF(ctx, ln.F{"in": "main"})

	var cfg config
	err := env.Parse(&cfg)
	if err != nil {
		ln.FatalErr(ctx, err)
	}

	nc, err := nats.Connect(cfg.NatsURL)
	if err != nil {
		ln.FatalErr(ctx, err)
	}
	_ = nc

	rp, err := redigo.NewRedisPoolFromURL(cfg.RedisURL)
	if err != nil {
		ln.FatalErr(ctx, err)
	}
	_ = rp

	ctx = ln.WithF(ctx, cfg.F())

	err = database.Migrate(cfg.DatabaseURL)
	if err != nil && err.Error() != "no change" {
		ln.FatalErr(ctx, err)
	}

	// wait for postgres
	time.Sleep(2 * time.Second)
	db, err := sqlx.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		ln.FatalErr(ctx, err)
	}

	db.SetMaxOpenConns(30)

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		_, err := db.Exec("SELECT 1+1")
		if err != nil {
			ln.Error(r.Context(), err)
			http.Error(w, "database error", http.StatusInternalServerError)
			return
		}

	})
}
