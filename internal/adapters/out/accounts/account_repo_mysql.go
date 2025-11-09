package accounts

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"fs-access-api/internal/app/config"
	"fs-access-api/internal/app/ports"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql" // MySQL driver
)

// MySQLAccountRepository is a MySQL-backed implementation of AccountRepositoryConfig.
type MySQLAccountRepository struct {
	common       config.AccountRepositoryCommonConfig
	bootstrap    bool
	db           *sql.DB
	queryTimeout time.Duration
}

// Enforce compile-time conformance to the interface
var _ ports.AccountRepository = (*MySQLAccountRepository)(nil)

// NewMySQLAccountRepository creates the service and opens a connection pool.
func NewMySQLAccountRepository(cfg config.AccountRepositoryMySqlConfig, common config.AccountRepositoryCommonConfig, bootstrap bool) (*MySQLAccountRepository, error) {
	if cfg.Host == "" || cfg.Port == 0 || cfg.Database == "" || cfg.User == "" {
		return nil, errors.New("invalid MySQL config: host/port/database/user are required")
	}

	tlsName := ""
	dsnExtra := "parseTime=true&charset=utf8mb4,utf8&collation=utf8mb4_unicode_ci"
	if !cfg.IgnoreSSL {
		name, err := registerMySQLTLSFromCA(cfg.SSLCaPath)
		if err != nil {
			return nil, fmt.Errorf("failed to register TLS config: %w", err)
		}
		tlsName = name
		dsnExtra += "&tls=" + tlsName
	}

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?%s",
		cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.Database, dsnExtra)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("sql.Open: %w", err)
	}
	// Sensible pool defaults; adjust for your workload
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(30 * time.Minute)

	repo := &MySQLAccountRepository{
		common:       common,
		bootstrap:    bootstrap,
		db:           db,
		queryTimeout: cfg.QueryTimeout,
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

func (s *MySQLAccountRepository) initSchema() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	stmts := []string{
		`CREATE TABLE IF NOT EXISTS group_info (
			groupname   VARCHAR(128)  NOT NULL,
			gid         INT UNSIGNED  NOT NULL,
			description VARCHAR(255)  NULL,
			home        VARCHAR(1024) NOT NULL,
			PRIMARY KEY (groupname)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`,

		`CREATE TABLE IF NOT EXISTS user_info (
			username    VARCHAR(128)  NOT NULL,
			uid         INT UNSIGNED  NOT NULL,
			groupname   VARCHAR(128)  NOT NULL,
			password    VARCHAR(255)  NOT NULL,
			description VARCHAR(255)  NULL,
			home        VARCHAR(1024) NOT NULL,
			expiration  DATETIME      NULL,
			disabled    TINYINT(1)    NOT NULL DEFAULT 0,
			PRIMARY KEY (username),
			UNIQUE KEY user_info_uid_uq (uid),
			CONSTRAINT user_info_groupname_fk
				FOREIGN KEY (groupname) REFERENCES group_info (groupname)
				ON UPDATE CASCADE ON DELETE RESTRICT
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`,
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

func (s *MySQLAccountRepository) HealthCheck() error {
	if err := pingWithTimeout(s.db, time.Second); err != nil {
		return fmt.Errorf("database unhealthy: %w", err)
	}
	return nil
}

func (s *MySQLAccountRepository) GetInfo() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), s.queryTimeout)
	defer cancel()

	const q = `SELECT version() ver, now() AS now;`
	row := s.db.QueryRowContext(ctx, q)

	var ver, now string
	if err := row.Scan(&ver, &now); err != nil {
		return "", err
	}

	msg := fmt.Sprintf("Connected to MySQL version: '%s', database time: '%s'", ver, now)

	return msg, nil
}

// --- Groups ---

func (s *MySQLAccountRepository) ListGroups() ([]ports.GroupInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), s.queryTimeout)
	defer cancel()

	const q = `SELECT groupname, gid, description, home FROM group_info ORDER BY groupname;`
	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer func(rows *sql.Rows) {
		_ = rows.Close()
	}(rows)

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

func (s *MySQLAccountRepository) GetGroup(name string) (ports.GroupInfo, error) {
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

func (s *MySQLAccountRepository) AddGroup(group ports.GroupInfo) (ports.GroupInfo, error) {
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
		if isDuplicateMySQL(err) {
			return ports.GroupInfo{}, ports.ErrAlreadyExists
		}
		return ports.GroupInfo{}, err
	}
	return s.GetGroup(group.Groupname)
}

func (s *MySQLAccountRepository) UpdateGroup(group ports.GroupInfo) (ports.GroupInfo, error) {
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

func (s *MySQLAccountRepository) DeleteGroup(name string) error {
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

// --- Users ---

func (s *MySQLAccountRepository) ListUsers() ([]ports.UserInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), s.queryTimeout)
	defer cancel()

	const q = `SELECT username, uid, groupname, password, description, home, expiration, disabled FROM user_info ORDER BY groupname`
	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer func(rows *sql.Rows) {
		_ = rows.Close()
	}(rows)

	var out []ports.UserInfo
	for rows.Next() {
		u, err := scanUserInfo(rows.Scan, SQLDialectMySQL)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func (s *MySQLAccountRepository) GetUser(name string) (ports.UserInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), s.queryTimeout)
	defer cancel()

	const q = `SELECT username, uid, groupname, password, description, home, expiration, disabled FROM user_info WHERE username = ?;`
	row := s.db.QueryRowContext(ctx, q, name)
	u, err := scanUserInfo(row.Scan, SQLDialectMySQL)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ports.UserInfo{}, ports.ErrNotFound
		}
		return ports.UserInfo{}, err
	}
	return u, nil
}

func (s *MySQLAccountRepository) GetNextUID() (uint32, error) {
	return getUserNextUID(s.db, s.queryTimeout, s.common.MinUID)
}

func (s *MySQLAccountRepository) AddUser(user ports.UserInfo) (ports.UserInfo, error) {
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
		user.Username, user.UID, user.Groupname, user.Password, user.Description, user.Home, user.Expiration, boolToInt(user.Disabled))
	if err != nil {
		if isDuplicateMySQL(err) {
			return ports.UserInfo{}, ports.ErrAlreadyExists
		}
		return ports.UserInfo{}, err
	}

	// Return what is stored (including normalized fields)
	return s.GetUser(user.Username)
}

func (s *MySQLAccountRepository) UpdateUser(user ports.UserInfo) (ports.UserInfo, error) {
	// Must not change password: we fetch existing hash and keep it.
	existing, err := s.GetUser(user.Username)
	if err != nil {
		return ports.UserInfo{}, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), s.queryTimeout)
	defer cancel()

	const q = `UPDATE user_info SET uid = ?, groupname = ?, password = ?, description = ?, home = ?, expiration = ?, disabled = ? WHERE username = ?;`
	_, err = s.db.ExecContext(ctx, q,
		user.UID, user.Groupname, user.Password, user.Description, user.Home, user.Expiration, boolToInt(user.Disabled), user.Username)
	if err != nil {
		return ports.UserInfo{}, err
	}

	user.Password = existing.Password
	return s.GetUser(user.Username)
}

func (s *MySQLAccountRepository) DeleteUser(name string) error {
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

func (s *MySQLAccountRepository) GetUserAuthzInfo(username string) (ports.UserAuthzInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), s.queryTimeout)
	defer cancel()

	const q = `SELECT u.uid, u.groupname, g.gid,  u.password, u.home AS user_home, g.home AS group_home, u.expiration, u.disabled
		FROM user_info AS u
		JOIN group_info AS g ON g.groupname = u.groupname
		WHERE u.username = ?;`

	res := ports.UserAuthzInfo{}
	row := s.db.QueryRowContext(ctx, q, username)
	var (
		expiration sql.NullTime
		disabled   int
	)

	if err := row.Scan(&res.UID, &res.Groupname, &res.GID, &res.Password, &res.UserHome, &res.GroupHome, &expiration, &disabled); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return res, ports.ErrNotFound
		}
		return res, err
	}
	res.Locked = ports.IsUserLocked(disabled != 0, nullTimeToPtr(expiration))
	return res, nil
}
