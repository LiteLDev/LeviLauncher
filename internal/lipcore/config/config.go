// Package config ports lip 0.38's user configuration
// (src/Lip.Core/Services/ConfigService.cs at v0.38.0-beta.1;
// protocol/userspace.md §2, config.schema.yaml) and the migration of lip
// v0.x's liprc.json (protocol/compatibility.md §4).
//
// The configuration is <Config>/config.yaml (lipdirs.ConfigDir()): a YAML
// file with "format": 4 and a mapping of the configuration keys to their
// values. The only configuration key is github_proxy; an absent key has its
// default value, none. A missing, unreadable or invalid config.yaml means
// every key has its default, which lip reports to the user (Message) and
// does not treat as a failure.
//
// When config.yaml is missing and <Config>/liprc.json is a JSON object with
// format_version 3, its github_proxy (when a string) is written atomically as
// config.yaml before it is read, and liprc.json is deleted. Any other
// liprc.json is kept, and every key has its default.
//
// Every function has a variant taking the configuration directory (In), for
// tests; the others use lipdirs.ConfigDir().
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/liteldev/LeviLauncher/internal/lipcore/doc"
	"github.com/liteldev/LeviLauncher/internal/lipcore/lipdirs"
	"github.com/liteldev/LeviLauncher/internal/lipcore/txn"
)

const (
	// GithubProxyKey is the only configuration key: a URL prefix through
	// which lip reaches GitHub.
	GithubProxyKey = "github_proxy"
	// FileName is the configuration file in the configuration directory.
	FileName = "config.yaml"
	// LiprcFileName is lip v0.x's configuration file.
	LiprcFileName = "liprc.json"

	formatKey              = "format"
	format                 = 4
	liprcFormatVersionKey  = "format_version"
	liprcFormatVersion     = 3
	configHelpLines        = "'format: 4' and 'github_proxy: https://example.com/'"
	unknownKeyMessageFront = "' is not a configuration key. The only configuration key is "
)

// Config is the configuration.
type Config struct {
	// GithubProxy is the value of github_proxy; meaningful only when
	// HasGithubProxy.
	GithubProxy string
	// HasGithubProxy is false when github_proxy has its default, none (lip's
	// null). A github_proxy of "" is set, to "".
	HasGithubProxy bool
}

// Level is how lip prints a Message.
type Level int

const (
	// Info is IUserInteraction.PrintInfo.
	Info Level = iota
	// Error is IUserInteraction.PrintError.
	Error
)

// Message is something lip tells the user while reading the configuration
// (IUserInteraction.PrintError / PrintInfo), in order.
type Message struct {
	Level Level
	Text  string
}

// KeyError is the ArgumentException of an unknown configuration key: a
// mistake of the user.
type KeyError struct{ Key string }

func (e *KeyError) Error() string {
	return "'" + e.Key + unknownKeyMessageFront + GithubProxyKey +
		", a URL prefix through which lip reaches GitHub."
}

// WriteError is the IOException or UnauthorizedAccessException of a failed
// write of config.yaml; Message is lip's message, Err the cause.
type WriteError struct {
	Message string
	Err     error
}

func (e *WriteError) Error() string { return e.Message }
func (e *WriteError) Unwrap() error { return e.Err }

// Load is ConfigService.GetGithubProxy in lipdirs.ConfigDir().
func Load() (Config, []Message, error) { return LoadIn(lipdirs.ConfigDir()) }

// LoadIn reads the configuration in dir, migrating liprc.json when
// config.yaml is missing. A config.yaml that cannot be read yields the
// default configuration and messages saying so; an error is returned only
// when migrating liprc.json fails to write config.yaml or to delete
// liprc.json.
func LoadIn(dir string) (Config, []Message, error) {
	var msgs []Message
	c, err := loadGithubProxy(dir, &msgs)
	return c, msgs, err
}

// Get is ConfigService.Get: the value of a key, "" when it has its default.
func Get(key string) (string, []Message, error) { return GetIn(lipdirs.ConfigDir(), key) }

// GetIn is Get in dir.
func GetIn(dir, key string) (string, []Message, error) {
	if err := checkKey(key); err != nil {
		return "", nil, err
	}
	c, msgs, err := LoadIn(dir)
	return c.GithubProxy, msgs, err
}

// List is ConfigService.List: every key with its value, "" for the default.
func List() (map[string]string, []Message, error) { return ListIn(lipdirs.ConfigDir()) }

// ListIn is List in dir.
func ListIn(dir string) (map[string]string, []Message, error) {
	c, msgs, err := LoadIn(dir)
	if err != nil {
		return nil, msgs, err
	}
	return map[string]string{GithubProxyKey: c.GithubProxy}, msgs, nil
}

// Set is ConfigService.Set: sets a key (only github_proxy), writing
// config.yaml with "format": 4 and the key.
func Set(key, value string) ([]Message, error) { return SetIn(lipdirs.ConfigDir(), key, value) }

// SetIn is Set in dir.
func SetIn(dir, key, value string) ([]Message, error) { return save(dir, key, &value) }

// Delete is ConfigService.Delete: resets a key (only github_proxy) to its
// default, writing config.yaml with "format": 4 only.
func Delete(key string) ([]Message, error) { return DeleteIn(lipdirs.ConfigDir(), key) }

// DeleteIn is Delete in dir.
func DeleteIn(dir, key string) ([]Message, error) { return save(dir, key, nil) }

func save(dir, key string, value *string) ([]Message, error) {
	if err := checkKey(key); err != nil {
		return nil, err
	}
	// Read first, as lip does: it migrates liprc.json and reports an invalid
	// config.yaml before replacing it.
	_, msgs, err := LoadIn(dir)
	if err != nil {
		return msgs, err
	}
	return msgs, saveConfig(dir, value)
}

func checkKey(key string) error {
	if key != GithubProxyKey {
		return &KeyError{Key: key}
	}
	return nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func loadGithubProxy(dir string, msgs *[]Message) (Config, error) {
	configPath := filepath.Join(dir, FileName)
	if !fileExists(configPath) {
		return migrateLiprc(dir, msgs)
	}
	c, err := readConfig(configPath)
	if err != nil {
		*msgs = append(*msgs,
			Message{Error, fmt.Sprintf("Cannot read the configuration file %s: %s", configPath, err.Error())},
			Message{Info, fmt.Sprintf("lip continues with the default configuration (no %s). Fix the file so"+
				" that it is YAML with the lines %s, or run 'lip config set %s <URL>' or 'lip config delete"+
				" %s' to rewrite it.", GithubProxyKey, configHelpLines, GithubProxyKey, GithubProxyKey)})
		return Config{}, nil
	}
	return c, nil
}

func readConfig(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	v, err := doc.ParseYAML(data, "")
	if err != nil {
		return Config{}, err
	}
	root, ok := v.(*doc.Object)
	if !ok {
		return Config{}, errors.New("It is not a YAML mapping of configuration keys to their values, like " +
			configHelpLines + " on separate lines.")
	}
	if f, ok := integerMember(root, formatKey); !ok || f != format {
		return Config{}, errors.New("Its field format is missing or not 4; it must be 'format: 4'.")
	}
	// Keys that are not configuration keys are ignored, whatever their value.
	value, ok := root.Get(GithubProxyKey)
	if !ok {
		return Config{}, nil
	}
	s, ok := value.(string)
	if !ok {
		return Config{}, errors.New("Its field github_proxy is not a single text value; it must be a URL" +
			" prefix like https://example.com/.")
	}
	return Config{GithubProxy: s, HasGithubProxy: true}, nil
}

func integerMember(o *doc.Object, key string) (int64, bool) {
	v, ok := o.Get(key)
	if !ok {
		return 0, false
	}
	return doc.IntegerMember(v)
}

// migrateLiprc is ConfigService.MigrateLiprc (protocol/compatibility.md §4).
func migrateLiprc(dir string, msgs *[]Message) (Config, error) {
	liprcPath := filepath.Join(dir, LiprcFileName)
	if !fileExists(liprcPath) {
		return Config{}, nil
	}
	c, err := readLiprc(liprcPath)
	if err != nil {
		configPath := filepath.Join(dir, FileName)
		*msgs = append(*msgs,
			Message{Error, fmt.Sprintf("Cannot read the lip v0.x configuration file %s: %s", liprcPath, err.Error())},
			Message{Info, fmt.Sprintf("lip continues with the default configuration (no %s) and keeps %s as it"+
				" is. Run 'lip config set %s <URL>' to write %s, or delete %s if you no longer need it.",
				GithubProxyKey, liprcPath, GithubProxyKey, configPath, liprcPath)})
		return Config{}, nil
	}
	var value *string
	if c.HasGithubProxy {
		value = &c.GithubProxy
	}
	if err := saveConfig(dir, value); err != nil {
		return Config{}, err
	}
	// File.Delete does nothing for a missing file (another process may have
	// migrated it meanwhile).
	if err := os.Remove(liprcPath); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return Config{}, err
	}
	*msgs = append(*msgs, Message{Info, fmt.Sprintf("Converted the lip v0.x configuration file %s to %s.",
		liprcPath, filepath.Join(dir, FileName))})
	return c, nil
}

// readLiprc reads liprc.json as ConfigService.MigrateLiprc does (see
// parseLiprcJSON).
func readLiprc(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	v, err := parseLiprcJSON(data)
	if err != nil {
		return Config{}, err
	}
	root, ok := v.(liprcObject)
	f, isInt := doc.IntegerMember(root[liprcFormatVersionKey])
	if !ok || !isInt || f != liprcFormatVersion {
		return Config{}, errors.New("It is not a JSON object with the field format_version set to 3, the" +
			" only lip v0.x configuration that lip can convert.")
	}
	if raw, ok := root[GithubProxyKey].(liprcString); ok {
		s, err := decodeJSONString(raw)
		if err != nil {
			return Config{}, err
		}
		return Config{GithubProxy: s, HasGithubProxy: true}, nil
	}
	return Config{}, nil
}

// saveConfig is ConfigService.SaveConfig: writes config.yaml atomically
// (protocol/userspace.md §2) with "format" and, unless value is nil,
// github_proxy: the whole content goes to a uniquely named .tmp file in the
// configuration directory, which is then renamed over config.yaml.
func saveConfig(dir string, value *string) error {
	o := doc.NewObject()
	o.Set(formatKey, json.Number("4"))
	if value != nil {
		o.Set(GithubProxyKey, *value)
	}
	configPath := filepath.Join(dir, FileName)
	tempPath := filepath.Join(dir, txn.NewGUID()+".tmp")
	err := writeAndMove(dir, tempPath, configPath, doc.MarshalYAML(o))
	if fileExists(tempPath) {
		_ = os.Remove(tempPath)
	}
	if err == nil {
		return nil
	}
	if errors.Is(err, fs.ErrPermission) {
		return &WriteError{Message: fmt.Sprintf("lip is not allowed to write the configuration file %s; it"+
			" was not changed.", configPath), Err: err}
	}
	return &WriteError{Message: fmt.Sprintf("Cannot write the configuration file %s; it was not changed.",
		configPath), Err: err}
}

func writeAndMove(dir, tempPath, configPath string, data []byte) error {
	// FileSystemExtensions.CreateFileWithDirectory.
	if err := os.MkdirAll(dir, 0o777); err != nil {
		return &WriteError{Message: fmt.Sprintf("Could not create the file %s.", tempPath), Err: err}
	}
	f, err := os.Create(tempPath)
	if err != nil {
		return &WriteError{Message: fmt.Sprintf("Could not create the file %s.", tempPath), Err: err}
	}
	_, err = f.Write(data)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	return os.Rename(tempPath, configPath)
}
