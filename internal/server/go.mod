module github.com/app-clone-tod

go 1.25.1

replace github.com/app-clone-tod-auth => ../controllers/auth/

require (
	github.com/app-clone-tod-auth v0.0.0-00010101000000-000000000000
	github.com/app-clone-tod-controllers v0.0.0-00010101000000-000000000000
	github.com/app-clone-tod-db v0.0.0-00010101000000-000000000000
	github.com/jackc/pgx/v5 v5.7.6
	github.com/joho/godotenv v1.5.1
)

require (
	cloud.google.com/go/compute/metadata v0.3.0 // indirect
	github.com/app-clone-tod-utils v0.0.0-00010101000000-000000000000 // indirect
	github.com/golang-jwt/jwt/v5 v5.3.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	golang.org/x/crypto v0.37.0 // indirect
	golang.org/x/oauth2 v0.33.0 // indirect
	golang.org/x/sync v0.13.0 // indirect
	golang.org/x/text v0.24.0 // indirect
)

replace github.com/app-clone-tod-controllers => ../controllers/

replace github.com/app-clone-tod-utils => ../utils/

replace github.com/app-clone-tod-db => ../db/
