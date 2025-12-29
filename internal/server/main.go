package main

import (
	"encoding/gob"
	"flag"
	"fmt"
	"log"
	"net/http"

	auth "github.com/app-clone-tod-auth"
	controllers "github.com/app-clone-tod-controllers"
	db "github.com/app-clone-tod-db"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/joho/godotenv"
)

type AppDescription struct {
	Name string
	Year int
}

var dbPool *pgxpool.Pool

func init() {
	var err error

	if err := godotenv.Load("../.env"); err != nil {
		log.Fatalf("Error loading env file, %s\n", err.Error())
	}

	// Db config holds db url (along with other configs) used for starting concurrent connections
	if dbPool, err = db.Init(); err != nil {
		log.Fatal(err.Error())
	}

}

func main() {
	var (
		port = flag.String("port", "8080", "Set server port")
		host = flag.String("host", "localhost", "Set host server")
	)

	flag.Parse()

	defer dbPool.Close()

	// Register for storing custom data types
	gob.Register(&AppDescription{})

	err := godotenv.Load("../.env")
	if err != nil {
		log.Fatalf("Error loading env file, %s\n", err.Error())
	}

	// middleware chains
	base := controllers.BaseChain
	// involves getting userID
	protected := controllers.PrivateChain

	auth := &auth.AuthHandler{}
	ctr := &controllers.Controller{}

	http.Handle("POST "+*host+"/logout/{$}", base.Handle(auth.Logout))
	http.Handle("POST "+*host+"/signup/{$}", base.Handle(auth.Signup(dbPool)))
	http.Handle("POST "+*host+"/auth/local/{$}", base.Handle(auth.AuthLocal(dbPool)))

	http.Handle("GET "+*host+"/auth/github/{$}", base.Handle(auth.AuthGithub()))
	http.Handle("GET "+*host+"/auth/google/{$}", base.Handle(auth.AuthGoogle()))
	http.Handle("GET "+*host+"/auth/github/callback/{$}", base.Handle(auth.AuthGithubCallback(dbPool)))
	http.Handle("GET "+*host+"/auth/google/callback/{$}", base.Handle(auth.AuthGoogleCallback(dbPool)))

	http.Handle("GET "+*host+"/users/auth/me/{$}", base.Handle(auth.AuthMe))
	http.Handle(*host+"/users/profile/", protected.Handle(ctr.Profile(dbPool)))
	http.Handle(*host+"/users/request/", protected.Handle(ctr.Request(dbPool)))
	http.Handle(*host+"/users/network/", protected.Handle(ctr.Network(dbPool)))
	http.Handle(*host+"/users/reaction/", protected.Handle(ctr.Reaction(dbPool)))
	http.Handle("GET "+*host+"/users/chat/{chatID}", protected.Handle(ctr.Chat(dbPool)))

	http.Handle(*host+"/users/post/{$}", protected.Handle(ctr.BasePostRoute(dbPool)))
	http.Handle(*host+"/users/post/{postID}", protected.Handle(ctr.DynamicPostRoute(dbPool)))
	http.Handle(*host+"/users/post/{postID}/comment/{commentID}", protected.Handle(ctr.Comment(dbPool)))

	fmt.Printf("\nServer listening on http://%s:%s\n", *host, *port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", *port), nil))
}
