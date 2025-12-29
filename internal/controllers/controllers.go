package controllers

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"time"

	auth "github.com/app-clone-tod-auth"
	customUtil "github.com/app-clone-tod-utils"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Unexported key type used in passing userID in request context
type key int

// userKey is the unexported key for user.User values in Contexts.
// Clients use NewUserContext and UserFromContext instead of using this key directly.
var userKey key

// NewContext returns a new Context that carries value u.
func NewUserContext(ctx context.Context, u int) context.Context {
	return context.WithValue(ctx, userKey, u)
}

// FromContext returns the User value stored in ctx, if any.
func UserFromContext(ctx context.Context) (int, bool) {
	id, ok := ctx.Value(userKey).(int)
	return id, ok
}

// type that has methods for each db object
type Controller struct{}

// Middleware chaining
//
// https://www.alexedwards.net/blog/organize-your-go-middleware-without-dependencies
type Chain []func(http.Handler) http.Handler

// Returns a chain of function calls with fn as the innermost (aka last being called)
func (c *Chain) Handle(fn http.HandlerFunc) http.Handler {
	return c.HandleChain(fn)
}

// Called by Chain.Handle() to create a chain of function calls
func (c *Chain) HandleChain(handler http.Handler) http.Handler {
	for _, fn := range slices.Backward(*c) {
		handler = fn(handler)
	}
	return handler
}

// Used for public endpoints
var BaseChain = Chain{AcceptJSON, AddTimeoutLimit}

// Used for endpoints requiring logged-in userID
var PrivateChain = append(BaseChain, GetUser)

// Appends userID from a jwt to client request.
// Returns unauthorized if token is malformed, missing, etc.
func GetUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(wr http.ResponseWriter, r *http.Request) {
		id, err := auth.GetCookieWithToken(r)

		if err != nil {
			fmt.Printf("error (cookie): %s\n", err.Error())
			wr.WriteHeader(http.StatusUnauthorized)
			return
		}

		if id == 0 {
			fmt.Printf("error (cookie): id returned is 0")
			wr.WriteHeader(http.StatusUnauthorized)
			return
		}

		// https://stackoverflow.com/questions/40891345/fix-should-not-use-basic-type-string-as-key-in-context-withvalue-golint
		// Then call UseFromContext() to get userID value
		ctx := NewUserContext(r.Context(), id)
		req := r.WithContext(ctx)

		next.ServeHTTP(wr, req)
	})
}

func AddTimeoutLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(wr http.ResponseWriter, r *http.Request) {
		var (
			timeout time.Duration
			err     error
		)

		if t := r.FormValue("timeout"); t != "" {
			timeout, err = time.ParseDuration(t)
		} else {
			// Default timeout of 5s
			timeout = customUtil.HTTP_TIMEOUT
		}

		if err != nil {
			fmt.Printf("error (internal): %s\n", err.Error())
			wr.WriteHeader(http.StatusInternalServerError)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()

		r = r.WithContext(ctx)
		fmt.Printf("\nTimeout limit of %s added...\n", timeout.String())
		next.ServeHTTP(wr, r)
	})
}

// Sets content-type to application/json
func AcceptJSON(next http.Handler) http.Handler {
	return http.HandlerFunc(func(wr http.ResponseWriter, r *http.Request) {
		wr.Header().Set("Content-type", "application/json")
		next.ServeHTTP(wr, r)
	})
}

func AcceptFormURLEncoded(next http.Handler) http.Handler {
	return http.HandlerFunc(func(wr http.ResponseWriter, r *http.Request) {
		wr.Header().Set("Content-type", "application/x-www-form-urlencoded")
		next.ServeHTTP(wr, r)
	})
}

type Author struct {
	FirstName string `json:"firstName,omitzero"`
	LastName  string `json:"lastName,omitzero"`
}

// repository-layer function used by several Controller methods
func FetchAuthor(p *pgxpool.Pool, ctx context.Context, authorID int) (*Author, error) {
	x := &Author{}

	err := p.QueryRow(context.Background(), `SELECT "firstName","lastName" FROM "Profile" WHERE "userId" = $1`, authorID).Scan(&x.FirstName, &x.LastName)
	if err != nil {
		return x, err
	}

	return x, nil
}
