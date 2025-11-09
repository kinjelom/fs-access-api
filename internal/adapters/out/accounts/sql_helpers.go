package accounts

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"errors"
	"fmt"
	"fs-access-api/internal/app/ports"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
)

func stringOrNil(s *string) any {
	if s == nil {
		return nil
	}
	if strings.TrimSpace(*s) == "" {
		return nil
	}
	return *s
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func nullStringToPtr(ns sql.NullString) *string {
	if ns.Valid {
		return &ns.String
	}
	return nil
}

func nullTimeToPtr(nt sql.NullTime) *time.Time {
	if nt.Valid {
		return &nt.Time
	}
	return nil
}

func nullTimeStringToPtr(ts sql.NullString) *time.Time {
	var expPtr *time.Time
	if ts.Valid && ts.String != "" {
		if t, err := time.Parse(time.RFC3339, strings.TrimSpace(ts.String)); err == nil {
			expPtr = &t
		}
	}
	return expPtr
}

func timeToTimeStringOrNil(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.UTC().Format(time.RFC3339)
}

// pingWithTimeout verifies the DB is reachable.
func pingWithTimeout(db *sql.DB, d time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), d)
	defer cancel()
	return db.PingContext(ctx)
}

func getUserNextUID(db *sql.DB, timeout time.Duration, minValue uint32) (uint32, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	const q = `SELECT COALESCE(MAX(uid) + 1, ?) FROM user_info;`
	var next sql.NullInt64
	if err := db.QueryRowContext(ctx, q, minValue).Scan(&next); err != nil {
		return 0, err
	}
	if !next.Valid || next.Int64 < 0 {
		return minValue, nil
	}
	return uint32(next.Int64), nil
}

// scanGroupInfo maps a single row into the model.GroupInfo.
func scanGroupInfo(scan func(dest ...any) error) (ports.GroupInfo, error) {
	res := ports.GroupInfo{}
	var (
		description sql.NullString
	)
	if err := scan(&res.Groupname, &res.GID, &description, &res.Home); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ports.GroupInfo{}, ports.ErrNotFound
		}
		return ports.GroupInfo{}, err
	}
	res.Description = nullStringToPtr(description)
	return res, nil
}

type SQLDialect string

const (
	SQLDialectMySQL  SQLDialect = "mysql"
	SQLDialectSQLite SQLDialect = "sqlite"
)

// scanUserInfo maps a single row into the model.UserInfo for different DB types.
func scanUserInfo(scan func(dest ...any) error, dialect SQLDialect) (ports.UserInfo, error) {
	res := ports.UserInfo{}
	var (
		description sql.NullString
		expiration  any
		disabled    int
	)

	if dialect == SQLDialectMySQL {
		expiration = new(sql.NullTime)
	} else {
		expiration = new(sql.NullString)
	}

	if err := scan(&res.Username, &res.UID, &res.Groupname, &res.Password, &description, &res.Home, expiration, &disabled); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return res, ports.ErrNotFound
		}
		return res, err
	}
	res.Description = nullStringToPtr(description)

	if dialect == SQLDialectMySQL {
		res.Expiration = nullTimeToPtr(*expiration.(*sql.NullTime))
	} else {
		res.Expiration = nullTimeStringToPtr(*expiration.(*sql.NullString))
	}
	res.Disabled = disabled != 0
	res.PasswordIsHash = true
	return res, nil
}

func isDuplicateSQLite(err error) bool {
	if err == nil {
		return false
	}
	// modernc.org/sqlite returns messages like: "UNIQUE constraint failed: user_info.username"
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unique constraint failed")
}

func isDuplicateMySQL(err error) bool {
	if err == nil {
		return false
	}
	// go-sql-driver/mysql exposes a driver-specific error type with Number = 1062
	type causer interface{ Unwrap() error }
	for {
		switch e := err.(type) {
		case *mysql.MySQLError:
			return e.Number == 1062
		case causer:
			err = e.Unwrap()
		default:
			return false
		}
	}
}

// registerMySQLTLSFromCA registers a custom TLS config using a CA file or directory (PEM).
// Returns the registered TLS profile name to be used via `tls=<name>` in DSN.
func registerMySQLTLSFromCA(caPath string) (string, error) {
	certPool := x509.NewCertPool()

	fi, err := os.Stat(caPath)
	if err != nil {
		return "", fmt.Errorf("stat CA path: %w", err)
	}

	loadFile := func(p string) error {
		pemBytes, err := os.ReadFile(p)
		if err != nil {
			return fmt.Errorf("read CA file %s: %w", p, err)
		}
		if ok := certPool.AppendCertsFromPEM(pemBytes); !ok {
			return fmt.Errorf("no valid certs in %s", p)
		}
		return nil
	}

	if fi.IsDir() {
		entries, err := os.ReadDir(caPath)
		if err != nil {
			return "", fmt.Errorf("read CA dir: %w", err)
		}
		found := false
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			// common PEM file extensions
			if strings.HasSuffix(name, ".pem") || strings.HasSuffix(name, ".crt") || strings.HasSuffix(name, ".cert") {
				if err := loadFile(filepath.Join(caPath, name)); err != nil {
					return "", err
				}
				found = true
			}
		}
		if !found {
			return "", fmt.Errorf("no PEM files found in %s", caPath)
		}
	} else {
		if err := loadFile(caPath); err != nil {
			return "", err
		}
	}

	cfg := &tls.Config{
		RootCAs:            certPool,
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: false,
	}
	const tlsName = "proftpd-mysql-custom"
	// Import is via github.com/go-sql-driver/mysql, so safe to reference here.
	mysqlRegisterTLSConfig(tlsName, cfg)
	return tlsName, nil
}

// Wrapped to make testing easier; go-sql-driver/mysql's RegisterTLSConfig is package-level.
func mysqlRegisterTLSConfig(name string, cfg *tls.Config) {
	err := mysql.RegisterTLSConfig(name, cfg)
	if err != nil {
		panic(">>> mysqlRegisterTLSConfig: " + err.Error())
	}
}
