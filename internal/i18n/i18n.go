// Package i18n holds the PC-side strings (tray menu, Windows toasts and the
// localhost pairing page) in Hebrew, Russian and English. Hebrew is the
// default. Go strings use fmt verbs; pairing-page strings that the page's
// script fills in use {name} placeholders.
package i18n

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
)

// Langs lists the supported languages; the first is the default.
var Langs = []string{"he", "ru", "en"}

// Default is the language used until the user picks another.
const Default = "he"

// FileName is the file in the data dir that remembers the tray language.
const FileName = "lang.txt"

var cur atomic.Value

func init() { cur.Store(Default) }

// Lang returns the current PC language.
func Lang() string { return cur.Load().(string) }

// Valid reports whether l is a supported language code.
func Valid(l string) bool { _, ok := table[l]; return ok }

// Set changes the current language. Unknown codes are ignored.
func Set(l string) bool {
	if !Valid(l) {
		return false
	}
	cur.Store(l)
	return true
}

// Load reads dir\lang.txt (missing or invalid: Default).
func Load(dir string) {
	b, _ := os.ReadFile(filepath.Join(dir, FileName))
	if !Set(strings.TrimSpace(string(b))) {
		Set(Default)
	}
}

// Save writes the current language to dir\lang.txt.
func Save(dir string) error {
	return os.WriteFile(filepath.Join(dir, FileName), []byte(Lang()), 0o600)
}

// T formats key in the current language.
func T(key string, args ...any) string { return In(Lang(), key, args...) }

// In formats key in lang, falling back to English, then to the key itself.
func In(lang, key string, args ...any) string {
	s, ok := table[lang][key]
	if !ok {
		if s, ok = table["en"][key]; !ok {
			return key
		}
	}
	if len(args) > 0 {
		return fmt.Sprintf(s, args...)
	}
	return s
}

// Dir is "rtl" for Hebrew, "ltr" otherwise.
func Dir(lang string) string {
	if lang == "he" {
		return "rtl"
	}
	return "ltr"
}

// Prefixed returns lang -> key -> string for keys starting with prefix
// (used to hand the pairing page its strings).
func Prefixed(prefix string) map[string]map[string]string {
	out := map[string]map[string]string{}
	for l, m := range table {
		out[l] = map[string]string{}
		for k, v := range m {
			if strings.HasPrefix(k, prefix) {
				out[l][k] = v
			}
		}
	}
	return out
}

var table = map[string]map[string]string{
	"he": {
		"tray.tooltip":     "CopyApp – ‏Ctrl+Alt+C שולח את הלוח לטלפון",
		"tray.pair":        "צימוד טלפון…",
		"tray.pairTip":     "הצגת קוד ה‑QR",
		"tray.send":        "שליחת הלוח לטלפון",
		"tray.open":        "פתיחת תיקיית הקבצים שהתקבלו",
		"tray.autostart":   "הפעלה עם Windows",
		"tray.repair":      "צימוד מחדש",
		"tray.repairTip":   "שכחת הטלפון הנוכחי",
		"tray.lang":        "שפה / Язык / Language",
		"tray.quit":        "יציאה",
		"toast.notSent":    "לא נשלח",
		"toast.cantSend":   "אי אפשר לשלוח את %s",
		"toast.tooLarge":   "%s גדול מ‑200 MB",
		"toast.cantRead":   "אי אפשר לקרוא את %s",
		"toast.sent":       "נשלח לטלפון",
		"toast.secret":     "נראה שבלוח יש סיסמה או סוד, אז הוא לא נשלח",
		"toast.nothing":    "אין מה לשלוח",
		"toast.clipEmpty":  "הלוח ריק",
		"toast.clipText":   "טקסט מהלוח",
		"toast.clipImage":  "תמונה מהלוח",
		"toast.outboxFull": "כבר יש 400 MB שמחכים לטלפון. נסו שוב בעוד כמה דקות",
		"toast.paired":     "הטלפון צומד",
		"toast.unpaired":   "הצימוד לטלפון בוטל",
		"toast.textTitle":  "הועתק מהטלפון",
		"toast.textBody":   "התקבל טקסט מהטלפון (%d תווים)",
		"toast.partTitle":  "נשמר חלקית מהטלפון",
		"toast.partBody":   "%d נשמרו, השאר נכשלו",
		"toast.imageTitle": "תמונה הועתקה מהטלפון",
		"toast.imageBody":  "הדביקו עם Ctrl+V",
		"toast.savedTitle": "נשמר מהטלפון",
		"toast.savedMore":  "%s ועוד %d",
		"pair.htmlTitle":   "צימוד הטלפון – CopyApp",
		"pair.lang":        "שפה",
		"pair.qrAlt":       "קוד QR לצימוד",
		"pair.expires":     "הקוד יפוג בעוד {s} שניות",
		"pair.paired":      "הטלפון צומד",
		"pair.close":       "אפשר לסגור את החלון.",
		"pair.expired":     "תוקף הקוד פג",
		"pair.again":       "הצגת קוד חדש",
		"pair.h1":          "צימוד הטלפון",
		"pair.sub":         "הטלפון והמחשב צריכים להיות מחוברים לאותה רשת Wi‑Fi.",
		"pair.s1":          "<b>סרקו את הקוד</b> במצלמה של הטלפון ופתחו את הקישור.",
		"pair.s2":          "אם הדפדפן כותב שהחיבור אינו פרטי, הקישו <b>מתקדם</b> › <b>המשך</b>. זה צפוי: המחשב נמצא ברשת ה‑Wi‑Fi שלכם.",
		"pair.ready":       "זהו. CopyApp נפתח בטלפון ומוכן לשימוש.",
		"pair.carefulSum":  "קוד האישור (למי שרוצה לוודא)",
		"pair.fpLabel":     "קוד האישור",
		"pair.fpNote":      "בטלפון, אותו קוד מופיע ב‑CopyApp תחת <b>⋯</b> › <b>אימות החיבור</b>. אם הקודים שונים, נתקו את הטלפון: ייתכן שמישהו אחר מחובר לרשת שלכם.",
		"pair.fpFull":      "טביעת אצבע SHA‑256 מלאה (באנדרואיד: הגדרות › פרטי כניסה מהימנים › משתמש):",
		"pair.optSum":      "רוצים את CopyApp בתפריט השיתוף? (לא חובה)",
		"pair.opt":         "CopyApp כבר עובד בלשונית של הדפדפן בטלפון. כדי שיופיע בתפריט <b>שיתוף</b> של אנדרואיד ושהאזהרה \"לא מאובטח\" תיעלם, פתחו בטלפון <b>⋯</b> › <b>להפוך לאפליקציה</b> ובצעו את השלבים (כדקה, פעם אחת).",
	},
	"ru": {
		"tray.tooltip":     "CopyApp – Ctrl+Alt+C отправляет буфер обмена на телефон",
		"tray.pair":        "Подключить телефон…",
		"tray.pairTip":     "Показать QR-код",
		"tray.send":        "Отправить буфер на телефон",
		"tray.open":        "Открыть папку с полученными файлами",
		"tray.autostart":   "Запускать вместе с Windows",
		"tray.repair":      "Подключить заново",
		"tray.repairTip":   "Забыть текущий телефон",
		"tray.lang":        "שפה / Язык / Language",
		"tray.quit":        "Выход",
		"toast.notSent":    "Не отправлено",
		"toast.cantSend":   "Не удалось отправить %s",
		"toast.tooLarge":   "%s больше 200 МБ",
		"toast.cantRead":   "Не удалось прочитать %s",
		"toast.sent":       "Отправлено на телефон",
		"toast.secret":     "Похоже, в буфере пароль или секрет — не отправляю",
		"toast.nothing":    "Нечего отправлять",
		"toast.clipEmpty":  "Буфер обмена пуст",
		"toast.clipText":   "Текст из буфера обмена",
		"toast.clipImage":  "Картинка из буфера обмена",
		"toast.outboxFull": "Телефон уже ждёт 400 МБ. Попробуйте через пару минут",
		"toast.paired":     "Телефон подключён",
		"toast.unpaired":   "Телефон отключён",
		"toast.textTitle":  "Скопировано с телефона",
		"toast.textBody":   "Получен текст с телефона (%d симв.)",
		"toast.partTitle":  "С телефона сохранено не всё",
		"toast.partBody":   "Сохранено: %d, остальное не удалось",
		"toast.imageTitle": "Картинка с телефона скопирована",
		"toast.imageBody":  "Вставьте её через Ctrl+V",
		"toast.savedTitle": "Сохранено с телефона",
		"toast.savedMore":  "%s и ещё %d",
		"pair.htmlTitle":   "Подключение телефона – CopyApp",
		"pair.lang":        "Язык",
		"pair.qrAlt":       "QR-код для подключения",
		"pair.expires":     "Код действует ещё {s} с",
		"pair.paired":      "Телефон подключён",
		"pair.close":       "Это окно можно закрыть.",
		"pair.expired":     "Срок действия кода истёк",
		"pair.again":       "Показать новый код",
		"pair.h1":          "Подключите телефон",
		"pair.sub":         "Телефон и компьютер должны быть в одной сети Wi‑Fi.",
		"pair.s1":          "<b>Отсканируйте код</b> камерой телефона и откройте ссылку.",
		"pair.s2":          "Если браузер пишет, что подключение не защищено, нажмите <b>Дополнительные</b> › <b>Перейти на сайт</b>. Так и должно быть: компьютер находится в вашей собственной сети Wi‑Fi.",
		"pair.ready":       "Вот и всё. CopyApp откроется на телефоне и сразу готов к работе.",
		"pair.carefulSum":  "Код сертификата (для внимательных)",
		"pair.fpLabel":     "Код сертификата",
		"pair.fpNote":      "На телефоне такой же код показан в CopyApp в разделе <b>⋯</b> › <b>Проверить подключение</b>. Если коды разные, отключите телефон: возможно, к вашей сети Wi‑Fi подключился кто-то посторонний.",
		"pair.fpFull":      "Полный отпечаток SHA-256 (на Android: Настройки › Надёжные сертификаты › Пользователь):",
		"pair.optSum":      "Хотите CopyApp в меню «Поделиться»? (необязательно)",
		"pair.opt":         "CopyApp уже работает во вкладке браузера на телефоне. Чтобы он появился в меню <b>Поделиться</b> Android, а предупреждение «Не защищено» пропало, откройте на телефоне <b>⋯</b> › <b>Сделать приложением</b> и выполните шаги (около минуты, один раз).",
	},
	"en": {
		"tray.tooltip":     "CopyApp - Ctrl+Alt+C sends the clipboard to your phone",
		"tray.pair":        "Pair phone…",
		"tray.pairTip":     "Show the QR code",
		"tray.send":        "Send clipboard to phone",
		"tray.open":        "Open received folder",
		"tray.autostart":   "Start with Windows",
		"tray.repair":      "Re-pair phone",
		"tray.repairTip":   "Forget the current phone",
		"tray.lang":        "שפה / Язык / Language",
		"tray.quit":        "Quit",
		"toast.notSent":    "Not sent",
		"toast.cantSend":   "Can't send %s",
		"toast.tooLarge":   "%s is larger than 200 MB",
		"toast.cantRead":   "Can't read %s",
		"toast.sent":       "Sent to phone",
		"toast.secret":     "Clipboard looks like a secret; skipped",
		"toast.nothing":    "Nothing to send",
		"toast.clipEmpty":  "The clipboard is empty",
		"toast.clipText":   "Text from clipboard",
		"toast.clipImage":  "Image from clipboard",
		"toast.outboxFull": "400 MB is already waiting for the phone; try again in a few minutes",
		"toast.paired":     "Phone paired",
		"toast.unpaired":   "Phone unpaired",
		"toast.textTitle":  "Copied from phone",
		"toast.textBody":   "Text received from phone (%d chars)",
		"toast.partTitle":  "Partly saved from phone",
		"toast.partBody":   "%d saved, the rest failed",
		"toast.imageTitle": "Image copied from phone",
		"toast.imageBody":  "Paste it with Ctrl+V",
		"toast.savedTitle": "Saved from phone",
		"toast.savedMore":  "%s and %d more",
		"pair.htmlTitle":   "Pair your phone – CopyApp",
		"pair.lang":        "Language",
		"pair.qrAlt":       "Pairing QR code",
		"pair.expires":     "Code expires in {s} s",
		"pair.paired":      "Phone paired",
		"pair.close":       "You can close this window.",
		"pair.expired":     "Code expired",
		"pair.again":       "Show a new code",
		"pair.h1":          "Pair your phone",
		"pair.sub":         "Phone and PC must be on the same Wi‑Fi.",
		"pair.s1":          "<b>Scan the code</b> with your phone's camera and open the link.",
		"pair.s2":          "If the browser says the connection is not private, tap <b>Advanced</b> › <b>Proceed</b>. This is expected: the PC is on your own Wi‑Fi.",
		"pair.ready":       "That's it. CopyApp opens on your phone, ready to use.",
		"pair.carefulSum":  "Certificate code (for the careful)",
		"pair.fpLabel":     "Certificate code",
		"pair.fpNote":      "Your phone shows the same code in CopyApp under <b>⋯</b> › <b>Verify connection</b>. If they differ, unpair the phone: someone else may be on your Wi‑Fi.",
		"pair.fpFull":      "Full SHA-256 fingerprint (Android shows it under Settings › Trusted credentials › User):",
		"pair.optSum":      "Want a Share-menu entry? (optional)",
		"pair.opt":         "CopyApp already works in your phone's browser tab. To get it in Android's <b>Share</b> menu and drop the “Not secure” warning, open <b>⋯</b> › <b>Make it a real app</b> on the phone and follow the steps (about a minute, once).",
	},
}
