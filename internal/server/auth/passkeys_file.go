package auth

import (
	"bytes"
	"encoding/json"

	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
)

// The file-backed service has no separate "table" to normalize -- a user's
// passkeys already live only inside that user's own JSON record, with no
// other record to join against, so there is nothing to migrate here. These
// methods just give the file backend the same passkey-storage interface as
// the database backend (see passkeys_sqldb.go), reading and writing
// defs.User.Passkeys under the existing file-service lock.

// listPasskeysLocked returns the credentials stored for the named user.
// The caller must already hold f.lock.
func (f *fileService) listPasskeysLocked(name string) ([]webauthn.Credential, error) {
	u, ok := f.data[name]
	if !ok {
		return nil, errors.ErrNoSuchUser.Context(name)
	}

	if len(u.Passkeys) == 0 {
		return nil, nil
	}

	var creds []webauthn.Credential

	if err := json.Unmarshal(u.Passkeys, &creds); err != nil {
		return nil, errors.New(err)
	}

	return creds, nil
}

// ListPasskeys returns every credential registered for the given user.
func (f *fileService) ListPasskeys(user defs.User) ([]webauthn.Credential, error) {
	f.lock.Lock()
	defer f.lock.Unlock()

	return f.listPasskeysLocked(user.Name)
}

// CountPasskeys returns the number of credentials registered for the given user.
func (f *fileService) CountPasskeys(user defs.User) (int, error) {
	f.lock.Lock()
	defer f.lock.Unlock()

	creds, err := f.listPasskeysLocked(user.Name)

	return len(creds), err
}

// AddPasskey registers a new credential for the given user.
func (f *fileService) AddPasskey(user defs.User, cred webauthn.Credential) error {
	f.lock.Lock()
	defer f.lock.Unlock()

	creds, err := f.listPasskeysLocked(user.Name)
	if err != nil {
		return err
	}

	creds = append(creds, cred)

	return f.writePasskeysLocked(user.Name, creds)
}

// UpdatePasskeySignCount persists an updated authenticator sign counter for
// the credential identified by credentialID.
func (f *fileService) UpdatePasskeySignCount(user defs.User, credentialID []byte, signCount uint32) error {
	f.lock.Lock()
	defer f.lock.Unlock()

	creds, err := f.listPasskeysLocked(user.Name)
	if err != nil {
		return err
	}

	found := false

	for i, c := range creds {
		if bytes.Equal(c.ID, credentialID) {
			creds[i].Authenticator.SignCount = signCount
			found = true

			break
		}
	}

	if !found {
		return errors.ErrNotFound
	}

	return f.writePasskeysLocked(user.Name, creds)
}

// DeletePasskeys removes every credential registered for the given user.
func (f *fileService) DeletePasskeys(user defs.User) error {
	f.lock.Lock()
	defer f.lock.Unlock()

	u, ok := f.data[user.Name]
	if !ok {
		return nil
	}

	u.Passkeys = nil
	f.data[user.Name] = u
	f.dirty = true

	return nil
}

// writePasskeysLocked marshals creds back into the named user's record and
// marks the store dirty. The caller must already hold f.lock and must have
// already confirmed the user exists (e.g. via listPasskeysLocked).
func (f *fileService) writePasskeysLocked(name string, creds []webauthn.Credential) error {
	u, ok := f.data[name]
	if !ok {
		return errors.ErrNoSuchUser.Context(name)
	}

	raw, err := json.Marshal(creds)
	if err != nil {
		return errors.New(err)
	}

	u.Passkeys = raw
	f.data[name] = u
	f.dirty = true

	return nil
}
