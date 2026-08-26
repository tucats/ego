package auth

import (
	"encoding/hex"
	"encoding/json"
	"strings"

	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"
	"github.com/tucats/ego/internal/cli/ui"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/resources"
)

// passkeysTable is the name of the table holding one row per registered
// WebAuthn credential, joined to its owning user via UserID. This replaces
// the earlier design where every credential for a user was packed into a
// single JSON array stored in the "credentials" table's own "passkeys"
// column -- a design that could not cleanly represent the many-passkeys-
// to-one-user relationship, and forced every single-credential change (add
// one, update one sign counter, clear all) to read and rewrite the user's
// entire credential array.
const passkeysTable = "passkeys"

// passkeyRow is the SQL-table shape of one registered WebAuthn credential.
// ID is a synthetic primary key for the row -- not the credential's own ID,
// which is opaque binary data assigned by the authenticator and is not a
// convenient SQL primary key. UserID is the join key back to defs.User.ID.
// CredentialID is the hex-encoded form of the credential's own opaque ID
// (webauthn.Credential.ID, a []byte -- hex-encoded because the resources
// framework has no []byte column type), broken out into its own column
// solely so UpdatePasskeySignCount can look up "the row for this specific
// credential" with a SQL filter instead of reading every row for the user
// and unmarshaling each one just to compare IDs in Go. Credential holds the
// full marshaled webauthn.Credential (including its own ID a second time,
// inside the blob) -- every other field of a passkey (attestation type,
// transport, AAGUID, flags, ...) stays in this JSON blob rather than being
// broken out, because nothing in this codebase queries on them, and several
// don't map to a scalar SQL column at all.
type passkeyRow struct {
	ID           string
	UserID       string
	CredentialID string
	Credential   json.RawMessage
}

// openPasskeyStore opens (creating if necessary) the passkeys table on svc,
// migrates a table that predates the CredentialID column (see
// addCredentialIDColumnIfMissing and backfillCredentialIDs), then migrates
// any legacy credentials still packed into the "credentials" table's old
// "passkeys" column (see migrateLegacyPasskeys).
func openPasskeyStore(svc *databaseService, connStr string) error {
	handle, err := resources.Open(passkeyRow{}, passkeysTable, connStr)
	if err != nil {
		return errors.New(err)
	}

	handle.SetPrimaryKey("ID")

	if err := handle.CreateIf(); err != nil {
		return errors.New(err)
	}

	svc.passkeyHandle = handle

	if err := addCredentialIDColumnIfMissing(handle); err != nil {
		return err
	}

	if err := backfillCredentialIDs(svc); err != nil {
		return err
	}

	return migrateLegacyPasskeys(svc)
}

// addCredentialIDColumnIfMissing migrates a passkeys table created before
// the CredentialID column existed: CreateIf only creates a table that is
// missing entirely, it never alters one that already exists, so a
// long-lived deployment's table would otherwise be stuck without this
// column forever. ALTER TABLE ADD COLUMN fails if the column is already
// there -- SQLite says "duplicate column name", PostgreSQL says "column ...
// already exists" -- so that specific failure is treated as success and
// silently ignored, the same detect-and-ignore approach already used for
// the "credentials" table's own "lasttokenat" column and the "task_state"
// table's "description" column.
func addCredentialIDColumnIfMissing(handle *resources.ResHandle) error {
	_, err := handle.Database.Exec(`ALTER TABLE "passkeys" ADD COLUMN "credentialid" TEXT`)
	if err == nil {
		return nil
	}

	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "duplicate column") || strings.Contains(msg, "already exists") {
		return nil
	}

	return errors.New(err)
}

// backfillCredentialIDs populates CredentialID for any passkeys row written
// before that column existed, so UpdatePasskeySignCount's column-based
// lookup finds every pre-existing credential, not just ones added after
// this migration landed.
func backfillCredentialIDs(svc *databaseService) error {
	rowSet, err := svc.passkeyHandle.Begin().Read()
	if err != nil {
		return errors.New(err)
	}

	for _, row := range rowSet {
		r, ok := row.(*passkeyRow)
		if !ok || r.CredentialID != "" {
			continue
		}

		var cred webauthn.Credential

		if err := json.Unmarshal(r.Credential, &cred); err != nil {
			continue
		}

		r.CredentialID = hex.EncodeToString(cred.ID)

		if err := svc.passkeyHandle.Begin().UpdateOne(*r); err != nil {
			return errors.New(err)
		}
	}

	return nil
}

// migrateLegacyPasskeys detects "credentials" rows that still carry their
// passkeys packed into the old "passkeys" column -- a deployment that
// predates the separate passkeys table -- and moves each credential in the
// array into its own row of the new table, then clears the column so the
// migration does not repeat on the next startup. This mirrors the
// detect-and-migrate pattern already used for the "lasttokenat" column (see
// NewDatabaseService) and the dsns_auth synthetic id migration.
func migrateLegacyPasskeys(svc *databaseService) error {
	rowSet, err := svc.userHandle.Begin().Read()
	if err != nil {
		return errors.New(err)
	}

	for _, row := range rowSet {
		user, ok := row.(*defs.User)
		if !ok {
			continue
		}

		text := strings.TrimSpace(string(user.Passkeys))
		if text == "" || text == "null" || text == "[]" {
			continue
		}

		if user.ID == uuid.Nil {
			// No stable join key to migrate this user's passkeys onto;
			// leave the legacy column alone rather than risk grouping
			// these credentials under a shared zero-value user ID.
			ui.Log(ui.ServerLogger, "server.db.error", ui.A{
				"error": errors.ErrNoSuchUser.Context(user.Name)})

			continue
		}

		var creds []webauthn.Credential

		if err := json.Unmarshal(user.Passkeys, &creds); err != nil {
			ui.Log(ui.ServerLogger, "server.db.error", ui.A{
				"error": err})

			continue
		}

		for _, cred := range creds {
			if err := svc.insertPasskey(user.ID, cred); err != nil {
				return errors.New(err)
			}
		}

		user.Passkeys = nil

		if err := svc.userHandle.Begin().Update(*user, svc.userHandle.Equals("name", user.Name)); err != nil {
			return errors.New(err)
		}

		ui.Log(ui.AuthLogger, "auth.webauthn.passkeys.migrated", ui.A{
			"user":  user.Name,
			"count": len(creds)})
	}

	return nil
}

// insertPasskey writes a single credential row for the given user.
func (pg *databaseService) insertPasskey(userID uuid.UUID, cred webauthn.Credential) error {
	raw, err := json.Marshal(cred)
	if err != nil {
		return errors.New(err)
	}

	row := passkeyRow{
		ID:           uuid.New().String(),
		UserID:       userID.String(),
		CredentialID: hex.EncodeToString(cred.ID),
		Credential:   raw,
	}

	return pg.passkeyHandle.Begin().Insert(row)
}

// ListPasskeys returns every credential registered for the given user.
func (pg *databaseService) ListPasskeys(user defs.User) ([]webauthn.Credential, error) {
	rowSet, err := pg.passkeyHandle.Begin().Read(pg.passkeyHandle.Equals("userid", user.ID.String()))
	if err != nil {
		return nil, errors.New(err)
	}

	creds := make([]webauthn.Credential, 0, len(rowSet))

	for _, row := range rowSet {
		r, ok := row.(*passkeyRow)
		if !ok {
			continue
		}

		var cred webauthn.Credential

		if err := json.Unmarshal(r.Credential, &cred); err != nil {
			continue
		}

		creds = append(creds, cred)
	}

	return creds, nil
}

// CountPasskeys returns the number of credentials registered for the given
// user, without unmarshaling each one.
func (pg *databaseService) CountPasskeys(user defs.User) (int, error) {
	rowSet, err := pg.passkeyHandle.Begin().Read(pg.passkeyHandle.Equals("userid", user.ID.String()))
	if err != nil {
		return 0, errors.New(err)
	}

	return len(rowSet), nil
}

// AddPasskey registers a new credential for the given user.
func (pg *databaseService) AddPasskey(user defs.User, cred webauthn.Credential) error {
	return pg.insertPasskey(user.ID, cred)
}

// UpdatePasskeySignCount persists an updated authenticator sign counter for
// the credential identified by credentialID. This is called after every
// successful passkey login to guard against clone replay. The CredentialID
// column lets this go straight to the one row that matters via a SQL
// filter, rather than reading every row for the user and unmarshaling each
// one just to compare IDs in Go.
func (pg *databaseService) UpdatePasskeySignCount(user defs.User, credentialID []byte, signCount uint32) error {
	rowSet, err := pg.passkeyHandle.Begin().Read(
		pg.passkeyHandle.Equals("userid", user.ID.String()),
		pg.passkeyHandle.Equals("credentialid", hex.EncodeToString(credentialID)),
	)
	if err != nil {
		return errors.New(err)
	}

	if len(rowSet) == 0 {
		return errors.ErrNotFound
	}

	r, ok := rowSet[0].(*passkeyRow)
	if !ok {
		return errors.ErrNotFound
	}

	var cred webauthn.Credential

	if err := json.Unmarshal(r.Credential, &cred); err != nil {
		return errors.New(err)
	}

	cred.Authenticator.SignCount = signCount

	raw, err := json.Marshal(cred)
	if err != nil {
		return errors.New(err)
	}

	r.Credential = raw

	return pg.passkeyHandle.Begin().UpdateOne(*r)
}

// DeletePasskeys removes every credential registered for the given user.
func (pg *databaseService) DeletePasskeys(user defs.User) error {
	if _, err := pg.passkeyHandle.Begin().Delete(pg.passkeyHandle.Equals("userid", user.ID.String())); err != nil {
		return errors.New(err)
	}

	return nil
}
