package config

import (
	"errors"
	"path/filepath"
	"sync"

	"github.com/shhac/lib-agent-cli/creds"
	"github.com/shhac/lib-agent-cli/xdg"
)

type Config struct {
	DefaultOrg    string                  `json:"default_org,omitempty"`
	Organizations map[string]Organization `json:"organizations"`
	Settings      Settings                `json:"settings"`
}

type Organization struct {
	Site string `json:"site,omitempty"`
}

type Settings struct {
	Defaults *DefaultsSettings `json:"defaults,omitempty"`
}

type DefaultsSettings struct {
	Format string `json:"format,omitempty"`
}

var (
	cache       *Config
	cacheMu     sync.Mutex
	overrideDir string
)

func SetConfigDir(dir string) {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	overrideDir = dir
	cache = nil
}

func ConfigDir() string {
	if overrideDir != "" {
		return overrideDir
	}
	return xdg.ConfigDir("agent-dd")
}

func configPath() string {
	return filepath.Join(ConfigDir(), "config.json")
}

// store is the shared credential-family file store: 0600 writes into a 0700
// parent, atomic replacement, and the lock that update() serializes on. This
// used to be hand-rolled with os.ReadFile/os.WriteFile, which meant two
// concurrent read-modify-writes (e.g. `org add` racing `org set-default`)
// could each build their write from a snapshot taken before the other landed
// — the loser's change vanished silently.
func store() creds.Store {
	return creds.Store{Path: configPath()}
}

func Read() *Config {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	if cache != nil {
		return cache
	}
	cache = readFromDiskLocked()
	return cache
}

// readFromDiskLocked loads the config straight from disk, bypassing the
// cache. Callers must hold cacheMu.
func readFromDiskLocked() *Config {
	cfg := newDefaultConfig()
	if err := store().Load(cfg); err != nil {
		cfg = newDefaultConfig()
	}
	if cfg.Organizations == nil {
		cfg.Organizations = make(map[string]Organization)
	}
	return cfg
}

func newDefaultConfig() *Config {
	return &Config{Organizations: make(map[string]Organization)}
}

// Write persists cfg as-is and invalidates the cache. It does not lock, so a
// caller doing its own read-modify-write around Write can still race another
// writer — use update (via StoreOrganization/RemoveOrganization/SetDefault)
// for any read-modify-write instead.
func Write(cfg *Config) error {
	if err := store().Save(cfg); err != nil {
		return err
	}
	cacheMu.Lock()
	cache = nil
	cacheMu.Unlock()
	return nil
}

func ClearCache() {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	cache = nil
}

// errNoop lets a mutate callback decline to write at all — e.g. SetDefault on
// an alias that doesn't exist — while still running under the lock.
var errNoop = errors.New("config: no change to persist")

// update runs mutate against the config loaded FRESH from disk (never the
// in-memory cache, which may be stale relative to a concurrent writer) while
// holding the store's exclusive lock across load, mutate, and save, then
// invalidates the cache so the next Read reflects what was just written.
//
// Two concurrent read-modify-writes that instead went through Read() +
// Write() would each start from a snapshot taken before the other's write
// landed, and the loser's change would be silently erased — the in-process
// analogue of the cross-process lost update.
func update(mutate func(cfg *Config) error) error {
	cfg := newDefaultConfig()
	err := store().Update(cfg, func() error {
		if cfg.Organizations == nil {
			cfg.Organizations = make(map[string]Organization)
		}
		return mutate(cfg)
	})
	if err != nil {
		if errors.Is(err, errNoop) {
			return nil
		}
		return err
	}

	cacheMu.Lock()
	cache = nil
	cacheMu.Unlock()
	return nil
}

func StoreOrganization(alias string, org Organization) error {
	return update(func(cfg *Config) error {
		cfg.Organizations[alias] = org
		if cfg.DefaultOrg == "" {
			cfg.DefaultOrg = alias
		}
		return nil
	})
}

func RemoveOrganization(alias string) error {
	return update(func(cfg *Config) error {
		delete(cfg.Organizations, alias)
		if cfg.DefaultOrg == alias {
			cfg.DefaultOrg = ""
			for name := range cfg.Organizations {
				cfg.DefaultOrg = name
				break
			}
		}
		return nil
	})
}

func SetDefault(alias string) error {
	return update(func(cfg *Config) error {
		if _, ok := cfg.Organizations[alias]; !ok {
			return errNoop
		}
		cfg.DefaultOrg = alias
		return nil
	})
}
