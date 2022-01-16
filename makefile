all:
	@mkdir -p bin
	go build -o bin/fleet_state_server fleet_state_server/*.go
	go build -o bin/vehicle_simulator vehicle_simulator/*.go
	go build -o bin/client client/*.go

fmt:
	go fmt fleet_state_server/*.go
	go fmt vehicle_simulator/*.go
	go fmt client/*.go
