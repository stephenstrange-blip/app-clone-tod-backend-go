module github.com/app-clone-tod-test

go 1.25.1

require (
	github.com/app-clone-tod-utils v0.0.0-00010101000000-000000000000
	github.com/joho/godotenv v1.5.1
)

replace github.com/app-clone-tod-auth => ../controllers/auth/

replace github.com/app-clone-tod-controllers => ../controllers/

replace github.com/app-clone-tod-utils => ../utils/

replace github.com/app-clone-tod-db => ../db/
