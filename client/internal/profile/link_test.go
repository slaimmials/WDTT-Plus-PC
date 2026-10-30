package profile

import (
	"reflect"
	"testing"
)

func TestParseModernLinkRoundTrip(t *testing.T) {
	want := Link{
		Name:      "Дом",
		Host:      "vpn.example.com",
		DTLSPort:  56000,
		WGPort:    56001,
		LocalPort: 9000,
		Password:  "p@ss word",
		Hashes:    []string{"abc", "def"},
	}
	got, err := ParseLink("вот ссылка: " + want.String() + ".")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestParseLegacyLink(t *testing.T) {
	got, err := ParseLink("wdtt://1.2.3.4:56000:56001:9000:secret:https://vk.com/call/join/AbC_1?x=1,def")
	if err != nil {
		t.Fatal(err)
	}
	if got.Host != "1.2.3.4" || got.Password != "secret" || !reflect.DeepEqual(got.Hashes, []string{"AbC_1", "def"}) {
		t.Fatalf("unexpected link %+v", got)
	}
}

func TestParseLinkRejectsInvalid(t *testing.T) {
	for _, input := range []string{
		"https://example.com",
		"wdtt://host:1:2:3:pass",
		"wdtt://connect?v=2&host=a&dtls=1&wg=2&local=3&password=x&hashes=h",
		"wdtt://connect?v=1&host=a/b&dtls=1&wg=2&local=3&password=x&hashes=h",
		"wdtt://connect?v=1&host=a&dtls=0&wg=2&local=3&password=x&hashes=h",
	} {
		if _, err := ParseLink(input); err == nil {
			t.Errorf("%q: expected error", input)
		}
	}
}

func TestNormalizeHashes(t *testing.T) {
	got, err := NormalizeHashes("https://vk.ru/call/join/one  two,one\nhttps://vk.com/call/join/three/")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"one", "two", "three"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	if _, err := NormalizeHashes("https://example.com/page"); err == nil {
		t.Fatal("expected error for non-call link")
	}
}
