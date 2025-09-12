## build: builds all binaries
build: clean build_back
	@printf "All binaries built!\n"

## clean: cleans all binaries and runs go clean
clean:
	@echo "Cleaning..."
	@- rm -f dist/*
	@go clean
	@echo "Cleaned!"

## build_back: builds the back end
build_back:
	@echo "Building back end..."
	@go build -o dist/shopit_api ./cmd/api
	@echo "Back end built!"

## start: back end
start: start_back

## start_back: starts the back end
start_back: build_back
	@echo "Starting the back end..."
	./dist/shopit_api &
	@echo "Back end running!"

## stop: stops the back end
stop: stop_back
	@echo "All applications stopped"

## stop_back: stops the back end
stop_back:
	@echo "Stopping the back end..."
	@-pkill -SIGTERM -f "shopit_api"
	@echo "Stopped back end"
