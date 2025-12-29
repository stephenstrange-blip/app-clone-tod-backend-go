package main

import (
	"context"
	"errors"
	"fmt"
	"log"

	db "github.com/app-clone-tod-db"
	customUtil "github.com/app-clone-tod-utils"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"golang.org/x/crypto/bcrypt"
)

type Profile struct {
	firstName  string
	lastName   string
	bio        string
	title      string
	profileURL string
}

type User struct {
	username string
	password string
}

type Category struct {
	name string
}

type Reacts struct {
	name string
}

func main() {
	if err := godotenv.Load("../.env"); err != nil {
		log.Fatalf("Error loading env file, %s\n", err.Error())
	}

	config, err := db.Init()
	if err != nil {
		log.Fatal(err.Error())
	}

	ctx := context.Background()

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	defer pool.Close()

	if err := ResetCount(pool, []string{"User", "Category", "Rooms", "Session", "Reacts"}); err != nil {
		fmt.Println(err.Error())
		return
	}

	tx, err := pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadWrite, IsoLevel: pgx.ReadCommitted})
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	defer tx.Rollback(ctx)

	adminUser := &User{
		username: "Admin",
		password: "admin",
	}

	if res, _ := tx.Exec(ctx, `SELECT '' FROM "User" WHERE id = $1`, 1); res.RowsAffected() != 1 {
		if err := adminUser.SignAdminUser(tx); err != nil {
			fmt.Println(err.Error())
			return
		}
		fmt.Println("Admin User created")

		adminProfile := &Profile{
			firstName:  "Admin",
			lastName:   "Admin",
			bio:        "Bio",
			title:      "Title",
			profileURL: "Profile URL",
		}

		if err := adminProfile.SignAdminProfile(tx); err != nil {
			fmt.Println(err.Error())
			return
		}
		fmt.Println("Admin Profile created")
	}

	like := &Reacts{name: "like"}
	heart := &Reacts{name: "heart"}

	_, err = tx.Exec(ctx, `INSERT INTO "Reacts" ("id", "name") VALUES ($1, $2)`, 1, like.name)
	fmt.Println("Like react created")
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	_, err = tx.Exec(ctx, `INSERT INTO "Reacts" ("id", "name") VALUES ($1, $2)`, 2, heart.name)
	fmt.Println("Heart react created")
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	generic := &Category{name: "Generic"}

	_, err = tx.Exec(ctx, `INSERT INTO "Category" ("id", "name") VALUES ($1, $2)`, 1, generic.name)
	if err != nil {
		fmt.Println("commit failed:", err)
		return
	}
	fmt.Println("Category created")

	if err := tx.Commit(ctx); err != nil {
		fmt.Println("commit failed:", err)
		return
	}

	fmt.Println("Finished seeding")

}

func (u *User) SignAdminUser(tx pgx.Tx) error {
	pw, err := bcrypt.GenerateFromPassword([]byte(u.password), customUtil.HASH_COST)
	if err != nil {
		return err
	}

	_, err = tx.Exec(context.Background(), `INSERT INTO "User" ("username", "password") VALUES ($1, $2) RETURNING "id"`, u.username, pw)
	if err != nil {
		return err
	}

	return nil
}

func (p *Profile) SignAdminProfile(tx pgx.Tx) error {

	res, err := tx.Exec(context.Background(), `INSERT INTO "Profile" ("userId", "firstName", "lastName", "bio", "title", "profileUrl") VALUES ($1, $2, $3, $4, $5, $6)`, 1, p.firstName, p.lastName, p.bio, p.title, p.profileURL)
	if err != nil {
		return err
	}

	if res.RowsAffected() != 1 {
		return errors.New("error creating admin profile")
	}

	return nil
}

func ResetCount(pool *pgxpool.Pool, tableNames []string) error {
	ctx := context.Background()

	tx, err := pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadWrite, IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Restarts autoincrement count to 1
	for _, table := range tableNames {
		query := fmt.Sprintf(`TRUNCATE TABLE "%s" RESTART IDENTITY CASCADE`, table)
		if _, err := tx.Exec(ctx, query); err != nil {
			return err
		}
		fmt.Printf("Restarting table %s\n", table)
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}
	return nil
}
