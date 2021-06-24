.PHONY: client server test

client:
	cd client && CATTLE_PROMETHEUS_METRICS=true go run main.go
server:
	cd server && CATTLE_PROMETHEUS_METRICS=true go run main.go
test:
	curl http://localhost:8123/client/foo/http/localhost:2112/metrics