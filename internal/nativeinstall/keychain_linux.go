//go:build linux

package nativeinstall

import (
	"crypto/rand"
	"errors"
	"fmt"
	"github.com/godbus/dbus/v5"
)

type serviceSecret struct {
	Session     dbus.ObjectPath
	Parameters  []byte
	Value       []byte
	ContentType string
}

func systemProtectedKey(account string, create bool) ([]byte, error) {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return nil, fmt.Errorf("Secret Service session unavailable: %w", err)
	}
	defer conn.Close()
	const name = "org.freedesktop.secrets"
	service := conn.Object(name, dbus.ObjectPath("/org/freedesktop/secrets"))
	var output dbus.Variant
	var session dbus.ObjectPath
	if err = service.Call("org.freedesktop.Secret.Service.OpenSession", 0, "plain", dbus.MakeVariant("")).Store(&output, &session); err != nil {
		return nil, fmt.Errorf("Secret Service unavailable: %w", err)
	}
	defer conn.Object(name, session).Call("org.freedesktop.Secret.Session.Close", 0)
	attributes := map[string]string{"application": "org.levimc.levilauncher", "cache": account}
	var unlocked, locked []dbus.ObjectPath
	if err = service.Call("org.freedesktop.Secret.Service.SearchItems", 0, attributes).Store(&unlocked, &locked); err != nil {
		return nil, err
	}
	if len(locked) > 0 {
		return nil, errors.New("unlock your desktop keyring to use the Microsoft account cache")
	}
	if len(unlocked) > 0 {
		var secret serviceSecret
		if err = conn.Object(name, unlocked[0]).Call("org.freedesktop.Secret.Item.GetSecret", 0, session).Store(&secret); err != nil {
			return nil, err
		}
		if len(secret.Value) != 32 {
			clear(secret.Value)
			return nil, errors.New("invalid Secret Service cache key")
		}
		return secret.Value, nil
	}
	if !create {
		return nil, errors.New("Microsoft account cache key is missing from Secret Service")
	}
	var collection dbus.ObjectPath
	if err = service.Call("org.freedesktop.Secret.Service.ReadAlias", 0, "default").Store(&collection); err != nil {
		return nil, err
	}
	if collection == "/" {
		return nil, errors.New("configure a default desktop keyring before saving the Microsoft account")
	}
	key := make([]byte, 32)
	if _, err = rand.Read(key); err != nil {
		return nil, err
	}
	properties := map[string]dbus.Variant{"org.freedesktop.Secret.Item.Label": dbus.MakeVariant("LeviLauncher Microsoft account cache"), "org.freedesktop.Secret.Item.Attributes": dbus.MakeVariant(attributes)}
	var item, prompt dbus.ObjectPath
	err = conn.Object(name, collection).Call("org.freedesktop.Secret.Collection.CreateItem", 0, properties, serviceSecret{session, nil, key, "application/octet-stream"}, true).Store(&item, &prompt)
	if err != nil {
		clear(key)
		return nil, fmt.Errorf("cannot save the Microsoft account key: %w", err)
	}
	if prompt != "/" {
		clear(key)
		return nil, errors.New("unlock your desktop keyring before saving the Microsoft account")
	}
	return key, nil
}
