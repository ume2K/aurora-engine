APP_NAME=gocore
PORT=8080

css-dev:
	@echo "Compiling SCSS (Dev)..."
	sass assets/scss/main.scss public/css/main.css

watch-css:
	@echo "Watching SCSS..."
	sass --watch assets/scss/main.scss:public/css/main.css

css-prod:
	@echo "Compiling SCSS (Production)..."
	sass --no-source-map --style=compressed assets/scss/main.scss public/css/main.css

run: css-dev
	@echo "Running locally..."
	go run cmd/server/main.go

dev:
	@echo "Starting Hot-Reload Server..."
	set APP_ENV=development && air

build: css-dev
	@echo "Building binary..."
	if not exist bin mkdir bin
	go build -o bin/server cmd/server/main.go

docker-build: css-prod
	@echo "Building Docker image..."
	docker build -t $(APP_NAME) .

docker-run:
	@echo "Running Container on port $(PORT)..."
	docker run --rm -p $(PORT):$(PORT) -e APP_ENV=production --env-file .env $(APP_NAME)

test-video:
	@if not exist scripts\test.mp4 ( \
		echo Generating test video... && \
		docker run --rm -v "$(CURDIR)/scripts:/out" mwader/static-ffmpeg:7.1 -y -f lavfi -i "color=blue:size=640x360:rate=25:duration=5" -c:v libx264 -preset ultrafast -loglevel error /out/test.mp4 && \
		echo Test video created. \
	)

failover-demo: test-video
	@go run scripts/failover-demo.go

loadtest:
	@echo "Running load test..."
	@where k6 >nul 2>&1 || (echo [ERROR] k6 not found. Install: winget install GrafanaLabs.k6 && exit /b 1)
	k6 run scripts/loadtest.js

clean:
	@echo "Cleaning up..."
	if exist bin rmdir /s /q bin
	docker rmi $(APP_NAME)
