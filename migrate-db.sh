# use this to migrate changes in your local database.
export POSTGRESQL_URL='postgres://user:password@localhost:5432/rbglive?sslmode=disable'
./migrate.linux-amd64 create -ext sql -dir migrations -seq initialize-db
