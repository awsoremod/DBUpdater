## DBUpdater

This is console utility for updating db with help of ordered set of migrations

repository copy https://gitlab.com/darth_sith_lord/dbupdater

go build -o ./cmd/dbupdater ./cmd/dbupdater   
go test -v ./...   

examples:   

init mode:   
./cmd/dbupdater/dbupdater.exe -host=localhost -port=5432 -dbname=db_local -username=developer -password=123 -migrations="./cmd/dbupdater/dir-for-migrations" -versiondb="v0.0.2" -verbose     
error:     
./cmd/dbupdater/dbupdater.exe -host=localhost -port=5432 -dbname=db_local -username=developer -password=123 -migrations="./cmd/dbupdater/dir-for-migrations" -versiondb="v0.0.3" -verbose     
