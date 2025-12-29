module github.com/app-clone-tod-auth

go 1.25.1

require (
	github.com/app-clone-tod-utils v0.0.0-00010101000000-000000000000 // local package
	github.com/golang-jwt/jwt/v5 v5.3.0
	github.com/jackc/pgx/v5 v5.7.6
	golang.org/x/oauth2 v0.33.0
)

require (
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	golang.org/x/crypto v0.37.0
	golang.org/x/sync v0.13.0 // indirect
	golang.org/x/text v0.24.0 // indirect
)

require cloud.google.com/go/compute/metadata v0.3.0 // indirect

replace github.com/app-clone-tod-utils => ../../utils/
