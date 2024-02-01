## DBUpdater

This is console utility for updating db with help of ordered set of migrations

go build -o ./cmd/dbupdater ./cmd/dbupdater   
go test -v ./...   

examples:   
    
ls:   
init mode:   
./cmd/dbupdater/dbupdater.exe -host=localhost -port=5432 -dbname=lambda_service_local -username=developer -password=123 -migrations="./cmd/dbupdater/dir-for-migrations" -versiondb="v0.0.2" -verbose     
error:     
./cmd/dbupdater/dbupdater.exe -host=localhost -port=5432 -dbname=lambda_service_local -username=developer -password=123 -migrations="./cmd/dbupdater/dir-for-migrations" -versiondb="v0.0.3" -verbose     
   
   
sme:   
./cmd/dbupdater/dbupdater.exe -host=localhost -port=5432 -dbname=state_machine_service_local -username=developer -password=123 -migrations="./cmd/dbupdater/dir-for-migrations" -versiondb="v0.0.2" -verbose    
error:     
./cmd/dbupdater/dbupdater.exe -host=localhost -port=5432 -dbname=state_machine_service_local -username=developer -password=123 -migrations="./cmd/dbupdater/dir-for-migrations" -versiondb="v0.0.3" -verbose    
   
   
sms:    
./cmd/dbupdater/dbupdater.exe -host=localhost -port=5432 -dbname=state_machine_service_int_tests -username=developer -password=123 -migrations="./cmd/dbupdater/dir-for-migrations" -versiondb="v0.0.2" -verbose   
error:   
./cmd/dbupdater/dbupdater.exe -host=localhost -port=5432 -dbname=state_machine_service_int_tests -username=developer -password=123 -migrations="./cmd/dbupdater/dir-for-migrations" -versiondb="v0.0.3" -verbose   
   
   
custom:   
init mode:   
./cmd/dbupdater/dbupdater.exe -host=localhost -port=5432 -dbname=first_for_username_1 -username=username_1 -password=password_1 -migrations="./cmd/dbupdater/dir-for-migrations" -versiondb="v0.0.2" -verbose    
error:
./cmd/dbupdater/dbupdater.exe -host=localhost -port=5432 -dbname=first_for_username_1 -username=username_1 -password=password_1 -migrations="./cmd/dbupdater/dir-for-migrations" -versiondb="v0.0.3" -verbose    
