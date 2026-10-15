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
		"tray.tooltip":        "CopyApp – ‏%s שולח את הלוח לטלפון",
		"tray.pair":           "צימוד טלפון…",
		"tray.pairTip":        "הצגת קוד ה‑QR",
		"tray.send":           "שליחת הלוח לטלפון",
		"tray.open":           "פתיחת תיקיית הקבצים שהתקבלו",
		"tray.autostart":      "הפעלה עם Windows",
		"tray.repair":         "צימוד מחדש",
		"tray.repairTip":      "שכחת הטלפון הנוכחי",
		"tray.lang":           "שפה / Язык / Language",
		"tray.quit":           "יציאה",
		"toast.notSent":       "לא נשלח",
		"toast.cantSend":      "אי אפשר לשלוח את %s",
		"toast.tooLarge":      "%s גדול מ‑%s",
		"toast.cantRead":      "אי אפשר לקרוא את %s",
		"toast.sent":          "נשלח לטלפון",
		"toast.secret":        "נראה שבלוח יש סיסמה או סוד, אז הוא לא נשלח",
		"toast.nothing":       "אין מה לשלוח",
		"toast.clipEmpty":     "הלוח ריק",
		"toast.clipText":      "טקסט מהלוח",
		"toast.clipImage":     "תמונה מהלוח",
		"toast.outboxFull":    "כבר יש 400 MB שמחכים לטלפון. נסו שוב בעוד כמה דקות",
		"toast.paired":        "הטלפון צומד",
		"toast.unpaired":      "הצימוד לטלפון בוטל",
		"toast.textTitle":     "הועתק מהטלפון",
		"toast.textBody":      "התקבל טקסט מהטלפון (%d תווים)",
		"toast.partTitle":     "נשמר חלקית מהטלפון",
		"toast.partBody":      "%d נשמרו, השאר נכשלו",
		"toast.imageTitle":    "תמונה הועתקה מהטלפון",
		"toast.imageBody":     "הדביקו עם Ctrl+V",
		"toast.savedTitle":    "נשמר מהטלפון",
		"toast.savedMore":     "%s ועוד %d",
		"pair.htmlTitle":      "צימוד הטלפון – CopyApp",
		"pair.lang":           "שפה",
		"pair.qrAlt":          "קוד QR לצימוד",
		"pair.expires":        "הקוד בתוקף עוד {t}",
		"pair.paired":         "הטלפון צומד",
		"pair.close":          "אפשר לסגור את החלון.",
		"pair.expired":        "תוקף הקוד פג",
		"pair.again":          "הצגת קוד חדש",
		"pair.h1":             "צימוד הטלפון",
		"pair.sub":            "שני צעדים, בלי להתקין כלום.",
		"pair.s1":             "<b>סרקו את הקוד</b> במצלמה של הטלפון ופתחו את הקישור.",
		"pair.s2":             "באזהרת הפרטיות הקישו <b>מתקדם</b> › <b>המשך</b>.",
		"pair.beforeTitle":    "לפני הסריקה",
		"pair.b1Named":        "חברו את הטלפון לאותה רשת Wi‑Fi של המחשב: {wifi}",
		"pair.b1":             "חברו את הטלפון לאותה רשת Wi‑Fi שאליה מחובר המחשב.",
		"pair.b2":             "רשת אורחים או „בידוד לקוחות“ (<bdi>AP/client isolation</bdi>) בנתב חוסמים את החיבור. השתמשו ברשת הראשית.",
		"pair.b3":             "הדפדפן יכתוב „החיבור שלך אינו פרטי“. זה צפוי: הקישו <b>מתקדם</b> › <b>המשך</b> (זה המחשב שלכם, ברשת ה‑Wi‑Fi שלכם).",
		"pair.tSum":           "לא מצליחים לפתוח את הדף?",
		"pair.t1":             "<b>„החיבור נדחה“ או שהדף נטען בלי סוף:</b> בדקו את הנקודות למעלה. כש‑Windows שואל על חומת האש, אשרו את CopyApp ברשתות <b>פרטיות וגם ציבוריות</b>. אם רשת ה‑Wi‑Fi מסומנת ב‑Windows כ„ציבורית“, חייבים לאשר גם ציבוריות.",
		"pair.t2":             "<b>עבד קודם, אבל המחשב עבר לרשת אחרת:</b> בתפריט של CopyApp ליד השעון בחרו שוב <b>צימוד טלפון</b>.",
		"pair.t3":             "<b>תקועים ב„אינו פרטי“ ואין כפתור מתקדם:</b> פתחו את הקישור ב‑Chrome.",
		"pair.t4":             "<b>עדיין לא עובד?</b> נסו לכבות לרגע את <b>הנתונים הסלולריים</b> (יש טלפונים ששולחים דרך הרשת הסלולרית כשה‑Wi‑Fi מראה „אין אינטרנט“).",
		"pair.ready":          "זהו. CopyApp נפתח בטלפון ומוכן לשימוש.",
		"pair.carefulSum":     "קוד האישור (למי שרוצה לוודא)",
		"pair.fpLabel":        "קוד האישור",
		"pair.fpNote":         "בטלפון, אותו קוד מופיע ב‑CopyApp תחת <b>⋯</b> › <b>אימות החיבור</b>. אם הקודים שונים, נתקו את הטלפון: ייתכן שמישהו אחר מחובר לרשת שלכם.",
		"pair.fpFull":         "טביעת אצבע SHA‑256 מלאה (באנדרואיד: הגדרות › פרטי כניסה מהימנים › משתמש):",
		"pair.optSum":         "רוצים את CopyApp בתפריט השיתוף? (לא חובה)",
		"pair.opt":            "CopyApp כבר עובד בלשונית של הדפדפן בטלפון. כדי שיופיע בתפריט <b>שיתוף</b> של אנדרואיד ושהאזהרה \"לא מאובטח\" תיעלם, פתחו בטלפון <b>⋯</b> › <b>להפוך לאפליקציה</b> ובצעו את השלבים (כדקה, פעם אחת).",
		"tray.openApp":        "פתיחת CopyApp",
		"tray.sendFile":       "שליחת קובץ לטלפון…",
		"tray.uninstall":      "הסרת ההתקנה…",
		"tray.receiving":      "מקבל את %s… %d%%",
		"tray.sending":        "שולח את %s… %d%%",
		"toast.savedIn":       "%s, בתיקייה %s",
		"ask.autostart":       "להפעיל את CopyApp אוטומטית כש‑Windows עולה?\n\nאפשר לשנות את זה אחר כך בחלון של CopyApp או בתפריט ליד השעון.",
		"un.confirm":          "להסיר את CopyApp מהמחשב?\n\nיוסרו האפליקציה, קיצורי הדרך בשולחן העבודה ובתפריט התחל, הפריט \"שלח אל\" וההפעלה עם Windows.",
		"un.data":             "למחוק גם את ההגדרות ואת נתוני הצימוד של CopyApp?\n\n%s\n\nבחרו \"לא\" כדי לשמור אותם להתקנה מחדש.",
		"un.recv":             "למחוק גם את התיקייה עם הקבצים שהתקבלו מהטלפון?\n\n%s\n\nבחרו \"לא\" כדי לשמור את הקבצים.",
		"hk.fallback":         "תוכנה אחרת כבר משתמשת ב‑Ctrl+Alt+C, לכן CopyApp משתמש ב‑%s כדי לשלוח את הלוח לטלפון.",
		"hk.none":             "תוכנות אחרות כבר משתמשות ב‑Ctrl+Alt+C וב‑Ctrl+Alt+Shift+C. שלחו את הלוח לטלפון מהתפריט ליד השעון.",
		"inst.fail":           "CopyApp לא הצליח להתקין את עצמו (%s). בינתיים הוא פועל מהמיקום הנוכחי.",
		"pick.file":           "בחרו קובץ לשליחה לטלפון",
		"pick.folder":         "בחרו איפה CopyApp ישמור את מה שמגיע מהטלפון",
		"home.htmlTitle":      "CopyApp",
		"home.status":         "מצב",
		"home.phone":          "טלפון",
		"home.paired":         "מצומד",
		"home.notPaired":      "לא מצומד",
		"home.connected":      "מחובר עכשיו",
		"home.lastSeen":       "נראה לאחרונה {t}",
		"home.notSeen":        "לא התחבר מאז ש‑CopyApp הופעל",
		"home.wifi":           "ה‑Wi‑Fi של המחשב",
		"home.noWifi":         "לא מחובר ל‑Wi‑Fi (כבל או לא ידוע)",
		"home.address":        "כתובת",
		"home.hotkey":         "שליחת הלוח של המחשב",
		"home.repair":         "צימוד מחדש",
		"home.transfers":      "העברות פעילות",
		"home.noTransfers":    "אין כרגע העברות.",
		"home.in":             "מהטלפון",
		"home.out":            "לטלפון",
		"home.settings":       "הגדרות",
		"home.language":       "שפה",
		"home.mode":           "מה לעשות עם מה שמגיע מהטלפון",
		"home.modeClip":       "העתקה ללוח",
		"home.modeClipHint":   "טקסט ותמונות נכנסים ללוח, מוכנים ל‑Ctrl+V. קבצים אחרים נשמרים בתיקייה.",
		"home.modeFolder":     "שמירה בתיקייה",
		"home.modeFolderHint": "הכול נשמר כקובץ: טקסט כקובץ txt, תמונות כתמונות.",
		"home.folder":         "תיקיית השמירה",
		"home.choose":         "בחירת תיקייה…",
		"home.openFolder":     "פתיחה",
		"home.max":            "הגודל המרבי",
		"home.maxHint":        "חל בשני הכיוונים. קבצים גדולים נכתבים ישר לדיסק.",
		"home.mb":             "{n} MB",
		"home.gb":             "{n} GB",
		"home.autostart":      "הפעלה עם Windows",
		"home.autostartHint":  "CopyApp נפתח בשקט ליד השעון כשנכנסים למחשב.",
		"home.app":            "האפליקציה",
		"home.uninstall":      "הסרת CopyApp…",
		"home.uninstallAsk":   "ענו על השאלות בחלון שנפתח במחשב.",
		"home.log":            "קובץ יומן:",
		"home.offline":        "CopyApp לא פועל. הפעילו אותו משולחן העבודה או מתפריט התחל.",
	},
	"ru": {
		"tray.tooltip":        "CopyApp – %s отправляет буфер обмена на телефон",
		"tray.pair":           "Подключить телефон…",
		"tray.pairTip":        "Показать QR-код",
		"tray.send":           "Отправить буфер на телефон",
		"tray.open":           "Открыть папку с полученными файлами",
		"tray.autostart":      "Запускать вместе с Windows",
		"tray.repair":         "Подключить заново",
		"tray.repairTip":      "Забыть текущий телефон",
		"tray.lang":           "שפה / Язык / Language",
		"tray.quit":           "Выход",
		"toast.notSent":       "Не отправлено",
		"toast.cantSend":      "Не удалось отправить %s",
		"toast.tooLarge":      "%s больше %s",
		"toast.cantRead":      "Не удалось прочитать %s",
		"toast.sent":          "Отправлено на телефон",
		"toast.secret":        "Похоже, в буфере пароль или секрет — не отправляю",
		"toast.nothing":       "Нечего отправлять",
		"toast.clipEmpty":     "Буфер обмена пуст",
		"toast.clipText":      "Текст из буфера обмена",
		"toast.clipImage":     "Картинка из буфера обмена",
		"toast.outboxFull":    "Телефон уже ждёт 400 МБ. Попробуйте через пару минут",
		"toast.paired":        "Телефон подключён",
		"toast.unpaired":      "Телефон отключён",
		"toast.textTitle":     "Скопировано с телефона",
		"toast.textBody":      "Получен текст с телефона (%d симв.)",
		"toast.partTitle":     "С телефона сохранено не всё",
		"toast.partBody":      "Сохранено: %d, остальное не удалось",
		"toast.imageTitle":    "Картинка с телефона скопирована",
		"toast.imageBody":     "Вставьте её через Ctrl+V",
		"toast.savedTitle":    "Сохранено с телефона",
		"toast.savedMore":     "%s и ещё %d",
		"pair.htmlTitle":      "Подключение телефона – CopyApp",
		"pair.lang":           "Язык",
		"pair.qrAlt":          "QR-код для подключения",
		"pair.expires":        "Код действует ещё {t}",
		"pair.paired":         "Телефон подключён",
		"pair.close":          "Это окно можно закрыть.",
		"pair.expired":        "Срок действия кода истёк",
		"pair.again":          "Показать новый код",
		"pair.h1":             "Подключите телефон",
		"pair.sub":            "Два шага, ничего устанавливать не нужно.",
		"pair.s1":             "<b>Отсканируйте код</b> камерой телефона и откройте ссылку.",
		"pair.s2":             "В предупреждении браузера нажмите <b>Дополнительные</b> › <b>Перейти на сайт</b>.",
		"pair.beforeTitle":    "Перед сканированием",
		"pair.b1Named":        "Подключите телефон к той же сети Wi‑Fi, что и этот компьютер: {wifi}",
		"pair.b1":             "Подключите телефон к той же сети Wi‑Fi, что и этот компьютер.",
		"pair.b2":             "Гостевая сеть или «изоляция клиентов» (AP/client isolation) на роутере блокирует связь. Используйте основную сеть Wi‑Fi.",
		"pair.b3":             "Браузер напишет «Подключение не защищено». Так и должно быть: нажмите <b>Дополнительные</b> › <b>Перейти на сайт</b> (это ваш компьютер в вашей сети Wi‑Fi).",
		"pair.tSum":           "Страница не открывается?",
		"pair.t1":             "<b>«Соединение отклонено» или бесконечная загрузка:</b> проверьте пункты выше. Когда Windows спросит про брандмауэр, разрешите CopyApp в <b>частных и общественных</b> сетях. Если ваша сеть Wi‑Fi отмечена в Windows как «Общедоступная», разрешение для общественных сетей обязательно.",
		"pair.t2":             "<b>Раньше работало, но компьютер сменил сеть:</b> в меню CopyApp у часов снова выберите <b>Подключить телефон</b>.",
		"pair.t3":             "<b>Застряли на «не защищено», а кнопки «Дополнительные» нет:</b> откройте ссылку в Chrome.",
		"pair.t4":             "<b>Всё ещё не работает?</b> Ненадолго выключите <b>мобильный интернет</b> (некоторые телефоны уходят в мобильную сеть, если у Wi‑Fi «нет доступа в интернет»).",
		"pair.ready":          "Вот и всё. CopyApp откроется на телефоне и сразу готов к работе.",
		"pair.carefulSum":     "Код сертификата (для внимательных)",
		"pair.fpLabel":        "Код сертификата",
		"pair.fpNote":         "На телефоне такой же код показан в CopyApp в разделе <b>⋯</b> › <b>Проверить подключение</b>. Если коды разные, отключите телефон: возможно, к вашей сети Wi‑Fi подключился кто-то посторонний.",
		"pair.fpFull":         "Полный отпечаток SHA-256 (на Android: Настройки › Надёжные сертификаты › Пользователь):",
		"pair.optSum":         "Хотите CopyApp в меню «Поделиться»? (необязательно)",
		"pair.opt":            "CopyApp уже работает во вкладке браузера на телефоне. Чтобы он появился в меню <b>Поделиться</b> Android, а предупреждение «Не защищено» пропало, откройте на телефоне <b>⋯</b> › <b>Сделать приложением</b> и выполните шаги (около минуты, один раз).",
		"tray.openApp":        "Открыть CopyApp",
		"tray.sendFile":       "Отправить файл на телефон…",
		"tray.uninstall":      "Удалить…",
		"tray.receiving":      "Получаю %s… %d%%",
		"tray.sending":        "Отправляю %s… %d%%",
		"toast.savedIn":       "%s, в папке %s",
		"ask.autostart":       "Запускать CopyApp автоматически вместе с Windows?\n\nЭто можно изменить позже в окне CopyApp или в меню у часов.",
		"un.confirm":          "Удалить CopyApp с этого компьютера?\n\nБудут удалены приложение, ярлыки на рабочем столе и в меню «Пуск», пункт «Отправить» и автозапуск.",
		"un.data":             "Удалить также настройки и данные подключения CopyApp?\n\n%s\n\nВыберите «Нет», чтобы сохранить их для повторной установки.",
		"un.recv":             "Удалить также папку с файлами, полученными с телефона?\n\n%s\n\nВыберите «Нет», чтобы сохранить файлы.",
		"hk.fallback":         "Ctrl+Alt+C уже занято другой программой, поэтому CopyApp отправляет буфер обмена на телефон по %s.",
		"hk.none":             "Ctrl+Alt+C и Ctrl+Alt+Shift+C заняты другими программами. Отправляйте буфер обмена через меню у часов.",
		"inst.fail":           "CopyApp не удалось установиться (%s). Пока он работает из текущей папки.",
		"pick.file":           "Выберите файл для отправки на телефон",
		"pick.folder":         "Выберите, куда CopyApp сохраняет полученное с телефона",
		"home.htmlTitle":      "CopyApp",
		"home.status":         "Состояние",
		"home.phone":          "Телефон",
		"home.paired":         "Подключён",
		"home.notPaired":      "Не подключён",
		"home.connected":      "На связи сейчас",
		"home.lastSeen":       "Был на связи {t}",
		"home.notSeen":        "Не подключался с запуска CopyApp",
		"home.wifi":           "Wi‑Fi компьютера",
		"home.noWifi":         "Не по Wi‑Fi (кабель или неизвестно)",
		"home.address":        "Адрес",
		"home.hotkey":         "Отправить буфер компьютера",
		"home.repair":         "Подключить заново",
		"home.transfers":      "Текущие передачи",
		"home.noTransfers":    "Сейчас ничего не передаётся.",
		"home.in":             "С телефона",
		"home.out":            "На телефон",
		"home.settings":       "Настройки",
		"home.language":       "Язык",
		"home.mode":           "Что делать с полученным с телефона",
		"home.modeClip":       "Копировать в буфер обмена",
		"home.modeClipHint":   "Текст и картинки попадают в буфер обмена, сразу можно Ctrl+V. Остальные файлы сохраняются в папку.",
		"home.modeFolder":     "Сохранять в папку",
		"home.modeFolderHint": "Всё сохраняется файлами: текст как .txt, картинки как изображения.",
		"home.folder":         "Папка для сохранения",
		"home.choose":         "Выбрать папку…",
		"home.openFolder":     "Открыть",
		"home.max":            "Максимальный размер",
		"home.maxHint":        "Действует в обе стороны. Большие файлы пишутся сразу на диск.",
		"home.mb":             "{n} МБ",
		"home.gb":             "{n} ГБ",
		"home.autostart":      "Запускать вместе с Windows",
		"home.autostartHint":  "CopyApp тихо запускается у часов при входе в систему.",
		"home.app":            "Приложение",
		"home.uninstall":      "Удалить CopyApp…",
		"home.uninstallAsk":   "Ответьте на вопросы в окне, которое открылось на компьютере.",
		"home.log":            "Файл журнала:",
		"home.offline":        "CopyApp не запущен. Запустите его с рабочего стола или из меню «Пуск».",
	},
	"en": {
		"tray.tooltip":        "CopyApp - %s sends the clipboard to your phone",
		"tray.pair":           "Pair phone…",
		"tray.pairTip":        "Show the QR code",
		"tray.send":           "Send clipboard to phone",
		"tray.open":           "Open received folder",
		"tray.autostart":      "Start with Windows",
		"tray.repair":         "Re-pair phone",
		"tray.repairTip":      "Forget the current phone",
		"tray.lang":           "שפה / Язык / Language",
		"tray.quit":           "Quit",
		"toast.notSent":       "Not sent",
		"toast.cantSend":      "Can't send %s",
		"toast.tooLarge":      "%s is larger than %s",
		"toast.cantRead":      "Can't read %s",
		"toast.sent":          "Sent to phone",
		"toast.secret":        "Clipboard looks like a secret; skipped",
		"toast.nothing":       "Nothing to send",
		"toast.clipEmpty":     "The clipboard is empty",
		"toast.clipText":      "Text from clipboard",
		"toast.clipImage":     "Image from clipboard",
		"toast.outboxFull":    "400 MB is already waiting for the phone; try again in a few minutes",
		"toast.paired":        "Phone paired",
		"toast.unpaired":      "Phone unpaired",
		"toast.textTitle":     "Copied from phone",
		"toast.textBody":      "Text received from phone (%d chars)",
		"toast.partTitle":     "Partly saved from phone",
		"toast.partBody":      "%d saved, the rest failed",
		"toast.imageTitle":    "Image copied from phone",
		"toast.imageBody":     "Paste it with Ctrl+V",
		"toast.savedTitle":    "Saved from phone",
		"toast.savedMore":     "%s and %d more",
		"pair.htmlTitle":      "Pair your phone – CopyApp",
		"pair.lang":           "Language",
		"pair.qrAlt":          "Pairing QR code",
		"pair.expires":        "Code expires in {t}",
		"pair.paired":         "Phone paired",
		"pair.close":          "You can close this window.",
		"pair.expired":        "Code expired",
		"pair.again":          "Show a new code",
		"pair.h1":             "Pair your phone",
		"pair.sub":            "Two steps, nothing to install.",
		"pair.s1":             "<b>Scan the code</b> with your phone's camera and open the link.",
		"pair.s2":             "On the privacy warning, tap <b>Advanced</b> › <b>Proceed</b>.",
		"pair.beforeTitle":    "Before you scan",
		"pair.b1Named":        "Connect your phone to the same Wi‑Fi as this PC: {wifi}",
		"pair.b1":             "Connect your phone to the same Wi‑Fi as this PC.",
		"pair.b2":             "A guest Wi‑Fi or “AP/client isolation” on the router blocks the connection. Use the main Wi‑Fi.",
		"pair.b3":             "Your browser will say “Your connection is not private”. That's expected: tap <b>Advanced</b> › <b>Proceed</b> (this is your own PC on your own Wi‑Fi).",
		"pair.tSum":           "Can't open the page?",
		"pair.t1":             "<b>Connection refused or timed out:</b> check the points above. When Windows asks about the firewall, allow CopyApp on <b>private and public</b> networks. If this Wi‑Fi is marked “Public” in Windows, public must be allowed too.",
		"pair.t2":             "<b>Worked before, but the PC changed networks:</b> open the CopyApp tray menu › <b>Pair phone</b> again.",
		"pair.t3":             "<b>Stuck on “not private” with no Advanced button:</b> open the link in Chrome.",
		"pair.t4":             "<b>Still failing?</b> Try turning <b>mobile data off</b> briefly (some phones send traffic over mobile data when the Wi‑Fi shows no internet).",
		"pair.ready":          "That's it. CopyApp opens on your phone, ready to use.",
		"pair.carefulSum":     "Certificate code (for the careful)",
		"pair.fpLabel":        "Certificate code",
		"pair.fpNote":         "Your phone shows the same code in CopyApp under <b>⋯</b> › <b>Verify connection</b>. If they differ, unpair the phone: someone else may be on your Wi‑Fi.",
		"pair.fpFull":         "Full SHA-256 fingerprint (Android shows it under Settings › Trusted credentials › User):",
		"pair.optSum":         "Want a Share-menu entry? (optional)",
		"pair.opt":            "CopyApp already works in your phone's browser tab. To get it in Android's <b>Share</b> menu and drop the “Not secure” warning, open <b>⋯</b> › <b>Make it a real app</b> on the phone and follow the steps (about a minute, once).",
		"tray.openApp":        "Open CopyApp",
		"tray.sendFile":       "Send a file to phone…",
		"tray.uninstall":      "Uninstall…",
		"tray.receiving":      "Receiving %s… %d%%",
		"tray.sending":        "Sending %s… %d%%",
		"toast.savedIn":       "%s, in %s",
		"ask.autostart":       "Start CopyApp automatically when Windows starts?\n\nYou can change this later in the CopyApp window or the tray menu.",
		"un.confirm":          "Uninstall CopyApp from this PC?\n\nThis removes the app, its Desktop and Start menu shortcuts, the \"Send to\" entry and Start with Windows.",
		"un.data":             "Also delete CopyApp's settings and pairing data?\n\n%s\n\nChoose No to keep them for a later reinstall.",
		"un.recv":             "Also delete the folder with the files you received from the phone?\n\n%s\n\nChoose No to keep your files.",
		"hk.fallback":         "Ctrl+Alt+C is already used by another program, so CopyApp uses %s to send the clipboard to your phone.",
		"hk.none":             "Ctrl+Alt+C and Ctrl+Alt+Shift+C are both used by other programs. Use the tray menu to send the clipboard to your phone.",
		"inst.fail":           "CopyApp could not install itself (%s). It runs from where it is for now.",
		"pick.file":           "Choose a file to send to the phone",
		"pick.folder":         "Choose where CopyApp saves items from the phone",
		"home.htmlTitle":      "CopyApp",
		"home.status":         "Status",
		"home.phone":          "Phone",
		"home.paired":         "Paired",
		"home.notPaired":      "Not paired",
		"home.connected":      "Connected now",
		"home.lastSeen":       "Last seen {t}",
		"home.notSeen":        "Not connected since CopyApp started",
		"home.wifi":           "PC Wi‑Fi",
		"home.noWifi":         "Not on Wi‑Fi (cable or unknown)",
		"home.address":        "Address",
		"home.hotkey":         "Send the PC clipboard",
		"home.repair":         "Re-pair phone",
		"home.transfers":      "Active transfers",
		"home.noTransfers":    "Nothing is being transferred right now.",
		"home.in":             "From phone",
		"home.out":            "To phone",
		"home.settings":       "Settings",
		"home.language":       "Language",
		"home.mode":           "What to do with items from the phone",
		"home.modeClip":       "Copy to clipboard",
		"home.modeClipHint":   "Text and pictures go to the clipboard, ready for Ctrl+V. Other files are saved to the folder.",
		"home.modeFolder":     "Save to folder",
		"home.modeFolderHint": "Everything is saved as a file: text as .txt, pictures as images.",
		"home.folder":         "Save folder",
		"home.choose":         "Choose folder…",
		"home.openFolder":     "Open",
		"home.max":            "Largest item",
		"home.maxHint":        "Applies in both directions. Big files go straight to disk.",
		"home.mb":             "{n} MB",
		"home.gb":             "{n} GB",
		"home.autostart":      "Start with Windows",
		"home.autostartHint":  "CopyApp starts quietly in the tray when you sign in.",
		"home.app":            "App",
		"home.uninstall":      "Uninstall CopyApp…",
		"home.uninstallAsk":   "Answer the questions in the window that opened on this PC.",
		"home.log":            "Log file:",
		"home.offline":        "CopyApp is not running. Start it from the Desktop or the Start menu.",
	},
}
