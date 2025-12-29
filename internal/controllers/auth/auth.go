package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/oauth2"

	customUtil "github.com/app-clone-tod-utils"
)

type AuthHandler struct {
}

type AuthResponse struct {
	Message string      `json:"message,omitzero"`
	User    AuthRequest `json:"user,omitzero"`
	Auth    bool        `json:"authenticated"`
}

type AuthRequest struct {
	ID       int    `json:"id,omitzero"`
	UserName string `json:"username,omitzero"`
	GoogleID string `json:"googleId,omitzero"` // Google provides id as string
	GithubID int    `json:"githubId,omitzero"` // Github provides id as int
	Password string `json:"password,omitzero"`
}

func (ca *AuthHandler) AuthMe(wr http.ResponseWriter, r *http.Request) {
	// Extracted tokens from cookie will throw error if unauthenticated
	if userID, err := GetCookieWithToken(r); err != nil {
		fmt.Printf("err (auth) : %s", err.Error())
		wr.WriteHeader(http.StatusUnauthorized)
		return
	} else {
		fmt.Printf("userID: %d\n\n", userID)
	}

	if p, err := json.Marshal(&AuthResponse{Message: "User is Authenticated	!", Auth: true}); err != nil {
		fmt.Printf("err (auth) : %s", err.Error())
		wr.WriteHeader(http.StatusUnauthorized)
		return
	} else {
		wr.Write(p)
	}
}

func (ca *AuthHandler) AuthGoogle() http.HandlerFunc {
	return func(wr http.ResponseWriter, r *http.Request) {
		// Set oauth config
		config := &GoogleConfig{}
		if err := config.SetGoogleClient([]string{"profile", "email", "openid"}); err != nil {
			fmt.Printf("error (auth) : %s", err.Error())
			wr.WriteHeader(http.StatusInternalServerError)
			return
		}

		cookie := &http.Cookie{
			Name:    customUtil.GOOGLE_OAUTH_COOKIE_NAME,
			Value:   "oauthState",
			Expires: time.Now().Add(24 * time.Hour),
		}

		// Encrypt cookie value
		if err := WriteEncrypted(cookie); err != nil {
			fmt.Printf("error (auth) : %s", err.Error())
			wr.WriteHeader(http.StatusInternalServerError)
			return
		}

		// Encode cookie value
		if err := Write(wr, cookie); err != nil {
			fmt.Printf("error (auth) : %s", err.Error())
			wr.WriteHeader(http.StatusInternalServerError)
			return
		}

		// Set cookie for later verification in google callback handler
		http.SetCookie(wr, cookie)

		// Set the encrypted and encoded cookie value as auth state for callback verification
		url := config.AuthCodeURL(cookie.Value, oauth2.AccessTypeOffline)
		http.Redirect(wr, r, url, http.StatusFound)
	}
}

func (ca *AuthHandler) AuthGoogleCallback(pool *pgxpool.Pool) http.HandlerFunc {
	return func(wr http.ResponseWriter, r *http.Request) {
		// Third-party vendor should return code for oauth
		code := r.FormValue("code")
		if code == "" {
			wr.WriteHeader(http.StatusUnauthorized)
			return
		}

		// Perform two security measures before executing handshake with github server
		// Both produce errors if cookie value is malformed, tampered, etc.
		// 1. Verify state returned by oauth server against cookie value set by client server
		if err := VerifyState(r, customUtil.GOOGLE_OAUTH_COOKIE_NAME); err != nil {
			fmt.Printf("error (auth): %s\n", err.Error())
			wr.WriteHeader(http.StatusUnauthorized)
			return
		}

		// 2. Decrypt (and decode) cookie value
		if _, err := ReadEncrypted(r, customUtil.GOOGLE_OAUTH_COOKIE_NAME); err != nil {
			status := http.StatusInternalServerError

			if err.Error() == "invalid cookie value" {
				status = http.StatusUnauthorized
			}

			fmt.Printf("error (auth): %s\n", err.Error())
			http.Error(wr, http.StatusText(status), status)
			return
		}

		googleConfig := &GoogleConfig{}
		// Set google oauth config with a slice of scopes as argument
		if err := googleConfig.SetGoogleClient([]string{"email", "profile", "openid"}); err != nil {
			fmt.Printf("error (auth): %s\n", err.Error())
			wr.WriteHeader(http.StatusUnauthorized)
			return
		}

		// Get access token from oauth server
		token, err := googleConfig.Exchange(r.Context(), code)
		if err != nil {
			fmt.Printf("error (auth): %s\n", err.Error())
			wr.WriteHeader(http.StatusUnauthorized)
			return
		}

		// Include token in every request sent to new client
		client := googleConfig.Client(r.Context(), token)

		req, err := http.NewRequestWithContext(r.Context(), "GET", customUtil.GOOGLE_USER_ENDPOINT, nil)
		if err != nil {
			fmt.Printf("error (auth): %s\n", err.Error())
			wr.WriteHeader(http.StatusUnauthorized)
			return
		}

		// Execute api request
		resp, err := client.Do(req)
		if err != nil {
			fmt.Printf("error (auth): %s\n", err.Error())
			wr.WriteHeader(http.StatusUnauthorized)
			return
		}

		defer resp.Body.Close()

		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			fmt.Printf("error (io): %s\n", err.Error())
			wr.WriteHeader(http.StatusInternalServerError)
			return
		}

		googleUser := &GoogleUser{}

		// Extract the necessary google user details
		err = json.Unmarshal(respBody, googleUser)
		if err != nil {
			fmt.Printf("error (json): %s\n", err.Error())
			wr.WriteHeader(http.StatusInternalServerError)
			return
		}

		// user variable wil be used to create jwt token
		user := &AuthRequest{GoogleID: googleUser.ID}

		// Get local user ID
		if err := googleUser.GetID(pool, r.Context(), user); err != nil {

			if errors.Is(err, pgx.ErrNoRows) {
				// If user is new, add to local db
				if signupErr := googleUser.Signup(pool, r.Context(), user); signupErr != nil {
					fmt.Printf("error (auth): %s\n", signupErr.Error())
					wr.WriteHeader(http.StatusUnauthorized)
					return
				}
			}

			fmt.Printf("error (auth): %s\n", err.Error())
			wr.WriteHeader(http.StatusInternalServerError)
			return
		}

		// Set new cookie from new user details
		if err := SetCookieWithToken(wr, user); err != nil {
			fmt.Printf("error (auth): %s\n", err.Error())
			wr.WriteHeader(http.StatusInternalServerError)
			return
		}

		http.Redirect(nil, nil, "", http.StatusFound)
	}
}

func (ca *AuthHandler) AuthGithub() http.HandlerFunc {

	return func(wr http.ResponseWriter, r *http.Request) {
		config := &GithubConfig{}

		// Populate config fields
		if err := config.SetGithubClient([]string{"user:email"}); err != nil {
			http.Error(wr, err.Error(), http.StatusInternalServerError)
			return
		}

		// use PKCE to protect against CSRF attacks
		// https://www.ietf.org/archive/id/draft-ietf-oauth-security-topics-22.html#name-countermeasures-6
		verifier := oauth2.GenerateVerifier()

		cookie := &http.Cookie{
			Name:    customUtil.GITHUB_OAUTH_COOKIE_NAME,
			Value:   verifier,
			Expires: time.Now().Add(24 * time.Hour),
		}

		// Encrypt the cookie value (i.e. the verifier)
		if err := WriteEncrypted(cookie); err != nil {
			http.Error(wr, err.Error(), http.StatusInternalServerError)
			return
		}

		// Encode cookie value (i.e. the verifier) to base64 because ReadEncrypted() automatically decodes base64 strings
		if err := Write(wr, cookie); err != nil {
			http.Error(wr, err.Error(), http.StatusInternalServerError)
			return
		}

		// Set cookie for callback verification
		http.SetCookie(wr, cookie)

		// Redirect user to consent url to ask for permission for the scopes specified in the config.
		url := config.AuthCodeURL(cookie.Value, oauth2.AccessTypeOffline, oauth2.VerifierOption(verifier))
		http.Redirect(wr, r, url, http.StatusFound)
	}
}

func (ca *AuthHandler) AuthGithubCallback(pool *pgxpool.Pool) http.HandlerFunc {
	return func(wr http.ResponseWriter, r *http.Request) {
		code := r.FormValue("code")

		// github server should return a code to client upon successful user login/verification
		if code == "" {
			wr.WriteHeader(http.StatusUnauthorized)
			return
		}

		// Perform two security measures before executing handshake with github server
		// Both produce errors if cookie value is malformed, tampered, etc.
		// 1. Check if the auth state is equivalent to the encrypted cookie value set in HandleAuthGithub()
		if err := VerifyState(r, customUtil.GITHUB_OAUTH_COOKIE_NAME); err != nil {
			fmt.Printf("error (auth): %s\n", err.Error())
			wr.WriteHeader(http.StatusUnauthorized)
			return
		}

		// 2. Get the PKCE verifier from the same cookie.
		verifier, err := ReadEncrypted(r, customUtil.GITHUB_OAUTH_COOKIE_NAME)
		if err != nil {
			statusCode := http.StatusInternalServerError

			// Return appropriate error and status code
			if err.Error() == "invalid cookie value" {
				statusCode = http.StatusUnauthorized
			}

			fmt.Printf("error (auth): %s\n", err.Error())
			http.Error(wr, http.StatusText(statusCode), statusCode)
			return
		}

		// set github githubConfig
		githubConfig := &GithubConfig{}
		if err := githubConfig.SetGithubClient([]string{"user:email"}); err != nil {
			fmt.Printf("error (auth): %s\n", err.Error())
			wr.WriteHeader(http.StatusInternalServerError)
			return
		}

		// Exchange() will do the handshake to retrieve the initial access token.
		token, err := githubConfig.Exchange(r.Context(), code, oauth2.VerifierOption(verifier))
		if err != nil {
			fmt.Printf("error (auth): %s\n", err.Error())
			wr.WriteHeader(http.StatusInternalServerError)
			return
		}

		// The HTTP Client returned by config.Client will refresh the token as necessary.
		client := githubConfig.Client(r.Context(), token)

		// Make a request to github REST endpoint
		req, err := http.NewRequestWithContext(r.Context(), "GET", customUtil.GITHUB_USER_ENDPOINT, nil)
		req.Header.Add("Accept", "application/vnd.github+json")
		if err != nil {
			fmt.Printf("error (internal): %s\n", err.Error())
			wr.WriteHeader(http.StatusInternalServerError)
			return
		}

		// Execute and read the response of the request
		resp, err := client.Do(req)
		if err != nil {
			fmt.Printf("error (auth): %s\n", err.Error())
			wr.WriteHeader(http.StatusInternalServerError)
			return
		}
		defer resp.Body.Close()

		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			fmt.Printf("error (internal): %s\n", err.Error())
			wr.WriteHeader(http.StatusInternalServerError)
			return
		}

		githubUser := &GithubUser{}

		// Get the necessary github user details
		err = json.Unmarshal(respBody, githubUser)
		if err != nil {
			fmt.Printf("error (internal): %s\n", err.Error())
			wr.WriteHeader(http.StatusInternalServerError)
			return
		}

		user := &AuthRequest{GithubID: githubUser.ID}

		// Get local user ID
		if err = githubUser.GetID(pool, r.Context(), user); err != nil {

			if errors.Is(err, pgx.ErrNoRows) {
				// If user is new, add to local db
				if signupErr := githubUser.Signup(pool, r.Context(), user); signupErr != nil {
					fmt.Printf("error (auth): %s\n", err.Error())
					wr.WriteHeader(http.StatusInternalServerError)
					return
				}
			}

			fmt.Printf("error (auth): %s\n", err.Error())
			wr.WriteHeader(http.StatusInternalServerError)
			return
		}

		// Set new cookie from new user details
		if err = SetCookieWithToken(wr, user); err != nil {
			fmt.Printf("error (auth): %s\n", err.Error())
			wr.WriteHeader(http.StatusInternalServerError)
			return
		}

		if p, err := json.Marshal(&AuthResponse{Message: "Done!", User: *user}); err != nil {
			fmt.Printf("error (auth): %s\n", err.Error())
			wr.WriteHeader(http.StatusInternalServerError)
			return
		} else {
			wr.Write(p)
		}

	}
}

func (ca *AuthHandler) Signup(pool *pgxpool.Pool) http.HandlerFunc {
	return func(wr http.ResponseWriter, r *http.Request) {
		params, err := ParseAuthParams(r)
		if err != nil {
			fmt.Printf("error (auth): %s\n", err.Error())
			wr.WriteHeader(http.StatusUnauthorized)
			return
		}

		if err = params.LocalSignup(pool); err != nil {
			fmt.Printf("error (auth): %s\n", err.Error())
			wr.WriteHeader(http.StatusUnauthorized)
			return
		}

		if p, err := json.Marshal(&AuthResponse{Message: "Done", User: AuthRequest{ID: params.ID, UserName: params.UserName}}); err != nil {
			fmt.Printf("error (internal): %s\n", err.Error())
			wr.WriteHeader(http.StatusInternalServerError)
			return
		} else {
			wr.Write(p)
		}
	}
}

func (ca *AuthHandler) AuthLocal(pool *pgxpool.Pool) http.HandlerFunc {
	return func(wr http.ResponseWriter, r *http.Request) {
		var err error

		params, err := ParseAuthParams(r)
		if err != nil {
			fmt.Printf("errorr (auth): %s\n", err.Error())
			wr.WriteHeader(http.StatusBadRequest)
			return
		}

		// Check if credentials exist in DB
		if err = params.VerifyLocal(pool); err != nil {
			var status int
			var invalidPassword = errors.New("invalid password")

			switch {
			case errors.Is(err, invalidPassword):
				status = http.StatusUnauthorized
			default:
				status = http.StatusInternalServerError
			}

			fmt.Printf("error (auth): %s\n", err.Error())
			wr.WriteHeader(status)
			return
		}

		// Remove previous cookie before proceeding (if any)
		// No error means cookie found
		if _, err := r.Cookie(customUtil.COOKIE_NAME); err == nil {
			fmt.Println("Removing previous cookie")
			RemoveCookie(wr, r)
		}

		// Set new cookie with JWT token
		if err = SetCookieWithToken(wr, params); err != nil {
			fmt.Printf("error (auth): %s\n", err.Error())
			wr.WriteHeader(http.StatusInternalServerError)
			return
		}

		if p, err := json.Marshal(&AuthResponse{Message: "Cookie set!", User: *params}); err != nil {
			fmt.Printf("error (internal): %s\n", err.Error())
			wr.WriteHeader(http.StatusInternalServerError)
			return
		} else {
			wr.Write(p)
		}
	}
}

func (ca *AuthHandler) Logout(wr http.ResponseWriter, r *http.Request) {
	// Check if cookie exist against the secret
	RemoveCookie(wr, r)

	if p, err := json.Marshal(&AuthResponse{Message: "Cookie Removed!"}); err != nil {
		http.Error(wr, err.Error(), http.StatusInternalServerError)
		return
	} else {
		wr.Write(p)
	}
}

func ParseAuthParams(r *http.Request) (*AuthRequest, error) {
	var (
		err error
		x   = &AuthRequest{}
	)

	params, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}

	if err = json.Unmarshal(params, x); err != nil {
		return nil, err
	}

	// Required for local auth
	if (x.Password == "" && x.UserName != "") || (x.Password != "" && x.UserName == "") {
		return nil, errors.New("bad request body")
	}

	return x, nil
}

// Verify username and password input from client
func (p *AuthRequest) VerifyLocal(pool *pgxpool.Pool) error {
	var expectedPassword string

	// get hashed password in DB
	err := pool.QueryRow(context.Background(), `SELECT id, password FROM "User" WHERE username = $1`, p.UserName).Scan(&p.ID, &expectedPassword)
	if err != nil {
		return err
	}

	// compare
	if err = bcrypt.CompareHashAndPassword([]byte(expectedPassword), []byte(p.Password)); err != nil {
		return errors.New("invalid password")
	}

	return nil
}

// Signup with username and password
func (p *AuthRequest) LocalSignup(pool *pgxpool.Pool) error {
	pw, err := bcrypt.GenerateFromPassword([]byte(p.Password), customUtil.HASH_COST)
	if err != nil {
		return err
	}

	// Retrieve auto-generated db ID
	if err = pool.QueryRow(context.Background(), `INSERT INTO "User" ("username", "password") VALUES ($1, $2) RETURNING "id"`, p.UserName, pw).Scan(&p.ID); err != nil {
		return err
	}

	return nil
}

// Verifies state sent by oauth server.
//
// Client server should set a cookie with the same state value before redirecting user to oauth consent uri.
func VerifyState(r *http.Request, cookieName string) error {
	state := r.FormValue("state")

	cookie, err := r.Cookie(cookieName)
	if err != nil {
		return err
	}

	if cookie.Value != state {
		return errors.New("invalid cookie")
	}

	return nil
}
