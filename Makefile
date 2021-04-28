all: npm_dependencies go_dependencies bundle

npm_dependencies:
	cd web; \
	npm i --no-dev;

go_dependencies:
	go get ./...

bundle:
	go build app/server/main.go

clean:
	rm -fr web/node_modules