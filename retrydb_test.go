package retrydb

import (
	"database/sql"
	"errors"
	"testing"

	"github.com/lib/pq"
)

func TestDriverRegistered(t *testing.T) {
	// El init() debe haber registrado "pq-retry". sql.Open no conecta aún, solo
	// valida que el driver exista.
	if _, err := sql.Open("pq-retry", "postgres://user:pass@localhost:5432/db?sslmode=disable"); err != nil {
		t.Fatalf("sql.Open(pq-retry) falló: %v", err)
	}
	found := false
	for _, d := range sql.Drivers() {
		if d == "pq-retry" {
			found = true
		}
	}
	if !found {
		t.Fatal("driver pq-retry no está registrado")
	}
}

func TestIsRetryable_PgBouncerCodes(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"26000 prepared stmt does not exist", &pq.Error{Code: "26000"}, true},
		{"08P01 protocol violation", &pq.Error{Code: "08P01"}, true},
		{"otro código pq (23505 unique)", &pq.Error{Code: "23505"}, false},
		{"mensaje prepared statement does not exist", errors.New(`pq: prepared statement "s1" does not exist`), true},
		{"mensaje prepared statement requires", errors.New(`bind message supplies ... prepared statement requires`), true},
		{"error genérico", errors.New("connection refused"), false},
	}
	for _, c := range cases {
		if got := isRetryable(c.err); got != c.want {
			t.Errorf("%s: isRetryable = %v, want %v", c.name, got, c.want)
		}
	}
}
