package modjar

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestGsonDecodeBWGMobSpawn(t *testing.T) {
	root := filepath.Join(os.TempDir(), "bwgjar2")
	raw := map[string][]byte{}
	for _, rel := range []string{
		"net/potionstudios/biomeswevegone/config/configs/BWGMobSpawnConfig.class",
		"net/potionstudios/biomeswevegone/config/configs/BWGMobSpawnConfig$BWGSpawnConfig.class",
	} {
		b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			t.Skip(err)
		}
		raw[rel[:len(rel)-len(".class")]] = b
	}
	v := newMachine(context.Background(), raw)
	v.host = &bindings{vm: v}
	json := "{\n  \"spawn\": {\n    \"man_o_war\": true,\n    \"oddion\": true\n  }\n}"
	got := v.gsonDecode(json, classLiteral("net/potionstudios/biomeswevegone/config/configs/BWGMobSpawnConfig"))
	o := got.(*object)
	spawnKey := fieldKey(reference{"net/potionstudios/biomeswevegone/config/configs/BWGMobSpawnConfig", "spawn", "Lnet/potionstudios/biomeswevegone/config/configs/BWGMobSpawnConfig$BWGSpawnConfig;"})
	spawn := o.fields[spawnKey].(*object)
	warKey := fieldKey(reference{"net/potionstudios/biomeswevegone/config/configs/BWGMobSpawnConfig$BWGSpawnConfig", "man_o_war", "Z"})
	if spawn.fields[warKey] != int32(1) {
		t.Fatalf("man_o_war=%v fields=%v", spawn.fields[warKey], spawn.fields)
	}
}

func TestGsonBoolRoundTrip(t *testing.T) {
	root := filepath.Join(os.TempDir(), "bwgjar2")
	raw := map[string][]byte{}
	for _, rel := range []string{
		"net/potionstudios/biomeswevegone/config/configs/BWGMobSpawnConfig.class",
		"net/potionstudios/biomeswevegone/config/configs/BWGMobSpawnConfig$BWGSpawnConfig.class",
	} {
		b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			t.Skip(err)
		}
		raw[rel[:len(rel)-len(".class")]] = b
	}
	v := newMachine(context.Background(), raw)
	v.host = &bindings{vm: v}
	first := v.gsonDecode(`{"spawn":{"man_o_war":true,"oddion":false}}`, classLiteral("net/potionstudios/biomeswevegone/config/configs/BWGMobSpawnConfig"))
	encoded := v.gsonEncode(first)
	second := v.gsonDecode(encoded, classLiteral("net/potionstudios/biomeswevegone/config/configs/BWGMobSpawnConfig"))
	spawnKey := fieldKey(reference{"net/potionstudios/biomeswevegone/config/configs/BWGMobSpawnConfig", "spawn", "Lnet/potionstudios/biomeswevegone/config/configs/BWGMobSpawnConfig$BWGSpawnConfig;"})
	warKey := fieldKey(reference{"net/potionstudios/biomeswevegone/config/configs/BWGMobSpawnConfig$BWGSpawnConfig", "man_o_war", "Z"})
	oddKey := fieldKey(reference{"net/potionstudios/biomeswevegone/config/configs/BWGMobSpawnConfig$BWGSpawnConfig", "oddion", "Z"})
	spawn := second.(*object).fields[spawnKey].(*object)
	if spawn.fields[warKey] != int32(1) || spawn.fields[oddKey] != int32(0) {
		t.Fatalf("roundtrip fields war=%v odd=%v encoded=%s", spawn.fields[warKey], spawn.fields[oddKey], encoded)
	}
}
