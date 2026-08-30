# go-retrydb — driver `pq-retry`

Driver `database/sql` que envuelve a `lib/pq` y reintenta de forma transparente
el error transitorio de prepared statement de PgBouncer (SQLSTATE `26000` /
`08P01`) en modo *transaction pooling*.

Es un **módulo Go remoto público**: `github.com/Dabvit/go-retrydb`. Los
microservicios lo consumen como dependencia normal (no hay copias por-servicio).

## Uso en un servicio

`go.mod`:

    require github.com/Dabvit/go-retrydb v1.0.0

En `main.go` (o donde se abra la DB):

```go
import _ "github.com/Dabvit/go-retrydb" // blank import: registra el driver en init()

db, err := sql.Open("pq-retry", dsn)    // en vez de "postgres"
```

Como es un repo **público**, `go mod download` lo descarga en el `docker build`
sin credenciales.

## Cómo modificar el driver

1. Editar `retrydb.go` y sus tests aquí.
2. Commit + push a `github.com/Dabvit/go-retrydb`.
3. Publicar un nuevo tag semver (p.ej. `git tag v1.1.0 && git push origin v1.1.0`).
4. En cada servicio que lo use: `go get github.com/Dabvit/go-retrydb@v1.1.0`.

## Verificación

`orquestaDeDIC/scripts/check_retrydb.sh` confirma que NINGÚN servicio haya
vuelto a introducir una copia local `internal/retrydb/` y que los servicios que
usan `pq-retry` lo declaren en su `go.mod` (útil para CI).
