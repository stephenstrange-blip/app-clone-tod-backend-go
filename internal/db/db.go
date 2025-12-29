package db

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func Init() (*pgxpool.Pool, error) {
	url, ok := os.LookupEnv("DATABASE_URL")
	if !ok {
		log.Fatalln("Cannot find database url")
	}

	// set pool config according to url string
	config, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, err
	}

	config.AfterConnect = afterConnect
	config.AfterRelease = afterRelease

	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		return nil, err
	}

	// Immediately test if db is running
	err = pool.Ping(context.Background())
	if err != nil {
		return nil, err
	}

	return pool, nil
}

func afterConnect(ctx context.Context, c *pgx.Conn) error {
	// fmt.Printf("\n----> New connection config: %+v\n", c.Config().Copy())
	fmt.Printf("\n----> New connection on %s expiring in %s\n", c.Config().Host, c.Config().ConnectTimeout)
	return nil
}

func afterRelease(c *pgx.Conn) bool {
	// fmt.Printf("\n<---- Released connection config: %+v\n", c.Config().Copy())
	fmt.Printf("\n<---- Connection on %s expiring in %s is released\n", c.Config().Host, c.Config().ConnectTimeout)
	return true
}
