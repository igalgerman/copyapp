/* CopyApp website — vanilla JS, no dependencies. */
(function () {
  "use strict";
  var doc = document, root = doc.documentElement;
  var mqReduce = window.matchMedia("(prefers-reduced-motion: reduce)");
  var mqDark = window.matchMedia("(prefers-color-scheme: dark)");
  var reduced = mqReduce.matches;
  function $(s, c) { return (c || doc).querySelector(s); }
  function $$(s, c) { return Array.prototype.slice.call((c || doc).querySelectorAll(s)); }
  function store(k, v) { try { if (v === undefined) return localStorage.getItem(k); localStorage.setItem(k, v); } catch (e) { return null; } }

  /* ------------------------------------------------------------------ i18n */
  var DICT = {
    en: {
      phonePath: "On Android: start in your browser. A text tile is also available as a development preview.",
      androidTitle: "Choose how to send from your phone",
      androidLead: "The browser is the standard route for every content type. The optional Android preview sends clipboard text to your PC.",
      browserBadge: "Available now · 1.0.2",
      browserTitle: "Browser & PWA",
      browserBody: "Text, photos and files in both directions. Scan the PC’s QR code and open CopyApp in Chrome on your phone, on the same Wi‑Fi.",
      browserStep1: "No APK is needed to start: use the page that opens on your phone.",
      browserStep2: "To send to the PC, choose Send clipboard, Photo or File. To receive on the phone, keep the page open and tap Copy or Save when needed.",
      browserStep3: "Want to share from Gallery or WhatsApp? Installing the PWA and trusting the PC certificate adds a Share target where supported by your phone.",
      browserAction: "Start with the browser",
      companionBadge: "For testing · Development preview",
      companionTitle: "Android tile for sending text",
      companionBody: "Copy text or a link in another app, open Quick Settings and tap <bdi dir='ltr'>CopyApp: Send to PC</bdi>. A small visible window reads the clipboard and sends it to the PC.",
      companionStep1: "Requires the new Windows development build too; the 1.0.2 download on this page does not support APK pairing.",
      companionStep2: "Pair once by scanning a QR code and comparing the full certificate fingerprint. Then add the tile to Quick Settings.",
      companionStep3: "Text and links only, from phone to PC. Use the browser for photos, files and the other direction. There is no automatic clipboard monitoring.",
      companionNote: "Experimental: not yet tested on a physical Samsung, and no stable APK download yet. The test file on GitHub may require sign-in and ZIP extraction. Updating may require reinstalling and pairing again.",
      companionAction: "Android checks & test APK",
      companionWindowsAction: "Windows checks & test builds",
      companionGuide: "Instructions & matching Windows build",
      shareOptionTitle: "CopyApp in Android’s Share menu",
      shareOptionBody: "To add the Share-menu entry: on the phone open ⋯ → Install as an app, compare the certificate code with the PC, install the certificate as instructed, then choose Install app in Chrome. After installation, choose CopyApp in Android’s Share menu and confirm sending to the PC. The ordinary browser tab remains available without installation.",
      knownTitle: "Before you download — 1.0.2",
      knownBody: "The Windows executable is unsigned. Pair only on a trusted network: bypassing the certificate warning does not verify the PC’s identity. Certificate menus, app installation and Share support vary by device and need checking on your phone. iPhone is unsupported; a changed PC address requires opening its new address.",
      knownTransfers: "Android downloads use Chrome’s download manager; browser permissions may block automatic actions. Files normally go to <bdi dir='ltr'>Downloads\\CopyApp</bdi>. If that location is unsafe, CopyApp uses the local <bdi dir='ltr'>%LOCALAPPDATA%\\CopyApp\\Received</bdi> folder. The PC window shows the actual destination.",
      knownLink: "Security and further limits",
      metaTitle: "CopyApp — From your PC to your phone. And back.",
      metaDesc: "CopyApp moves text, photos and files between a Windows PC and an Android phone over your home Wi‑Fi. No cloud, no account. Free and open source.",
      skip: "Skip to content", navLabel: "Main", navHow: "How it works", navFeatures: "Features", navVideos: "Videos", navTrust: "Privacy", navFaq: "FAQ",
      themeToggle: "Switch light or dark theme", headerCta: "Download",
      title1: "From your PC to your phone.", title2: "And back.",
      lead: "Text, photos and files move between Windows and Android over your home Wi‑Fi — with no account and no relay server.",
      download: "Download for Windows", free: "Free", watchHow: "Watch how it works",
      stStatus: "Connected · Phone", stClipboard: "Clipboard", stSnippet: "“Meet at 7:30, Dizengoff 50”", stReceived: "From your phone",
      stSendClip: "Send clipboard", stPhoto: "Photo", stFile: "File", stWifi: "Home Wi‑Fi",
      trustLabel: "CopyApp in short", t1: "No cloud", t2: "No account", t3: "Your Wi‑Fi only", t4: "Encrypted", t5: "Free & open source",
      howTitle: "Three steps. About a minute.",
      howLead: "Start in your phone’s browser, with no account or installation. The text-sending tile is a separate development preview.",
      s1t: "Run the exe", s1b: "Download <bdi>copyapp.exe</bdi> and double‑click it. It installs itself for your Windows user, adds a shortcut and opens a window with a QR code.",
      s2t: "Scan the QR", s2b: "Your phone’s camera opens CopyApp in the browser. Nothing to install on the phone — just be on the same Wi‑Fi.",
      s3t: "Copy & paste across", s3b: "From the phone: send the clipboard, a photo or a file. From the PC: use the active sending shortcut or the Send clipboard button in CopyApp. When content appears on the phone, tap Copy or Save as needed.",
      howVideo: "Prefer to watch? See the install video",
      fTitle: "Everything you need. Nothing you don’t.", fTextItem: "Clipboard text",
      f1t: "Files up to 2 GB", f1b: "Text, images and big files, with progress, speed and time left. Large files go straight to disk.",
      f3t: "One shortcut", f3b: "The default is <bdi>Ctrl+Alt+C</bdi>, with <bdi>Ctrl+Alt+Shift+C</bdi> as a fallback. You can also send from the PC window. Keep CopyApp open on the phone to receive.",
      fClip: "Clipboard", fFolder: "Folder", fClipOut: "Pasted ✓",
      f2t: "Clipboard or folder", f2b: "Choose where things land, on both sides. Your choice is remembered.",
      f4t: "In the Share menu", f4b: "Optional: install CopyApp as an app on your phone and send from any app with Share.",
      f5t: "Speaks your language", f5b: "The Windows and browser interfaces speak Hebrew, English or Russian. The experimental Android companion is in English.",
      fStartQ: "Start with Windows?", f6t: "Starts with Windows — if you want", f6b: "CopyApp asks instead of deciding for you. Change it anytime.",
      unitMB: "MB", f7t: "Tiny and private", f7b: "One Windows file. No ads and no relay server for transfers.",
      vTitle: "See it work", vTab1: "Install & pair", vTab2: "Using it",
      v1cap: "Version 1.0.2 — installation and browser pairing walkthrough.", vOpen: "Open the video on its own",
      v2cap: "Version 1.0.2 — text, a photo and a file through the PC and phone browser.",
      vNote: "These English tutorials show the version 1.0.2 browser flow. Some Android and Windows screens are labeled illustrations; the new APK is not shown. Four Hebrew tutorials recorded on real devices are not ready yet.",
      netLabel: "The internet", homeLabel: "Your Wi‑Fi", pairChip: "Pairing code valid for",
      secTitle: "Built for trust", secLead: "Your PC and phone transfer content directly inside your own network. Builds configured for Sentry send content-free diagnostic events to that service.",
      sec1t: "Encrypted traffic", sec1b: "Everything between PC and phone travels over HTTPS, with a certificate created on your own PC.",
      sec2t: "One‑time pairing code", sec2b: "The QR code works once and expires after 10 minutes. Only the phone that scanned it gets paired.",
      sec3t: "Local network only", sec3b: "CopyApp doesn’t open your PC to the internet or relay through a server. Outside your Wi‑Fi it simply doesn’t work.",
      sec4t: "Your files stay yours", sec4b: "Received files go to <bdi>Downloads\\CopyApp</bdi>, or a folder you choose. Unpair the phone anytime.",
      sec5t: "Open source", sec5b: "All the code is on GitHub: read it, check it, build it yourself.", sec5link: "View the source",
      faqTitle: "Questions people ask",
      q1: "Is it really free?", a1: "Yes. No subscription, no ads, no pro tier. The code is open source under the MIT license.",
      q2: "Do I need to install anything on the phone?", a2: "Not for everyday browser use: scan the QR code and open the page. The optional PWA adds a shortcut and Share target where supported. The text-sending tile is a separate development APK; browser photo and file transfers do not need it.",
      q3: "Why does Windows or Chrome show a warning?",
      a3a: "The exe isn’t code‑signed yet, so Windows may say “Windows protected your PC”. Click <b>More info → Run anyway</b>. If the firewall asks, click <b>Allow</b>.",
      a3b: "On the phone, Chrome warns because the certificate was created on the PC. On your own trusted network, tap <b>Advanced → Proceed</b> to use the browser tab. Bypassing the warning does not verify the PC’s identity; compare the certificate code before the optional certificate installation.",
      q4: "Does it work outside my home Wi‑Fi?", a4: "No, by design. PC and phone must be on the same Wi‑Fi. Guest networks or “client isolation” on the router block the connection, so use your main network.",
      q5: "What about iPhone?", a5: "Not yet. CopyApp targets Windows 10/11 and Android with Chrome. Certificate/app installation and Share-menu support vary by phone and browser.",
      q6: "What about large files?", a6: "Up to 2 GB per file by default, with a progress bar. Large files are written straight to disk, so they don’t fill your memory. You can change the limit in the CopyApp window on your PC.",
      q7: "How do I uninstall it?", a7: "Open the CopyApp window — from the shortcut or the icon near the clock — and choose <b>Uninstall</b>. Files you received stay where they are.",
      q8: "Is it safe? Is it really open source?", a8: "The full code is on GitHub. Content moves directly over HTTPS inside your local network, with one-time pairing. Builds configured for Sentry send diagnostic events there without clipboard or file contents. Compare the PC certificate when pairing and the download’s SHA‑256 on the release page.",
      ctaTitle: "Two minutes from now, it just works.", ctaLead: "One download, one scan, done.", ghStar: "Star on GitHub",
      sha: "Want to verify the file? Its SHA‑256 checksum is on", shaLink: "the release page",
      tagline: "Fewer steps. More connection.", privacy: "This site has no analytics, cookies or trackers. Language and theme preferences stay in your browser.",
      source: "Source code", releases: "Releases", license: "MIT license"
    },
    ru: {
      phonePath: "На Android начните с браузера. Плитка для текста доступна в тестовой версии.",
      androidTitle: "Как отправлять с телефона",
      androidLead: "Браузер — основной способ для всех типов содержимого. Отдельная тестовая версия Android отправляет текст из буфера на компьютер.",
      browserBadge: "Доступно сейчас · 1.0.2",
      browserTitle: "Браузер и PWA",
      browserBody: "Текст, фото и файлы в обе стороны. Отсканируйте QR-код компьютера и откройте CopyApp в Chrome на телефоне в той же сети Wi‑Fi.",
      browserStep1: "Для начала APK не нужен: пользуйтесь страницей, открывшейся на телефоне.",
      browserStep2: "Для отправки на компьютер выберите буфер, фото или файл. Для приёма на телефоне держите страницу открытой и при необходимости нажмите «Копировать» или «Сохранить».",
      browserStep3: "Хотите делиться из Галереи или WhatsApp? Установка PWA и доверие сертификату компьютера добавляют пункт в меню «Поделиться», если телефон это поддерживает.",
      browserAction: "Начать в браузере",
      companionBadge: "Для тестирования · Версия в разработке",
      companionTitle: "Плитка Android для отправки текста",
      companionBody: "Скопируйте текст или ссылку в другом приложении, откройте быстрые настройки и нажмите <bdi dir='ltr'>CopyApp: Send to PC</bdi>. Небольшое видимое окно прочитает буфер и отправит текст на компьютер.",
      companionStep1: "Нужна также новая тестовая сборка Windows; версия 1.0.2 на этой странице не поддерживает подключение APK.",
      companionStep2: "Один раз отсканируйте QR-код и сравните полный отпечаток сертификата. Затем добавьте плитку в быстрые настройки.",
      companionStep3: "Только текст и ссылки с телефона на компьютер. Для фото, файлов и обратного направления используйте браузер. Автоматического слежения за буфером нет.",
      companionNote: "Экспериментальная версия: ещё не проверена на физическом Samsung, стабильного APK для загрузки пока нет. Тестовый файл в GitHub может потребовать входа и распаковки ZIP. При обновлении могут понадобиться переустановка и повторное подключение.",
      companionAction: "Проверки Android и тестовый APK",
      companionWindowsAction: "Проверки Windows и тестовые сборки",
      companionGuide: "Инструкция и подходящая сборка Windows",
      shareOptionTitle: "CopyApp в меню «Поделиться» Android",
      shareOptionBody: "Чтобы добавить пункт в меню «Поделиться», на телефоне откройте ⋯ → Установить как приложение, сравните код сертификата с компьютером, установите сертификат по инструкции, затем выберите «Установить приложение» в Chrome. После установки выберите CopyApp в меню «Поделиться» и подтвердите отправку на компьютер. Обычная вкладка работает без установки.",
      knownTitle: "Перед загрузкой — 1.0.2",
      knownBody: "Файл Windows не подписан. Подключайте телефон только в доверенной сети: обход предупреждения сертификата не подтверждает личность компьютера. Меню сертификатов, установка приложения и поддержка «Поделиться» зависят от устройства и требуют проверки на вашем телефоне. iPhone не поддерживается; при смене адреса компьютера нужно открыть новый адрес.",
      knownTransfers: "На Android скачиванием управляет менеджер загрузок Chrome; разрешения браузера могут блокировать автоматические действия. Обычно файлы попадают в <bdi dir='ltr'>Downloads\\CopyApp</bdi>. Если папка небезопасна, используется локальная <bdi dir='ltr'>%LOCALAPPDATA%\\CopyApp\\Received</bdi>. Окно компьютера показывает фактическую папку.",
      knownLink: "Безопасность и другие ограничения",
      metaTitle: "CopyApp — С компьютера на телефон. И обратно.",
      metaDesc: "CopyApp передаёт текст, фото и файлы между ПК с Windows и телефоном на Android через домашний Wi‑Fi. Без облака и аккаунта. Бесплатно, с открытым кодом.",
      skip: "К содержимому", navLabel: "Навигация", navHow: "Как это работает", navFeatures: "Возможности", navVideos: "Видео", navTrust: "Безопасность", navFaq: "Вопросы",
      themeToggle: "Светлая или тёмная тема", headerCta: "Скачать",
      title1: "С компьютера на телефон.", title2: "И обратно.",
      lead: "Текст, фото и файлы передаются между Windows и Android через домашний Wi‑Fi — без аккаунта и сервера-посредника.",
      download: "Скачать для Windows", free: "Бесплатно", watchHow: "Посмотреть, как это работает",
      stStatus: "Подключено · Телефон", stClipboard: "Буфер обмена", stSnippet: "«Встречаемся в 19:30, Дизенгоф 50»", stReceived: "С телефона",
      stSendClip: "Отправить буфер", stPhoto: "Фото", stFile: "Файл", stWifi: "Домашний Wi‑Fi",
      trustLabel: "CopyApp коротко", t1: "Без облака", t2: "Без аккаунта", t3: "Только ваш Wi‑Fi", t4: "Шифрование", t5: "Бесплатно, открытый код",
      howTitle: "Три шага. Примерно минута.",
      howLead: "Начните с браузера на телефоне — без аккаунта и установки. Плитка для отправки текста — отдельная тестовая версия.",
      s1t: "Запустите exe", s1b: "Скачайте <bdi>copyapp.exe</bdi> и дважды щёлкните по нему. Программа установится для вашего пользователя, создаст ярлык и откроет окно с QR‑кодом.",
      s2t: "Отсканируйте QR", s2b: "Камера телефона откроет CopyApp в браузере. На телефон ничего ставить не нужно — только тот же Wi‑Fi.",
      s3t: "Копируйте в обе стороны", s3b: "С телефона: буфер обмена, фото или файл. С компьютера: действующее сочетание клавиш или кнопка отправки в окне CopyApp. Когда содержимое появится на телефоне, при необходимости нажмите «Копировать» или «Сохранить».",
      howVideo: "Удобнее посмотреть? Видео установки",
      fTitle: "Всё нужное. Ничего лишнего.", fTextItem: "Текст из буфера",
      f1t: "Файлы до 2 ГБ", f1b: "Текст, картинки и большие файлы — с прогрессом, скоростью и оставшимся временем. Большие файлы сразу записываются на диск.",
      f3t: "Одно сочетание клавиш", f3b: "По умолчанию <bdi>Ctrl+Alt+C</bdi>, запасной вариант — <bdi>Ctrl+Alt+Shift+C</bdi>. Можно отправить и из окна компьютера. Для приёма держите CopyApp открытым на телефоне.",
      fClip: "В буфер", fFolder: "В папку", fClipOut: "Вставлено ✓",
      f2t: "В буфер или в папку", f2b: "Выберите, куда всё попадает, — на обеих сторонах. Выбор запоминается.",
      f4t: "В меню «Поделиться»", f4b: "По желанию: установите CopyApp как приложение на телефон и отправляйте из любого приложения через «Поделиться».",
      f5t: "Говорит на вашем языке", f5b: "Интерфейсы Windows и браузера — на иврите, английском или русском. Тестовая версия Android — на английском.",
      fStartQ: "Запускать с Windows?", f6t: "Запуск с Windows — если хотите", f6b: "CopyApp спрашивает, а не решает за вас. Можно изменить в любой момент.",
      unitMB: "МБ", f7t: "Маленький и приватный", f7b: "Один файл для Windows. Без рекламы и сервера-посредника для передачи.",
      vTitle: "Посмотрите в деле", vTab1: "Установка и подключение", vTab2: "Как пользоваться",
      v1cap: "Версия 1.0.2 — установка и подключение в браузере.", vOpen: "Открыть видео отдельно",
      v2cap: "Версия 1.0.2 — текст, фото и файл через компьютер и браузер телефона.",
      vNote: "Эти видео на английском показывают работу через браузер в версии 1.0.2. Некоторые экраны Android и Windows — помеченные иллюстрации; нового APK в видео нет. Четыре ролика на иврите с записью реальных устройств пока не готовы.",
      netLabel: "Интернет", homeLabel: "Ваш Wi‑Fi", pairChip: "Код подключения действует ещё",
      secTitle: "Построено на доверии", secLead: "Компьютер и телефон передают содержимое напрямую в вашей сети. Сборки с настроенным Sentry отправляют туда диагностические события без содержимого буфера.",
      sec1t: "Шифрование", sec1b: "Всё между компьютером и телефоном идёт по HTTPS, с сертификатом, созданным на вашем компьютере.",
      sec2t: "Одноразовый код", sec2b: "QR‑код срабатывает один раз и истекает через 10 минут. Подключается только телефон, который его отсканировал.",
      sec3t: "Только локальная сеть", sec3b: "CopyApp не открывает компьютер для интернета и не использует сервер‑посредник. Вне вашего Wi‑Fi он просто не работает.",
      sec4t: "Файлы остаются у вас", sec4b: "Полученные файлы сохраняются в <bdi>Downloads\\CopyApp</bdi> или в выбранную папку. Телефон можно отключить в любой момент.",
      sec5t: "Открытый код", sec5b: "Весь код на GitHub: читайте, проверяйте, собирайте сами.", sec5link: "Посмотреть код",
      faqTitle: "Частые вопросы",
      q1: "Это правда бесплатно?", a1: "Да. Без подписки, рекламы и платной версии. Код открыт под лицензией MIT.",
      q2: "Нужно что‑то ставить на телефон?", a2: "Для обычной работы в браузере — нет: отсканируйте QR-код и откройте страницу. Необязательная PWA добавляет ярлык и пункт «Поделиться», если поддерживается. Плитка отправки текста — отдельный тестовый APK; для фото и файлов в браузере он не нужен.",
      q3: "Почему Windows или Chrome показывают предупреждение?",
      a3a: "У файла пока нет цифровой подписи, поэтому Windows может написать «Система Windows защитила ваш компьютер». Нажмите <b>Подробнее → Выполнить в любом случае</b>. Если спросит брандмауэр — <b>Разрешить</b>.",
      a3b: "На телефоне Chrome предупреждает, потому что сертификат создан на компьютере. Только в своей доверенной сети нажмите <b>Дополнительные → Перейти</b>. Обход предупреждения не подтверждает личность компьютера; перед необязательной установкой сертификата сравните его код с компьютером.",
      q4: "Работает вне домашнего Wi‑Fi?", a4: "Нет, и это намеренно. Компьютер и телефон должны быть в одной сети Wi‑Fi. Гостевая сеть или «изоляция клиентов» в роутере блокируют соединение — используйте основную сеть.",
      q5: "А iPhone?", a5: "Пока нет. CopyApp предназначен для Windows 10/11 и Android с Chrome. Установка сертификата, приложения и меню «Поделиться» зависят от телефона и браузера.",
      q6: "А большие файлы?", a6: "До 2 ГБ на файл по умолчанию, с индикатором прогресса. Большие файлы сразу записываются на диск и не занимают память. Лимит можно изменить в окне CopyApp на компьютере.",
      q7: "Как удалить?", a7: "Откройте окно CopyApp — ярлыком или значком рядом с часами — и выберите <b>Удалить</b>. Полученные файлы останутся на месте.",
      q8: "Это безопасно? Код правда открыт?", a8: "Весь код на GitHub. Содержимое передаётся напрямую по HTTPS в локальной сети с одноразовым подключением. Сборки с настроенным Sentry отправляют туда диагностические события без содержимого буфера и файлов. Сравнивайте сертификат компьютера при подключении и SHA‑256 загрузки на странице релиза.",
      ctaTitle: "Через две минуты всё уже работает.", ctaLead: "Одна загрузка, одно сканирование — и готово.", ghStar: "Звезда на GitHub",
      sha: "Хотите проверить файл? Контрольная сумма SHA‑256 указана", shaLink: "на странице релиза",
      tagline: "Меньше шагов. Больше связи.", privacy: "На сайте нет аналитики, cookie и трекеров. Настройки языка и темы сохраняются в вашем браузере.",
      source: "Исходный код", releases: "Релизы", license: "Лицензия MIT"
    }
  };
  /* Strings used only by the animated demo. */
  var STAGE = {
    he: { copied: "הועתק ללוח", copiedPc: "הועתק ללוח במחשב", received: "התקבל בטלפון", fromPc: "מהמחשב", fromPhone: "מהטלפון", shop: "קניות: חלב, לחם, קפה", text: "טקסט" },
    en: { copied: "Copied to clipboard", copiedPc: "Copied to PC clipboard", received: "Received on phone", fromPc: "From PC", fromPhone: "From phone", shop: "Shopping: milk, bread, coffee", text: "Text" },
    ru: { copied: "Скопировано в буфер", copiedPc: "Скопировано в буфер ПК", received: "Получено на телефоне", fromPc: "С компьютера", fromPhone: "С телефона", shop: "Купить: молоко, хлеб, кофе", text: "Текст" }
  };

  /* Hebrew is authored in the HTML; capture it so we can switch back. */
  var HE = {};
  $$("[data-i18n]").forEach(function (el) { HE[el.dataset.i18n] = el.tagName === "TITLE" ? el.textContent : el.innerHTML; });
  $$("[data-i18n-aria]").forEach(function (el) { HE[el.dataset.i18nAria] = el.getAttribute("aria-label"); });
  $$("[data-i18n-content]").forEach(function (el) { HE[el.dataset.i18nContent] = el.getAttribute("content"); });
  $$("[data-i18n-svg]").forEach(function (el) { HE[el.dataset.i18nSvg] = el.textContent; });
  DICT.he = HE;

  var lang = "he";
  function t(k) { return (DICT[lang] && DICT[lang][k]) || HE[k] || ""; }
  function s(k) { return STAGE[lang][k]; }

  function setLang(l, persist) {
    if (!DICT[l]) l = "he";
    lang = l;
    root.lang = l;
    root.dir = l === "he" ? "rtl" : "ltr";
    $$("[data-i18n]").forEach(function (el) {
      var v = t(el.dataset.i18n);
      if (el.tagName === "TITLE") { doc.title = v; } else if (el.innerHTML !== v) { el.innerHTML = v; }
    });
    $$("[data-i18n-aria]").forEach(function (el) { el.setAttribute("aria-label", t(el.dataset.i18nAria)); });
    $$("[data-i18n-content]").forEach(function (el) { el.setAttribute("content", t(el.dataset.i18nContent)); });
    $$("[data-i18n-svg]").forEach(function (el) { el.textContent = t(el.dataset.i18nSvg); el.setAttribute("direction", root.dir); });
    var homeLbl = $(".lbl-home"); if (homeLbl) homeLbl.setAttribute("x", root.dir === "rtl" ? "376" : "44");
    $$("[data-lang]").forEach(function (b) { b.setAttribute("aria-pressed", String(b.dataset.lang === l)); });
    if (persist) store("copyapp.lang", l);
    root.classList.remove("i18n-pending");
    placeTabPill();
    if (stage) stage.reset();
  }
  $$("[data-lang]").forEach(function (b) {
    b.addEventListener("click", function () {
      if (b.dataset.lang === lang) return;
      setLang(b.dataset.lang, true);
      var live = $("#live"); if (live) live.textContent = b.textContent;
    });
  });

  /* ----------------------------------------------------------------- theme */
  function effectiveDark() {
    var a = root.getAttribute("data-theme");
    return a ? a === "dark" : mqDark.matches;
  }
  function syncTheme() { root.classList.toggle("is-dark", effectiveDark()); }
  syncTheme();
  if (mqDark.addEventListener) mqDark.addEventListener("change", syncTheme);
  var themeBtn = $(".theme-toggle");
  themeBtn.addEventListener("click", function () {
    var next = effectiveDark() ? "light" : "dark";
    root.setAttribute("data-theme", next);
    store("copyapp.theme", next);
    syncTheme();
  });

  /* ---------------------------------------------------------------- header */
  var header = $(".site-header"), ticking = false;
  function onScroll() {
    if (ticking) return; ticking = true;
    requestAnimationFrame(function () {
      ticking = false;
      var y = window.scrollY;
      header.classList.toggle("scrolled", y > 24);
      if (stage) stage.scrollParallax(y);
    });
  }
  window.addEventListener("scroll", onScroll, { passive: true });

  /* ------------------------------------------------------- reveal and live */
  var hasIO = "IntersectionObserver" in window;
  function revealAll() { $$(".reveal").forEach(function (el) { el.classList.add("in"); }); }
  if (!hasIO) { revealAll(); }
  else {
    var revealIO = new IntersectionObserver(function (entries) {
      var batch = entries.filter(function (e) { return e.isIntersecting; });
      batch.forEach(function (e, i) {
        e.target.style.setProperty("--i", reduced ? 0 : Math.min(i, 6));
        e.target.classList.add("in");
        revealIO.unobserve(e.target);
      });
    }, { rootMargin: "0px 0px -8% 0px", threshold: 0.12 });
    $$(".reveal").forEach(function (el) { revealIO.observe(el); });

    /* "live" = in view, so looping micro-animations only run when seen. */
    var liveIO = new IntersectionObserver(function (entries) {
      entries.forEach(function (e) {
        e.target.classList.toggle("live", e.isIntersecting);
        if (e.isIntersecting) e.target.classList.add("in");
        var fn = e.target._onLive; if (fn) fn(e.isIntersecting);
      });
    }, { threshold: 0.25 });
    $$(".tile, .step, .steps-wrap, .trust-visual").forEach(function (el) { liveIO.observe(el); });
  }

  /* ------------------------------------------------------------ count-ups */
  function countUp(el, to, dec, dur) {
    if (reduced) { el.textContent = to.toFixed(dec); return; }
    var t0 = performance.now();
    (function step(now) {
      var p = Math.min(1, (now - t0) / dur), e = 1 - Math.pow(1 - p, 3);
      el.textContent = (to * e).toFixed(dec);
      if (p < 1) requestAnimationFrame(step);
    })(t0);
  }
  var tiny = $(".tile-tiny");
  if (tiny) tiny._onLive = function (on) {
    var c = $(".cnt", tiny);
    if (on && !c._done) { c._done = true; countUp(c, 11, 0, 1400); }
  };
  if (!hasIO && tiny) $(".cnt", tiny).textContent = "11";

  /* -------------------------------------------------------- files demo */
  (function () {
    var tile = $(".tile-files"); if (!tile) return;
    var bar = $(".pbar i", tile), pct = $(".cnt-pct", tile), gb = $(".cnt", tile);
    var raf = 0, t0 = 0, CYCLE = 5200, HOLD = 1400;
    function paint(p) { bar.style.setProperty("--p", p.toFixed(3)); pct.textContent = Math.round(p * 100); gb.textContent = (1.4 * p).toFixed(1); }
    function frame(now) {
      var el = (now - t0) % (CYCLE + HOLD);
      var p = Math.min(1, el / CYCLE); p = 1 - Math.pow(1 - p, 1.6);
      paint(p); raf = requestAnimationFrame(frame);
    }
    if (reduced || !hasIO) { paint(0.72); return; }
    paint(0);
    tile._onLive = function (on) {
      if (on && !raf) { t0 = performance.now(); raf = requestAnimationFrame(frame); }
      else if (!on && raf) { cancelAnimationFrame(raf); raf = 0; }
    };
  })();

  /* Toggle demos: destination + start-with-Windows. */
  function toggler(sel, cls, every) {
    var tile = $(sel); if (!tile) return;
    var demo = $(".demo", tile), id = 0;
    if (reduced || !hasIO) { if (cls === "on") demo.classList.add("on"); return; }
    tile._onLive = function (on) {
      if (on && !id) id = setInterval(function () { demo.classList.toggle(cls); }, every);
      else if (!on && id) { clearInterval(id); id = 0; }
    };
  }
  toggler(".tile-dest", "b", 2400);
  toggler(".tile-start", "on", 1900);

  /* ----------------------------------------------------- pairing countdown */
  (function () {
    var vis = $(".trust-visual"), cd = $(".countdown"); if (!vis || !cd) return;
    var left = 600, id = 0;
    function show() { cd.textContent = Math.floor(left / 60) + ":" + String(left % 60).padStart(2, "0"); }
    var svg = $(".net", vis);
    if (reduced) { try { svg.pauseAnimations(); svg.setCurrentTime(0.9); } catch (e) {} return; }
    vis._onLive = function (on) {
      try { on ? svg.unpauseAnimations() : svg.pauseAnimations(); } catch (e) {}
      if (on && !id) id = setInterval(function () { left = left > 0 ? left - 1 : 600; show(); }, 1000);
      else if (!on && id) { clearInterval(id); id = 0; }
    };
  })();

  /* ------------------------------------------------------------- video tabs */
  var tabs = $$('[role="tab"]'), pill = $(".tab-pill");
  function placeTabPill() {
    if (!pill) return;
    var sel = $('[role="tab"][aria-selected="true"]');
    if (!sel || !sel.offsetWidth) return;
    pill.style.width = sel.offsetWidth + "px";
    pill.style.transform = "translateX(" + sel.offsetLeft + "px)";
  }
  function selectTab(name, focus) {
    tabs.forEach(function (tb) {
      var on = tb.dataset.tab === name;
      tb.setAttribute("aria-selected", String(on));
      tb.tabIndex = on ? 0 : -1;
      var panel = doc.getElementById(tb.getAttribute("aria-controls"));
      if (on) {
        if (panel.hidden) { panel.hidden = false; if (!reduced) { panel.classList.remove("enter"); void panel.offsetWidth; panel.classList.add("enter"); } }
        var v = $("video", panel); if (v && v.preload === "none") v.preload = "metadata";
        if (focus) tb.focus();
      } else {
        panel.hidden = true;
        var pv = $("video", panel); if (pv && !pv.paused) pv.pause();
      }
    });
    placeTabPill();
  }
  tabs.forEach(function (tb, i) {
    tb.addEventListener("click", function () { selectTab(tb.dataset.tab); });
    tb.addEventListener("keydown", function (e) {
      var k = e.key, n = tabs.length, j = -1;
      var fwd = root.dir === "rtl" ? "ArrowLeft" : "ArrowRight", back = root.dir === "rtl" ? "ArrowRight" : "ArrowLeft";
      if (k === fwd) j = (i + 1) % n; else if (k === back) j = (i - 1 + n) % n; else if (k === "Home") j = 0; else if (k === "End") j = n - 1;
      if (j >= 0) { e.preventDefault(); selectTab(tabs[j].dataset.tab, true); }
    });
  });
  $$("[data-video]").forEach(function (a) { a.addEventListener("click", function () { selectTab(a.dataset.video); }); });
  window.addEventListener("resize", placeTabPill);
  if (doc.fonts && doc.fonts.ready) doc.fonts.ready.then(placeTabPill);
  /* If a video file is missing, keep the designed placeholder and hide the dead controls. */
  $$(".vbox video").forEach(function (v) {
    var src = $("source", v);
    if (src) src.addEventListener("error", function () { v.closest(".vbox").classList.add("missing"); v.removeAttribute("controls"); });
  });

  /* ------------------------------------------------------------ FAQ accordion */
  $$(".acc details").forEach(function (d) {
    var sum = $("summary", d), body = $(".ans", d), anim = null;
    sum.addEventListener("click", function (e) {
      if (reduced || !body.animate) return; // native toggle
      e.preventDefault();
      if (anim) anim.cancel();
      if (!d.open) {
        d.open = true;
        var h = body.scrollHeight;
        anim = body.animate([{ height: "0px", opacity: 0 }, { height: h + "px", opacity: 1 }], { duration: 380, easing: "cubic-bezier(.2,.75,.2,1)" });
        anim.onfinish = function () { anim = null; };
      } else {
        var h2 = body.offsetHeight;
        anim = body.animate([{ height: h2 + "px", opacity: 1 }, { height: "0px", opacity: 0 }], { duration: 300, easing: "cubic-bezier(.4,0,.2,1)" });
        anim.onfinish = function () { d.open = false; anim = null; };
      }
    });
  });

  /* --------------------------------------------------------------- hero stage */
  var stage = (function () {
    var el = $("#stage"); if (!el) return null;
    var inner = $(".stage-in", el), layer = $("#flyers"), pc = $(".pc", el), phone = $(".phone", el), wifi = $(".wifi-tag", el);
    var pcFeed = $("#pcFeed"), phoneFeed = $("#phoneFeed"), pcClip = $("#pcClip"), keys = $$("#keys kbd");
    var pcToast = $("#pcToast"), phoneToast = $("#phoneToast");
    var visible = true, gen = 0, waiters = [];
    var px = 0, py = 0, sy = 0;

    function running() { return visible && !doc.hidden && !reduced; }
    function gate() { return running() ? Promise.resolve() : new Promise(function (r) { waiters.push(r); }); }
    function wake() { if (running()) { var w = waiters; waiters = []; w.forEach(function (r) { r(); }); } }
    function sleep(ms) { return new Promise(function (r) { setTimeout(r, ms); }); }

    function feedItem(kind, name, sub) {
      var li = doc.createElement("li");
      var th = doc.createElement("span"); th.className = "thumb " + kind;
      if (kind === "file") th.textContent = "MP4";
      if (kind === "text") th.textContent = "Aa";
      var nm = doc.createElement("span"); nm.className = "nm"; nm.textContent = name;
      var sm = doc.createElement("small"); sm.textContent = sub; nm.appendChild(sm);
      var ck = doc.createElement("span"); ck.className = "ck"; ck.textContent = "✓";
      li.appendChild(th); li.appendChild(nm); li.appendChild(ck);
      return li;
    }
    function addFeed(feed, li, max) {
      li.classList.add("in");
      feed.insertBefore(li, feed.firstChild);
      var items = $$("li:not(.out)", feed);
      if (items.length > max) {
        var old = items[items.length - 1]; old.classList.add("out");
        setTimeout(function () { old.remove(); }, 400);
      }
    }
    function toast(elT, msg) {
      $("span", elT).textContent = msg;
      elT.classList.add("show");
      clearTimeout(elT._t);
      elT._t = setTimeout(function () { elT.classList.remove("show"); }, 1700);
    }
    function card(kind, title, sub, ring) {
      var c = doc.createElement("div"); c.className = "fly-card " + kind;
      if (kind === "text") { c.textContent = title; return c; }
      var th = doc.createElement("span"); th.className = "thumb " + kind; if (kind === "file") th.textContent = "MP4";
      var tx = doc.createElement("span"); tx.textContent = title; tx.dir = "ltr";
      var sm = doc.createElement("small"); sm.textContent = sub; tx.appendChild(sm);
      c.appendChild(th); c.appendChild(tx);
      if (ring) {
        var r = doc.createElement("span"); r.className = "ring";
        r.innerHTML = '<svg viewBox="0 0 36 36"><circle class="tr" cx="18" cy="18" r="15.9" pathLength="100"/><circle class="pr" cx="18" cy="18" r="15.9" pathLength="100"/></svg><span>0</span>';
        c.appendChild(r);
      }
      return c;
    }
    function center(node, L, mode) {
      var r = node.getBoundingClientRect();
      return { x: r.left + r.width / 2 - L.left, y: (mode === "top" ? r.top + Math.min(26, r.height / 2) : r.top + r.height / 2) - L.top };
    }
    function fly(from, to, c, dur, back, myGen) {
      var L = inner.getBoundingClientRect();
      var a = center(from, L), b = center(to, L, "top");
      var f = doc.createElement("div"); f.className = "flyer"; f.appendChild(c); layer.appendChild(f);
      var w = f.offsetWidth, h = f.offsetHeight;
      var peak = Math.min(a.y, b.y) - L.height * 0.22, mx = (a.x + b.x) / 2, rot = back ? -5 : 5;
      function tr(x, y, sc, r) { return "translate(" + (x - w / 2) + "px," + (y - h / 2) + "px) rotate(" + (r || 0) + "deg) scale(" + sc + ")"; }
      var anim = f.animate([
        { transform: tr(a.x, a.y, 0.4), opacity: 0 },
        { transform: tr(a.x, a.y - 10, 1), opacity: 1, offset: 0.14 },
        { transform: tr(mx, peak, 1.06, rot), opacity: 1, offset: 0.52 },
        { transform: tr(b.x, b.y, 0.92), opacity: 1, offset: 0.9 },
        { transform: tr(b.x, b.y, 0.55), opacity: 0 }
      ], { duration: dur, easing: "cubic-bezier(.45,.05,.35,1)", fill: "forwards" });
      var ring = $(".ring", c);
      if (ring) {
        var pr = $(".pr", ring), lbl = $("span", ring), t0 = performance.now();
        (function tick(now) {
          if (!f.isConnected) return;
          var p = Math.max(0, Math.min(1, ((now - t0) / dur - 0.12) / 0.76));
          pr.style.strokeDashoffset = String(100 - p * 100); lbl.textContent = Math.round(p * 100);
          if (p < 1) requestAnimationFrame(tick);
        })(t0);
      }
      el.classList.add("is-moving"); el.classList.toggle("dir-back", !!back);
      return anim.finished.then(function () { f.remove(); el.classList.remove("is-moving"); return myGen === gen; }, function () { f.remove(); return false; });
    }
    function press(node, ms) { node.classList.add("is-press"); return sleep(ms).then(function () { node.classList.remove("is-press"); }); }

    var steps = [
      function (g) { // PC -> phone, text via Ctrl+Alt+C
        $("span", pcClip).textContent = t("stSnippet");
        keys.forEach(function (k, i) { setTimeout(function () { k.classList.add("on"); }, i * 130); });
        pcClip.classList.add("is-send");
        return sleep(650).then(function () {
          keys.forEach(function (k) { k.classList.remove("on"); }); pcClip.classList.remove("is-send");
          return fly(pcClip, phoneFeed, card("text", t("stSnippet")), 1500, false, g);
        }).then(function (ok) {
          if (!ok) return;
          addFeed(phoneFeed, feedItem("text", t("stSnippet"), s("fromPc")), 3);
          toast(phoneToast, s("copied"));
          return sleep(1300);
        });
      },
      function (g) { // phone -> PC, photo
        return press($("#paPhoto"), 380).then(function () {
          return fly($("#paPhoto"), pcFeed, card("photo", "beach.jpg", "3.2 MB", true), 1700, true, g);
        }).then(function (ok) {
          if (!ok) return;
          addFeed(pcFeed, feedItem("photo", "beach.jpg", "3.2 MB · " + s("fromPhone")), 2);
          toast(pcToast, s("copiedPc"));
          return sleep(1300);
        });
      },
      function (g) { // PC -> phone, big file with progress
        return sleep(200).then(function () {
          return fly($("#keys"), phoneFeed, card("file", "trip-video.mp4", "1.4 GB", true), 2600, false, g);
        }).then(function (ok) {
          if (!ok) return;
          addFeed(phoneFeed, feedItem("file", "trip-video.mp4", "1.4 GB · " + s("fromPc")), 3);
          toast(phoneToast, s("received"));
          return sleep(1300);
        });
      },
      function (g) { // phone -> PC, clipboard text lands in the PC clipboard
        return press($("#paClip"), 380).then(function () {
          return fly($("#paClip"), pcClip, card("text", s("shop")), 1500, true, g);
        }).then(function (ok) {
          if (!ok) return;
          var sp = $("span", pcClip); sp.textContent = s("shop");
          pcClip.classList.add("is-send"); setTimeout(function () { pcClip.classList.remove("is-send"); }, 400);
          toast(pcToast, s("copiedPc"));
          return sleep(1800);
        });
      }
    ];

    function staticScene() {
      pcFeed.innerHTML = ""; phoneFeed.innerHTML = "";
      pcFeed.appendChild(feedItem("photo", "beach.jpg", "3.2 MB · " + s("fromPhone")));
      phoneFeed.appendChild(feedItem("file", "trip-video.mp4", "1.4 GB · " + s("fromPc")));
      phoneFeed.appendChild(feedItem("text", t("stSnippet"), s("fromPc")));
      $("span", phoneToast).textContent = s("copied"); phoneToast.classList.add("show");
    }

    function loop(g) {
      var i = 0;
      (function next() {
        if (g !== gen) return;
        gate().then(function () {
          if (g !== gen) return;
          return steps[i](g);
        }).then(function () {
          if (g !== gen) return;
          i = (i + 1) % steps.length;
          next();
        });
      })();
    }
    function reset() {
      gen++;
      layer.innerHTML = ""; el.classList.remove("is-moving");
      $("span", pcClip).textContent = t("stSnippet");
      [pcToast, phoneToast].forEach(function (x) { x.classList.remove("show"); });
      if (reduced) { staticScene(); return; }
      pcFeed.innerHTML = ""; phoneFeed.innerHTML = "";
      var g = gen; setTimeout(function () { loop(g); }, 900);
    }

    /* Parallax: pointer (fine pointers only) + gentle scroll drift. */
    function applyParallax() {
      if (reduced) { pc.style.transform = phone.style.transform = wifi.style.transform = ""; el.style.transform = ""; return; }
      pc.style.transform = "translate(" + (px * -8) + "px," + (py * -6) + "px)";
      phone.style.transform = "translate(" + (px * 12) + "px," + (py * 9) + "px) rotate(" + (4 + px * 1.5) + "deg)";
      wifi.style.transform = "translate(" + (px * 5) + "px," + (py * 10) + "px) rotate(-3deg)";
      el.style.transform = "translateY(" + (sy * 0.06).toFixed(1) + "px)";
    }
    var hero = $(".hero");
    if (window.matchMedia("(pointer: fine)").matches) {
      hero.addEventListener("pointermove", function (e) {
        var r = hero.getBoundingClientRect();
        px = (e.clientX - r.left) / r.width * 2 - 1; py = (e.clientY - r.top) / r.height * 2 - 1;
        requestAnimationFrame(applyParallax);
      });
      hero.addEventListener("pointerleave", function () { px = py = 0; requestAnimationFrame(applyParallax); });
    }
    if (hasIO) new IntersectionObserver(function (en) { visible = en[0].isIntersecting; wake(); }, { threshold: 0.05 }).observe(el);
    doc.addEventListener("visibilitychange", wake);

    return {
      reset: reset,
      wake: wake,
      scrollParallax: function (y) { if (y < 1200) { sy = y; applyParallax(); } }
    };
  })();

  function setReduced(on) {
    reduced = on; root.classList.toggle("reduced", on);
    if (stage) { stage.reset(); stage.wake(); }
  }
  if (mqReduce.addEventListener) mqReduce.addEventListener("change", function (e) { setReduced(e.matches); });
  root.classList.toggle("reduced", reduced);

  /* ------------------------------------------------------------------ boot */
  var saved = store("copyapp.lang");
  setLang(saved === "en" || saved === "ru" ? saved : "he", false);
  onScroll();
  requestAnimationFrame(function () { $(".hero").classList.add("loaded"); });
  $$(".hero-copy > :not(h1)").forEach(function (n, i) { n.style.setProperty("--i", i); });
})();
