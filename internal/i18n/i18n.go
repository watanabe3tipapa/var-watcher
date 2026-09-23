package i18n

import "fmt"

var messages = map[string]map[string]string{
	"ja": {
		"err_invalid_since":        "開始日時の形式が不正です：%s",
		"err_invalid_until":        "終了日時の形式が不正です：%s",
		"err_store_not_configured": "永続化が無効です（store 未設定）",
		"frontend_not_embedded":    "フロントエンドが同梱されていません（make frontend で再ビルドしてください）",
	},
	"en": {
		"err_invalid_since":        "invalid since: %s",
		"err_invalid_until":        "invalid until: %s",
		"err_store_not_configured": "store not configured",
		"frontend_not_embedded":    "frontend not embedded; run `make frontend` and rebuild",
	},
}

// NormalizeLang はサポート言語のみに正規化する。未知の値は "ja"。
func NormalizeLang(l string) string {
	if l == "en" {
		return "en"
	}
	return "ja"
}

// T は lang の辞書から key に対応する文言を返す。未知キー・未知言語は日本語→キー名の順にフォールバック。
func T(lang, key string, args ...any) string {
	m, ok := messages[lang]
	if !ok {
		m = messages["ja"]
	}
	msg, ok := m[key]
	if !ok {
		msg = messages["ja"][key]
		if !ok {
			return key
		}
	}
	if len(args) == 0 {
		return msg
	}
	return fmt.Sprintf(msg, args...)
}
