package credential

import (
	"encoding/json"

	"github.com/shhac/lib-agent-cli/creds"
)

const keychainService = "app.paulie.agent-dd"

func keychain() *creds.Keychain {
	return creds.NewKeychain(keychainService)
}

func keychainStore(name, apiKey, appKey string) error {
	kc := keychain()
	if !kc.Available() {
		return creds.ErrKeychainUnavailable
	}

	data, _ := json.Marshal(map[string]string{
		"api_key": apiKey,
		"app_key": appKey,
	})

	return kc.Set(name, string(data))
}

func keychainGet(name string) (apiKey, appKey string, err error) {
	kc := keychain()
	if !kc.Available() {
		return "", "", creds.ErrKeychainUnavailable
	}

	out, ok := kc.Get(name)
	if !ok {
		return "", "", creds.ErrKeychainUnavailable
	}

	var keys map[string]string
	if err := json.Unmarshal([]byte(out), &keys); err != nil {
		return "", "", err
	}
	return keys["api_key"], keys["app_key"], nil
}

func keychainDelete(name string) {
	kc := keychain()
	if !kc.Available() {
		return
	}
	_ = kc.Delete(name)
}
