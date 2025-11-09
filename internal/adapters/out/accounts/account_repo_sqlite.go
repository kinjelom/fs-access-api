package accounts

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"fs-access-api/internal/app/config"
	"fs-access-api/internal/app/ports"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// Ensure compile-time conformance
var _ ports.AccountRepository = (*SQLiteAccountRepository)(nil)

// SQLiteAccountRepository is a SQLite backed implementation (WAL mode).
type SQLiteAccountRepository struct {
	cfg          config.AccountRepositorySqliteConfig
	common       config.AccountRepositoryCommonConfig
	bootstrap    bool
	db           *sql.DB
	queryTimeout time.Duration
	writeTimeout time.Duration
}

// NewSQLiteAccountRepository opens (and initializes) SQLite database file.
func NewSQLiteAccountRepository(cfg config.AccountRepositorySqliteConfig, common config.AccountRepositoryCommonConfig, bootstrap bool) (*SQLiteAccountRepository, error) {

	if bootstrap && cfg.CreateDbDir {
		dir := filepath.Dir(cfg.DbFilePath)
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return nil, fmt.Errorf("cannot create sqlite dir %s: %w", dir, err)
		}
	}
	writersWait := fmt.Sprintf("%d", cfg.WriteTimeout.Milliseconds())
	dsn := cfg.DbFilePath +
		"?_pragma=journal_mode(WAL)" + // many readers, one writer
		"&_pragma=synchronous(NORMAL)" + // good durability/perf balance
		"&_pragma=busy_timeout(" + writersWait + ")" + // writers wait instead of erroring
		"&_pragma=foreign_keys(ON)" // enforce referential integrity

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("sqlite open: %w", err)
	}

	repo := &SQLiteAccountRepository{
		cfg:          cfg,
		common:       common,
		db:           db,
		queryTimeout: cfg.QueryTimeout,
		writeTimeout: cfg.WriteTimeout,
	}

	if bootstrap {
		// Create the schema if not exists.
		if err := repo.initSchema(); err != nil {
			_ = db.Close()
			return nil, err
		}
	}

	// Health check
	if err := repo.HealthCheck(); err != nil {
		_ = db.Close()
		return nil, err
	}

	return repo, nil
}

func (s *SQLiteAccountRepository) initSchema() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	stmts := []string{
		`CREATE TABLE IF NOT EXISTS group_info (
			groupname   TEXT PRIMARY KEY,
			gid         INTEGER NOT NULL CHECK (gid BETWEEN 0 AND 4294967295),
			description TEXT,
			home        TEXT NOT NULL
		);`,

		`CREATE TABLE IF NOT EXISTS user_info (
			username    TEXT PRIMARY KEY,
			uid         INTEGER, -- optional; when NULL assign programmatically
			groupname   TEXT NOT NULL,
			password    TEXT NOT NULL,
			description TEXT,
			home        TEXT NOT NULL,
			expiration  TEXT,    -- RFC3339 or NULL
			disabled    INTEGER NOT NULL DEFAULT 0 CHECK (disabled IN (0,1)),
			FOREIGN KEY (groupname)
				REFERENCES group_info(groupname)
				ON UPDATE CASCADE ON DELETE RESTRICT,
			CHECK (uid IS NULL OR (uid BETWEEN 0 AND 4294967295))
		);`,

		`CREATE UNIQUE INDEX IF NOT EXISTS idx_user_info_uid ON user_info(uid);`,
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	for _, q := range stmts {
		if _, err := tx.ExecContext(ctx, q); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

func (s *SQLiteAccountRepository) HealthCheck() error {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	return s.db.PingContext(ctx)
}

func (s *SQLiteAccountRepository) GetInfo() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), s.queryTimeout)
	defer cancel()

	const q = `SELECT sqlite_version(), datetime('now')`
	row := s.db.QueryRowContext(ctx, q)
	var ver, now string
	if err := row.Scan(&ver, &now); err != nil {
		return "", err
	}
	return fmt.Sprintf("Connected to SQLite (%s) version: '%s', database time: '%s'", s.cfg.DbFilePath, ver, now), nil
}

// -------- Groups --------

func (s *SQLiteAccountRepository) ListGroups() ([]ports.GroupInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), s.queryTimeout)
	defer cancel()

	const q = `SELECT groupname, gid, description, home FROM group_info ORDER BY groupname;`
	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ports.GroupInfo
	for rows.Next() {
		u, err := scanGroupInfo(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func (s *SQLiteAccountRepository) GetGroup(name string) (ports.GroupInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), s.queryTimeout)
	defer cancel()

	const q = `SELECT groupname, gid, description, home FROM group_info WHERE groupname = ?;`
	row := s.db.QueryRowContext(ctx, q, name)
	u, err := scanGroupInfo(row.Scan)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ports.GroupInfo{}, ports.ErrNotFound
		}
		return ports.GroupInfo{}, err
	}
	return u, nil
}

func (s *SQLiteAccountRepository) AddGroup(group ports.GroupInfo) (ports.GroupInfo, error) {
	if strings.TrimSpace(group.Groupname) == "" {
		return ports.GroupInfo{}, errors.New("group name is required")
	}
	if group.GID < s.common.MinGID {
		return ports.GroupInfo{}, fmt.Errorf("group GID is lower than %d", s.common.MinGID)
	}

	ctx, cancel := context.WithTimeout(context.Background(), s.queryTimeout)
	defer cancel()

	const q = `INSERT INTO group_info (groupname, gid, description, home) VALUES (?, ?, ?, ?);`
	_, err := s.db.ExecContext(ctx, q, group.Groupname, group.GID, group.Description, group.Home)
	if err != nil {
		if isDuplicateSQLite(err) {
			return ports.GroupInfo{}, ports.ErrAlreadyExists
		}
		return ports.GroupInfo{}, err
	}
	return s.GetGroup(group.Groupname)
}

func (s *SQLiteAccountRepository) UpdateGroup(group ports.GroupInfo) (ports.GroupInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), s.queryTimeout)
	defer cancel()

	const q = `UPDATE group_info SET gid = ?, description = ?, home = ? WHERE groupname = ?;`
	res, err := s.db.ExecContext(ctx, q, group.GID, group.Description, group.Home, group.Groupname)
	if err != nil {
		return ports.GroupInfo{}, err
	}
	aff, _ := res.RowsAffected()
	if aff == 0 {
		return ports.GroupInfo{}, ports.ErrNotFound
	}
	return s.GetGroup(group.Groupname)
}

func (s *SQLiteAccountRepository) DeleteGroup(name string) error {
	ctx, cancel := context.WithTimeout(context.Background(), s.queryTimeout)
	defer cancel()

	const q = `DELETE FROM group_info WHERE groupname = ?;`
	res, err := s.db.ExecContext(ctx, q, name)
	if err != nil {
		return err
	}
	aff, _ := res.RowsAffected()
	if aff == 0 {
		return ports.ErrNotFound
	}
	return nil
}

// -------- Users --------

func (s *SQLiteAccountRepository) ListUsers() ([]ports.UserInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), s.queryTimeout)
	defer cancel()

	const q = `SELECT username, uid, groupname, password, description, home, expiration, disabled FROM user_info ORDER BY username;`
	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ports.UserInfo
	for rows.Next() {
		u, err := scanUserInfo(rows.Scan, SQLDialectSQLite)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func (s *SQLiteAccountRepository) GetUser(name string) (ports.UserInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), s.queryTimeout)
	defer cancel()

	const q = `SELECT username, uid, groupname, password, description, home, expiration, disabled FROM user_info WHERE username = ?;`
	row := s.db.QueryRowContext(ctx, q, name)
	u, err := scanUserInfo(row.Scan, SQLDialectSQLite)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ports.UserInfo{}, ports.ErrNotFound
		}
		return ports.UserInfo{}, err
	}
	return u, nil
}

func (s *SQLiteAccountRepository) GetNextUID() (uint32, error) {
	return getUserNextUID(s.db, s.queryTimeout, s.common.MinUID)
}

func (s *SQLiteAccountRepository) AddUser(user ports.UserInfo) (ports.UserInfo, error) {
	if strings.TrimSpace(user.Username) == "" {
		return ports.UserInfo{}, errors.New("user name is required")
	}
	if user.Password == "" {
		return ports.UserInfo{}, errors.New("password is required")
	}
	if user.UID < s.common.MinUID {
		return ports.UserInfo{}, fmt.Errorf("user UID is lower than %d", s.common.MinGID)
	}

	ctx, cancel := context.WithTimeout(context.Background(), s.queryTimeout)
	defer cancel()

	const q = `INSERT INTO user_info (username, uid, groupname, password, description, home, expiration, disabled) VALUES (?, ?, ?, ?, ?, ?, ?, ?);`
	_, err := s.db.ExecContext(ctx, q,
		user.Username, user.UID, user.Groupname, user.Password,
		stringOrNil(user.Description), user.Home, timeToTimeStringOrNil(user.Expiration), boolToInt(user.Disabled),
	)
	if err != nil {
		if isDuplicateSQLite(err) {
			return ports.UserInfo{}, ports.ErrAlreadyExists
		}
		// Could also be FK violation if group does not exist.
		return ports.UserInfo{}, err
	}
	return s.GetUser(user.Username)
}

func (s *SQLiteAccountRepository) UpdateUser(user ports.UserInfo) (ports.UserInfo, error) {
	_, err := s.GetUser(user.Username)
	if err != nil {
		return ports.UserInfo{}, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), s.queryTimeout)
	defer cancel()

	const q = `UPDATE user_info
	           SET uid = ?, groupname = ?,  password = ?, description = ?, home = ?, expiration = ?, disabled = ?
	           WHERE username = ?;`
	_, err = s.db.ExecContext(ctx, q,
		user.UID, user.Groupname, user.Password,
		stringOrNil(user.Description), user.Home, timeToTimeStringOrNil(user.Expiration), boolToInt(user.Disabled),
		user.Username,
	)
	if err != nil {
		return ports.UserInfo{}, err
	}
	return s.GetUser(user.Username)
}

func (s *SQLiteAccountRepository) DeleteUser(name string) error {
	ctx, cancel := context.WithTimeout(context.Background(), s.queryTimeout)
	defer cancel()

	const q = `DELETE FROM user_info WHERE username = ?;`
	res, err := s.db.ExecContext(ctx, q, name)
	if err != nil {
		return err
	}
	aff, _ := res.RowsAffected()
	if aff == 0 {
		return ports.ErrNotFound
	}
	return nil
}

func (s *SQLiteAccountRepository) GetUserAuthzInfo(username string) (ports.UserAuthzInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), s.queryTimeout)
	defer cancel()

	const q = `SELECT u.uid, u.groupname, g.gid,  u.password, u.home AS user_home, g.home AS group_home, u.expiration, u.disabled
		FROM user_info AS u
		JOIN group_info AS g ON g.groupname = u.groupname
		WHERE u.username = ?;`
	row := s.db.QueryRowContext(ctx, q, username)

	res := ports.UserAuthzInfo{
		Username: username,
	}
	var (
		expiration sql.NullString
		disabled   int
	)
	if err := row.Scan(&res.UID, &res.Groupname, &res.GID, &res.Password, &res.UserHome, &res.GroupHome, &expiration, &disabled); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ports.UserAuthzInfo{}, ports.ErrNotFound
		}
		return ports.UserAuthzInfo{}, err
	}
	res.Locked = ports.IsUserLocked(disabled != 0, nullTimeStringToPtr(expiration))
	return res, nil
}
