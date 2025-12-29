package auth

import (
	"context"
	"errors"
	"os"

	customUtil "github.com/app-clone-tod-utils"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
)

type GithubConfig struct {
	oauth2.Config
}

type GithubUser struct {
	ID         int    `json:"id"`
	Username   string `json:"login"`
	AVATAR_URL string `json:"avatar_url"`
}

func (c *GithubConfig) SetGithubClient(scopes []string) error {
	err := errors.New("missing environment variables")

	id, ok := os.LookupEnv("GITHUB_CLIENT_ID")
	if !ok {
		return err
	}

	clientSecret, ok := os.LookupEnv("GITHUB_CLIENT_SECRET")
	if !ok {
		return err
	}

	serverURL, ok := os.LookupEnv("BASE_SERVER_URL")
	if !ok {
		return err
	}

	c.ClientID = id
	c.ClientSecret = clientSecret
	c.Endpoint = github.Endpoint
	c.Scopes = scopes
	c.RedirectURL = serverURL + customUtil.GITHUB_REDIRECT_URI

	return nil
}

func (g *GithubUser) GetID(pool *pgxpool.Pool, ctx context.Context, user *AuthRequest) error {
	// Get auto-generated ID from local DB
	if err := pool.QueryRow(ctx, `SELECT "id" FROM "User" WHERE "githubId" = $1`, g.ID).Scan(&user.ID); err != nil {
		return err
	}

	return nil
}

func (g *GithubUser) Signup(pool *pgxpool.Pool, ctx context.Context, user *AuthRequest) error {
	var err error

	// If user is new, add to local db and retrieve auto-generated db ID
	if err = pool.QueryRow(ctx, `INSERT INTO "User" ("githubId") VALUES ($1) RETURNING "id"`, g.ID).Scan(&user.ID); err != nil {
		return err
	}

	// Populate user profile
	res, err := pool.Exec(ctx, `INSERT INTO "Profile" ("userId", "firstName", "lastName") VALUES ($1, $2, $3)`, user.ID, g.Username, g.Username)
	if err != nil {
		return err
	}

	if res.RowsAffected() != 1 {
		return errors.New("cannot populate profile of new user")
	}

	return nil
}
