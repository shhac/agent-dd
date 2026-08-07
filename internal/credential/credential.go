package credential

import (
	"fmt"
	"path/filepath"

	"github.com/shhac/lib-agent-cli/creds"

	"github.com/shhac/agent-dd/internal/config"
)

const keychainSentinel = "__KEYCHAIN__"

type Credential struct {
	APIKey          string `json:"api_key"`
	AppKey          string `json:"app_key"`
	KeychainManaged bool   `json:"keychain_managed,omitempty"`
}

type credentialEntry struct {
	APIKey          string `json:"api_key"`
	AppKey          string `json:"app_key"`
	KeychainManaged bool   `json:"keychain_managed,omitempty"`
}

type NotFoundError struct {
	Name string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("organization credential %q not found", e.Name)
}

func credentialsPath() string {
	return filepath.Join(config.ConfigDir(), "credentials.json")
}

// store is the shared credential-family file store: 0600 writes into a 0700
// parent, atomic replacement, and the lock that updateIndex serializes on.
// This used to be hand-rolled with os.ReadFile/os.WriteFile, which carried a
// lost-update race: two concurrent Store/Remove calls each built their write
// from a snapshot taken before the other landed, and the loser's entry
// vanished.
func store() creds.Store {
	return creds.Store{Path: credentialsPath()}
}

func readIndex() (map[string]credentialEntry, error) {
	index := map[string]credentialEntry{}
	if err := store().Load(&index); err != nil {
		return nil, err
	}
	if index == nil {
		index = make(map[string]credentialEntry)
	}
	return index, nil
}

// updateIndex applies mutate to the index loaded fresh from disk, holding one
// exclusive lock across the load, the mutation, and the save — so two
// concurrent `org add`/`org remove` invocations serialize instead of
// clobbering each other.
//
// The index write is the step that must not race: by the time it happens the
// secret is already in the OS keychain (or, on the fallback path, about to be
// written into the same 0600 file), so an entry lost to a concurrent writer
// leaves a live credential that `auth list` cannot show and `auth remove`
// cannot delete — it looks the name up in this index first. Measured against
// the hand-rolled version, twenty concurrent writers left one surviving entry.
func updateIndex(mutate func(index map[string]credentialEntry) error) error {
	index := map[string]credentialEntry{}
	return store().Update(&index, func() error {
		if index == nil {
			index = make(map[string]credentialEntry)
		}
		return mutate(index)
	})
}

func Store(name string, cred Credential) (string, error) {
	storage := "file"
	entry := credentialEntry{
		APIKey: cred.APIKey,
		AppKey: cred.AppKey,
	}

	if err := keychainStore(name, cred.APIKey, cred.AppKey); err == nil {
		entry.APIKey = keychainSentinel
		entry.AppKey = keychainSentinel
		entry.KeychainManaged = true
		storage = "keychain"
	}

	if err := updateIndex(func(index map[string]credentialEntry) error {
		index[name] = entry
		return nil
	}); err != nil {
		return "", err
	}
	return storage, nil
}

func Get(name string) (*Credential, error) {
	index, err := readIndex()
	if err != nil {
		return nil, err
	}
	entry, ok := index[name]
	if !ok {
		return nil, &NotFoundError{Name: name}
	}

	cred := &Credential{
		APIKey:          entry.APIKey,
		AppKey:          entry.AppKey,
		KeychainManaged: entry.KeychainManaged,
	}

	if entry.KeychainManaged {
		if apiKey, appKey, err := keychainGet(name); err == nil {
			cred.APIKey = apiKey
			cred.AppKey = appKey
		}
	}

	return cred, nil
}

func Remove(name string) error {
	return updateIndex(func(index map[string]credentialEntry) error {
		entry, ok := index[name]
		if !ok {
			return &NotFoundError{Name: name}
		}

		if entry.KeychainManaged {
			keychainDelete(name)
		}

		delete(index, name)
		return nil
	})
}

func List() ([]string, error) {
	index, err := readIndex()
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(index))
	for name := range index {
		names = append(names, name)
	}
	return names, nil
}
