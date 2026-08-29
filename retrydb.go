// Package retrydb registra un driver database/sql llamado "pq-retry" que
// envuelve al driver de lib/pq y REINTENTA de forma transparente las
// operaciones que fallan con el error transitorio de prepared statement
// (SQLSTATE 26000 "prepared statement does not exist" / 08P01) que ocurre con
// lib/pq detrás de PgBouncer en modo transaction pooling.
//
// El error solo afecta al statement (protocolo), NO a los datos: PgBouncer
// reasignó la conexión de servidor entre Parse y Bind, así que el statement
// preparado ya no existe en esa conexión. Reintentar la MISMA operación con una
// nueva ejecución es seguro e idempotente (son sentencias sueltas, no dentro de
// una transacción del cliente).
//
// Uso: en lugar de sql.Open("postgres", dsn) → sql.Open("pq-retry", dsn).
// No requiere cambiar ninguna query ni la lógica de repositorio.
//
// ============================================================================
// FUENTE ÚNICA DE VERDAD: este archivo vive en el módulo canónico
// github.com/Dabvit/go-retrydb y se propaga a los 20 servicios Go mediante
// orquestaDeDIC/scripts/sync_retrydb.sh. NO editar las copias por-servicio a
// mano: editar aquí y correr el script.
// ============================================================================
package retrydb

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"io"
	"strings"

	"github.com/lib/pq"
)

func init() {
	sql.Register("pq-retry", &retryDriver{base: pq.Driver{}})
}

// isRetryable detecta el error transitorio de prepared statement de PgBouncer.
func isRetryable(err error) bool {
	if err == nil {
		return false
	}
	if pqErr, ok := err.(*pq.Error); ok {
		// 26000: prepared statement does not exist
		// 08P01: protocol violation (bind message ... prepared statement ...)
		if pqErr.Code == "26000" || pqErr.Code == "08P01" {
			return true
		}
	}
	msg := err.Error()
	return strings.Contains(msg, "prepared statement") &&
		(strings.Contains(msg, "does not exist") || strings.Contains(msg, "requires"))
}

type retryDriver struct{ base pq.Driver }

func (d *retryDriver) Open(name string) (driver.Conn, error) {
	c, err := d.base.Open(name)
	if err != nil {
		return nil, err
	}
	return &retryConn{Conn: c, dsn: name}, nil
}

// retryConn envuelve la conexión de lib/pq. Implementa las interfaces de
// contexto (Queryer/Execer) para interceptar y reintentar.
type retryConn struct {
	driver.Conn
	dsn string
}

func (c *retryConn) reopen() error {
	// Cierra la conexión rota y abre una nueva del mismo driver.
	_ = c.Conn.Close()
	nc, err := (pq.Driver{}).Open(c.dsn)
	if err != nil {
		return err
	}
	c.Conn = nc
	return nil
}

func (c *retryConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	q, ok := c.Conn.(driver.QueryerContext)
	if !ok {
		return nil, driver.ErrSkip
	}
	rows, err := q.QueryContext(ctx, query, args)
	if isRetryable(err) {
		if rerr := c.reopen(); rerr == nil {
			if q2, ok := c.Conn.(driver.QueryerContext); ok {
				return q2.QueryContext(ctx, query, args)
			}
		}
	}
	return rows, err
}

func (c *retryConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	e, ok := c.Conn.(driver.ExecerContext)
	if !ok {
		return nil, driver.ErrSkip
	}
	res, err := e.ExecContext(ctx, query, args)
	if isRetryable(err) {
		if rerr := c.reopen(); rerr == nil {
			if e2, ok := c.Conn.(driver.ExecerContext); ok {
				return e2.ExecContext(ctx, query, args)
			}
		}
	}
	return res, err
}

// Prepare/PrepareContext: delega. Los statements preparados explícitamente
// (db.Prepare) son poco usados en estos servicios; el error se da sobre todo en
// el statement SIN nombre de Query/Exec, ya cubierto arriba.
func (c *retryConn) Prepare(query string) (driver.Stmt, error) {
	return c.Conn.Prepare(query)
}

// Aseguramos passthrough de las capacidades opcionales del Conn subyacente.
func (c *retryConn) Begin() (driver.Tx, error) { return c.Conn.Begin() }

func (c *retryConn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	if b, ok := c.Conn.(driver.ConnBeginTx); ok {
		return b.BeginTx(ctx, opts)
	}
	return c.Conn.Begin()
}

func (c *retryConn) PrepareContext(ctx context.Context, query string) (driver.Stmt, error) {
	if p, ok := c.Conn.(driver.ConnPrepareContext); ok {
		return p.PrepareContext(ctx, query)
	}
	return c.Conn.Prepare(query)
}

func (c *retryConn) Ping(ctx context.Context) error {
	if p, ok := c.Conn.(driver.Pinger); ok {
		return p.Ping(ctx)
	}
	return nil
}

func (c *retryConn) Close() error { return c.Conn.Close() }

// compile-time checks
var (
	_ driver.QueryerContext     = (*retryConn)(nil)
	_ driver.ExecerContext      = (*retryConn)(nil)
	_ driver.ConnBeginTx        = (*retryConn)(nil)
	_ driver.ConnPrepareContext = (*retryConn)(nil)
	_ driver.Pinger             = (*retryConn)(nil)
	_ io.Closer                 = (*retryConn)(nil)
)
