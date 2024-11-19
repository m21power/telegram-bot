sdocker:
	sudo sysctl -w kernel.apparmor_restrict_unprivileged_userns=0
startdocker: sdocker
	systemctl --user start docker-desktop
# staring from this till v4.17.0 it is docker command, after that it is to create migrate file u can see on their documentation
startmigration:
	docker run -it --rm --network host --volume "$(shell pwd)/db:/db" migrate/migrate:v4.17.0 create -ext sql -dir /db/migrations -seq followers
# this is to create mysql container if doen't exist else it will start the container
rundocker:
	docker run --name referrals_db -p 3306:3306 -e MYSQL_ROOT_PASSWORD=abate -d mysql:9.1
# create database in docker container #simple-api should be the name of the container we created above
createdb:
	docker exec -it referrals_db mysql -uroot -pabate -e "CREATE DATABASE referrals_db;"

# migrate up
migrateup:
	docker run -it --rm --network host --volume "$(shell pwd)/db:/db" migrate/migrate:v4.17.0 -path /db/migrations -database "mysql://root:abate@tcp(localhost:3306)/gringram" up
migratedown:
	docker run -it --rm --network host --volume "$(shell pwd)/db:/db" migrate/migrate:v4.17.0 -path /db/migrations -database "mysql://root:abate@tcp(localhost:3306)/gringram" down