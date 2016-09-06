.PHONY: build clean

build: bin/lambda-resource-linux-amd64
	ln -s lambda-resource-linux-amd64 bin/in || true
	ln -s lambda-resource-linux-amd64 bin/out || true
	ln -s lambda-resource-linux-amd64 bin/check || true


bin/lambda-resource-linux-amd64:
	CGO_ENABLED=0 $GOOS=linux $GOARCH=amd64 go build -o bin/lambda-resource-linux-amd64

image: clean build
	docker build -t hdsydsvenskan/lambda-resource:latest .

clean:
	rm bin/* || true
