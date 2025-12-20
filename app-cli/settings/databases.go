package settings

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/errors"
)

type dbPersist struct {
	Application string
	Name        string
	Table       string
	Items       string
	scheme      string
	constr      string
	db          *sql.DB
}

const (
	configType   = "config"
	fileType     = "file"
	sqlite3Type  = "sqlite3"
	sqliteType   = "sqlite"
	postgresType = "postgres"
)

func NewDatabaseConfigService(application, scheme, name string) (dbPersist, error) {
	var (
		err        error
		connection string
	)

	handle := dbPersist{
		Application: application,
		Name:        name,
		Table:       "config_ids",
		Items:       "config_items",
		scheme:      scheme,
		constr:      name,
	}

	if scheme == sqlite3Type || scheme == sqliteType {
		connection = strings.TrimPrefix(name, scheme+"://")
		scheme = sqlite3Type
	} else {
		connection = name
	}

	ui.Log(ui.AppLogger, "settings.db.open", ui.A{
		"connection": connection,
		"scheme":     scheme})

	handle.db, err = sql.Open(scheme, connection)
	if err != nil {
		return handle, err
	}

	tx, _ := handle.db.Begin()
	defer tx.Rollback()

	sql := fmt.Sprintf(`
	create table IF NOT EXISTS %s (
	    id string PRIMARY KEY ,
        description TEXT NOT NULL,
        name TEXT NOT NULL,
        version INTEGER NOT NULL,
		salt TEXT NOT NULL,
		modified TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
		`, strconv.Quote(handle.Table))

	_, err = tx.Exec(sql)
	if err != nil {
		ui.Log(ui.AppLogger, "settings.db.error", ui.A{
			"sql":   sql,
			"table": handle.Items,
			"error": err})

		return handle, err
	}

	ui.Log(ui.AppLogger, "settings.table.created", ui.A{
		"table": handle.Table,
	})

	sql = fmt.Sprintf(`
    create table IF NOT EXISTS %s (
        id TEXT,
        key TEXT NOT NULL,
        value TEXT NOT NULL
	)`,
		strconv.Quote(handle.Items))

	_, err = tx.Exec(sql)
	if err != nil {
		ui.Log(ui.AppLogger, "settings.db.error", ui.A{
			"sql":   sql,
			"table": handle.Items,
			"error": err})

		return handle, err
	}

	ui.Log(ui.AppLogger, "settings.table.created", ui.A{
		"table": handle.Items,
	})

	err = tx.Commit()

	return handle, err
}

func (d dbPersist) Load(application, name string) (*Configuration, error) {
	if d.db == nil {
		return nil, errors.ErrDatabaseClientClosed
	}

	config, err := d.findConfig(name)
	if err != nil {
		if err == sql.ErrNoRows {
			var result sql.Result

			// If no configuration, create a new default one.
			config = Configuration{
				Name:        name,
				Description: name + " configuration",
				ID:          uuid.New().String(),
				Version:     1,
				Salt:        saltString(),
				Dirty:       true,
				Items:       make(map[string]string),
			}

			// Create a configuration entry in the database for this new profile.
			sql := fmt.Sprintf(`INSERT INTO %s (id, description, name, version, salt) VALUES ($1, $2, $3, $4, $5)`, d.Table)

			tx, _ := d.db.Begin()

			result, err = tx.Exec(sql,
				config.ID,
				config.Description,
				config.Name,
				config.Version,
				config.Salt)

			if err != nil {
				ui.Log(ui.AppLogger, "settings.db.error", ui.A{
					"sql":   sql,
					"error": err})

				tx.Rollback()

				return nil, err
			}

			if count, _ := result.RowsAffected(); count == 0 {
				ui.Log(ui.AppLogger, "settings.db.error", ui.A{
					"sql":   sql,
					"error": errors.ErrTableNoRows})

				tx.Rollback()

				return nil, err
			}

			// Copy any items from the configuration defaults (uuid of 0) to the new configuration.
			sql = fmt.Sprintf("SELECT key, value FROM %s WHERE id = '00000000-0000-0000-0000-000000000000'", d.Items)
			rows, err := tx.Query(sql)

			if err != nil {
				ui.Log(ui.AppLogger, "settings.db.error", ui.A{
					"sql":   sql,
					"error": err})

				tx.Rollback()

				return nil, err
			}

			count := 0

			for rows.Next() {
				var key, value string

				err := rows.Scan(&key, &value)
				if err != nil {
					ui.Log(ui.AppLogger, "settings.db.error", ui.A{
						"sql":   sql,
						"error": err})

					tx.Rollback()

					return nil, err
				}

				// Note, no decryption possible here. So if it's an item that must be
				// decrypted, we don't use it as it cannot be verified as valid.
				if _, ok := encryptedKeyValue[key]; ok {
					continue
				}

				config.Items[key] = value
				count++
			}

			ui.Log(ui.AppLogger, "settings.db.default.items.copied", ui.A{
				"count": count,
				"name":  name})

			tx.Commit()

			ui.Log(ui.AppLogger, "settings.db.default", ui.A{
				"application": application,
				"name":        name})

			return &config, nil
		}

		return &config, err
	}

	sql := fmt.Sprintf(`SELECT key, value FROM %s WHERE id = $1 ORDER BY key`, d.Items)

	rows, err := d.db.Query(sql, config.ID)
	if err != nil {
		ui.Log(ui.AppLogger, "settings.db.error", ui.A{
			"sql":   sql,
			"table": d.Items,
			"error": err})

		return nil, err
	}

	defer rows.Close()

	config.Items = make(map[string]string)

	for rows.Next() {
		var key, value string

		err := rows.Scan(&key, &value)
		if err != nil {
			ui.Log(ui.AppLogger, "settings.db.error", ui.A{
				"sql":   sql,
				"table": d.Items,
				"error": err})

			return nil, err
		}

		// Some specific items must be decrypted.
		if _, ok := encryptedKeyValue[key]; ok {
			value, _ = Decrypt(value, config.Salt+internalProfileID)
		}

		config.Items[key] = value
	}

	ui.Log(ui.AppLogger, "settings.db.load", ui.A{
		"application": application,
		"name":        name,
		"count":       len(config.Items)})

	return &config, nil
}

func (d dbPersist) Save(cp *Configuration) error {
	var rows sql.Result

	if d.db == nil {
		return errors.ErrDatabaseClientClosed
	}

	ui.Log(ui.AppLogger, "settings.db.save", ui.A{
		"name":  CurrentConfiguration.Name,
		"count": len(CurrentConfiguration.Items)})

	tx, err := d.db.Begin()
	if err != nil {
		ui.Log(ui.AppLogger, "settings.db.error", ui.A{
			"table": "",
			"error": err})

		return err
	}

	defer tx.Rollback()

	// Update the configuration record with a new timestamp
	sql := fmt.Sprintf(`UPDATE %s SET modified = CURRENT_TIMESTAMP WHERE id = %s`,
		strconv.Quote(d.Table),
		strconv.Quote(cp.ID))

	rows, err = tx.Exec(sql)
	if err != nil {
		ui.Log(ui.AppLogger, "settings.db.error", ui.A{
			"table": d.Table,
			"sql":   sql,
			"error": err})

		return err
	}

	if count, _ := rows.RowsAffected(); count == 0 {
		ui.Log(ui.AppLogger, "settings.db.profile.not.updated", ui.A{
			"name": cp.Name})

		return errors.ErrNoSuchProfile.Context(cp.Name)
	}

	// Delete all existing items for this configuration
	sql = fmt.Sprintf(`DELETE FROM %s WHERE id = %s`,
		strconv.Quote(d.Items),
		strconv.Quote(cp.ID))

	_, err = tx.Exec(sql)
	if err != nil {
		ui.Log(ui.AppLogger, "settings.db.error", ui.A{
			"table": d.Items,
			"sql":   sql,
			"error": err})

		return err
	}

	// Insert the items for this configuration
	for key, value := range cp.Items {
		sql := fmt.Sprintf(`INSERT INTO %s (id, key, value) VALUES ($1, $2, $3)`, strconv.Quote(d.Items))

		// Some specific items must be encrypted.
		if _, ok := encryptedKeyValue[key]; ok {
			value, _ = Encrypt(value, cp.Salt+internalProfileID)
		}

		_, err = tx.Exec(sql, cp.ID, key, value)

		if err != nil {
			ui.Log(ui.AppLogger, "settings.db.error", ui.A{
				"table": d.Items,
				"sql":   sql,
				"error": err})

			return err
		}
	}

	ui.Log(ui.AppLogger, "settings.db.load", ui.A{
		"application": d.Application,
		"name":        cp.Name,
		"count":       len(cp.Items)})

	return tx.Commit()
}

func (d dbPersist) DeleteProfile(name string) error {
	if d.db == nil {
		return errors.ErrDatabaseClientClosed
	}

	if name == CurrentConfiguration.Name {
		return errors.ErrCannotDeleteActiveProfile.Context(name)
	}

	tx, err := d.db.Begin()
	if err != nil {
		return err
	}

	defer tx.Rollback()

	sql := fmt.Sprintf(`DELETE FROM %s WHERE id = %s`,
		strconv.Quote(d.Table),
		strconv.Quote(CurrentConfiguration.ID))

	_, err = tx.Exec(sql)
	if err != nil {
		return err
	}

	sql = fmt.Sprintf(`DELETE FROM %s WHERE id = %s`,
		strconv.Quote(d.Items),
		strconv.Quote(CurrentConfiguration.ID))

	_, err = tx.Exec(sql)
	if err == nil {
		err = tx.Commit()
	}

	return err
}

// For the database, there is no difference between the Load and UseProfile functions. There is no in-memory
// cache like there is for the file-system configuration files.
func (d dbPersist) UseProfile(name string) (*Configuration, error) {
	cp, err := d.Load(d.Application, name)
	if err != nil {
		ui.Log(ui.InternalLogger, "settings.db.error", ui.A{
			"error": err})
	}

	return cp, err
}

func (d dbPersist) findConfig(name string) (Configuration, error) {
	c := Configuration{
		Name: name,
	}

	if d.db == nil {
		return c, errors.ErrDatabaseClientClosed
	}

	sql := fmt.Sprintf(`
    SELECT id, description, version, salt FROM %s WHERE name = %s LIMIT 1`,
		strconv.Quote(d.Table),
		strconv.Quote(name))

	row := d.db.QueryRow(sql)
	err := row.Scan(&c.ID, &c.Description, &c.Version, &c.Salt)

	if err == nil {
		ui.Log(ui.AppLogger, "settings.db.found", ui.A{
			"name": name})
	} else {
		ui.Log(ui.AppLogger, "settings.db.not.found", ui.A{
			"name": name})
	}

	c.Items = make(map[string]string)

	return c, err
}

func (d dbPersist) Close() {
	if d.db != nil {
		_ = d.db.Close()
		d.db = nil

		ui.Log(ui.AppLogger, "settings.db.closed", ui.A{
			"config": d.constr,
		})
	}
}

func saltString() string {
	result := ""
	salt := make([]byte, 16)

	for len(result) < 64 {
		rand.Read(salt)
		text := strings.ReplaceAll(base64.StdEncoding.EncodeToString(salt), "=", "")
		result += text
	}

	return result[:64]
}
