# go-retrydb — driver canónico `pq-retry`

**Fuente única de verdad** del driver `database/sql` `pq-retry` que envuelve a
`lib/pq` y reintenta de forma transparente el error transitorio de prepared
statement de PgBouncer (SQLSTATE `26000` / `08P01`) en modo *transaction
pooling*.

## Por qué existe este módulo

Go no comparte código entre módulos sin un módulo remoto o vendoring, y el
contexto de build de Docker de cada microservicio es **su propia carpeta**. Por
eso el driver se mantiene como una fuente canónica aquí y se **copia** a cada
servicio en `<svc>/internal/retrydb/retrydb.go`.

## Cómo modificar el driver

1. Editar **solo** `retrydb.go` de este módulo.
2. Correr el script de sincronización:
   ```bash
   orquestaDeDIC/scripts/sync_retrydb.sh
   ```
3. Verificar (CI): `sync_retrydb.sh --check` termina con exit 1 si alguna copia
   quedó desactualizada (drift).

**No** editar las copias por-servicio a mano.

## Uso en un servicio

```go
import _ "github.com/Dabvit/<svc>/internal/retrydb" // registra el driver en init()

db, err := sql.Open("pq-retry", dsn) // en vez de "postgres"
```

## Migración futura a import remoto

Cuando exista el repo remoto `github.com/Dabvit/go-retrydb`, la migración a un
import real (sin copias) es trivial: publicar este módulo y reemplazar el blank
import por la dependencia. La fuente ya está aislada aquí.
