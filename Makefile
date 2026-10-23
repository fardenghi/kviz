# kviz: compilar, correr y probar. Correr desde este directorio (o con make -C kviz ...).

.PHONY: run build ui test test-go test-web

# Compila la interfaz y el binario, y arranca kviz (abre el navegador).
run: build
	./bin/kviz

# Binario único con la interfaz incluida: bin/kviz.
build: ui
	go build -o bin/kviz ./cmd/kviz

ui: web/node_modules
	npm --prefix web run build

web/node_modules: web/package.json web/package-lock.json
	npm --prefix web ci --no-audit --no-fund --loglevel=error
	@touch web/node_modules

test: test-go test-web

test-go:
	go test ./...

test-web: web/node_modules
	npm --prefix web test
