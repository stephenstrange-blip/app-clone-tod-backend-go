package utils

import (
	"strings"
	"time"
)

const (
	STEP_LENGTH              = 4
	ABC                      = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	HASH_COST                = 10
	COOKIE_NAME              = "token"
	GITHUB_OAUTH_COOKIE_NAME = "github_cookie"
	GOOGLE_OAUTH_COOKIE_NAME = "google_cookie"
	GITHUB_USER_ENDPOINT     = "https://api.github.com/user"
	GOOGLE_USER_ENDPOINT     = "https://www.googleapis.com/oauth2/v3/userinfo"
	GITHUB_REDIRECT_URI      = "/auth/github/callback/"
	GOOGLE_REDIRECT_URI      = "/auth/google/callback/"
	HTTP_TIMEOUT             = time.Second * 5
)

// Repeat "0" 4 times
var MinSegment = strings.Repeat(string(ABC[0]), STEP_LENGTH)

// Repeat "Z" 4 times
var MaxSegment = strings.Repeat(string(ABC[len(ABC)-1]), STEP_LENGTH)
