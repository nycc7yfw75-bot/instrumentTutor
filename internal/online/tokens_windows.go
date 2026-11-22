//go:build windows

package online

import (
	"errors"

	"github.com/danieljoos/wincred"
)

// CredStore keeps the account in Windows Credential Manager (a generic
// credential under "Headliner/account", readable only by this Windows
// user). github.com/danieljoos/wincred is a small, maintained wrapper of
// the Win32 Cred* calls with no other dependencies.
type CredStore struct{ Target string }

// SystemStore is the platform's token store.
func SystemStore() TokenStore { return CredStore{Target: credTarget} }

func (c CredStore) Load() (Saved, error) {
	cred, err := wincred.GetGenericCredential(c.Target)
	if err != nil {
		if errors.Is(err, wincred.ErrElementNotFound) {
			return Saved{}, ErrNoToken
		}
		return Saved{}, err
	}
	return decodeSaved(cred.CredentialBlob)
}

func (c CredStore) Save(s Saved) error {
	b, err := encodeSaved(s)
	if err != nil {
		return err
	}
	cred := wincred.NewGenericCredential(c.Target)
	cred.UserName = s.Email
	cred.CredentialBlob = b
	cred.Persist = wincred.PersistLocalMachine
	return cred.Write()
}

func (c CredStore) Clear() error {
	cred, err := wincred.GetGenericCredential(c.Target)
	if err != nil {
		if errors.Is(err, wincred.ErrElementNotFound) {
			return nil
		}
		return err
	}
	return cred.Delete()
}
