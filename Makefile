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

install:
	echo "hash=$(git log -n 1 --no-merges --pretty=format:%h)" > hash.env; \
	mv main /bin/tum-live