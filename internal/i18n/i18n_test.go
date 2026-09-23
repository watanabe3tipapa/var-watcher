package i18n

import "testing"

func TestT(t *testing.T) {
	cases := []struct {
		lang, key, want string
	}{
		{"ja", "err_invalid_since", "開始日時の形式が不正です：%s"},
		{"en", "err_invalid_since", "invalid since: %s"},
		{"ja", "err_store_not_configured", "永続化が無効です（store 未設定）"},
		{"en", "err_store_not_configured", "store not configured"},
	}
	for _, c := range cases {
		if got := T(c.lang, c.key); got != c.want {
			t.Errorf("T(%q, %q) = %q, want %q", c.lang, c.key, got, c.want)
		}
	}
}

func TestTArgs(t *testing.T) {
	if got := T("en", "err_invalid_since", "abc"); got != "invalid since: abc" {
		t.Errorf("with args = %q", got)
	}
	if got := T("ja", "err_invalid_since", "abc"); got != "開始日時の形式が不正です：abc" {
		t.Errorf("ja with args = %q", got)
	}
}

func TestTUnknownKey(t *testing.T) {
	if got := T("ja", "no_such_key"); got != "no_such_key" {
		t.Errorf("unknown key = %q", got)
	}
	if got := T("fr", "err_invalid_until"); got != "終了日時の形式が不正です：%s" {
		t.Errorf("unknown lang falls back to ja = %q", got)
	}
}

func TestNormalizeLang(t *testing.T) {
	cases := map[string]string{"ja": "ja", "en": "en", "fr": "ja", "": "ja"}
	for in, want := range cases {
		if got := NormalizeLang(in); got != want {
			t.Errorf("NormalizeLang(%q) = %q, want %q", in, got, want)
		}
	}
}
