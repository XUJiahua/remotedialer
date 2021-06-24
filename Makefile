.PHONY: client server

client:
	cd client && CATTLE_PROMETHEUS_METRICS=true go run main.go
server:
	cd server && CATTLE_PROMETHEUS_METRICS=true go run main.go
