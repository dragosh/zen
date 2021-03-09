build/amd:
	mkdir -p zen.app/Contents/MacOS
	export CGO_ENABLED=1 && GOARCH=amd64 go build -o target/release/zen.app/Contents/MacOS/zen

build/arm:
	mkdir -p zen.app/Contents/MacOS
	export CGO_ENABLED=1 && GOARCH=arm64 go build -o target/release/zen.app/Contents/MacOS/zen

build:
	go build -o zen -o target/release/zen

