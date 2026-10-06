// Виджеты сервисов на главной странице.
// Каждый виджет — интеграция с внешним готовым сервисом стороннего
// разработчика: погода (wttr.in), курсы ЦБ (cbr-xml-daily.ru),
// перевод (MyMemory), QR-коды (goqr.me), рабочий день (isdayoff.ru),
// время в филиалах (timeapi.io), обратная связь (FormSubmit).
// Подходы разные: JSON через fetch, простой текст, картинка, POST-форма.
(function () {
  "use strict";

  function byId(id) {
    return document.getElementById(id);
  }

  // Число с двумя знаками и разделителем тысяч: 84.9309 → «84,93».
  function money2(n) {
    return Number(n).toLocaleString("ru-RU", {
      minimumFractionDigits: 2,
      maximumFractionDigits: 2,
    });
  }

  function pad(n) {
    return (n < 10 ? "0" : "") + n;
  }

  function escapeHtml(value) {
    return String(value)
      .replace(/&/g, "&amp;")
      .replace(/</g, "&lt;")
      .replace(/>/g, "&gt;");
  }

  /* 1. Погода у офиса — wttr.in (JSON, без ключа) */
  (function () {
    var btn = byId("w-btn");
    if (!btn) return;
    var out = byId("w-out");

    btn.addEventListener("click", function () {
      out.textContent = "Загружаем прогноз…";
      fetch("https://wttr.in/Moscow?format=j1&lang=ru")
        .then(function (r) {
          if (!r.ok) throw new Error("HTTP " + r.status);
          return r.json();
        })
        .then(function (data) {
          var c = data.current_condition[0];
          var desc =
            c.lang_ru && c.lang_ru[0]
              ? c.lang_ru[0].value
              : c.weatherDesc[0].value;
          out.innerHTML =
            "Сейчас у башни «Око»: <strong>" +
            c.temp_C +
            " °C</strong>, ощущается как " +
            c.FeelsLikeC +
            " °C, ветер " +
            c.windspeedKmph +
            " км/ч (" +
            c.winddir16Point +
            "), " +
            escapeHtml(desc.toLowerCase()) +
            ".";
        })
        .catch(function () {
          out.textContent = "Погода недоступна — сервис wttr.in не ответил.";
        });
    });
  })();

  /* 2. Курс валют — cbr-xml-daily.ru (JSON ЦБ РФ, без ключа) */
  (function () {
    var btn = byId("cur-btn");
    if (!btn) return;
    var out = byId("cur-out");

    btn.addEventListener("click", function () {
      out.textContent = "Загружаем курсы…";
      fetch("https://www.cbr-xml-daily.ru/daily_json.js")
        .then(function (r) {
          if (!r.ok) throw new Error("HTTP " + r.status);
          return r.json();
        })
        .then(function (data) {
          var parts = ["USD", "EUR", "CNY"].map(function (code) {
            var v = data.Valute[code];
            return v.Name + " — <strong>" + money2(v.Value) + " ₽</strong>";
          });
          out.innerHTML = "Курсы ЦБ РФ: " + parts.join("; ") + ".";
        })
        .catch(function () {
          out.textContent = "Курсы недоступны — сервис ЦБ не ответил.";
        });
    });
  })();

  /* 3. Переводчик RU↔EN — MyMemory (JSON, без ключа) */
  (function () {
    var btn = byId("tr-btn");
    if (!btn) return;
    var out = byId("tr-out");

    btn.addEventListener("click", function () {
      var text = byId("tr-text").value.trim();
      var dir = byId("tr-dir").value;
      if (!text) {
        out.textContent = "Введите фразу для перевода.";
        return;
      }
      out.textContent = "Переводим…";
      var url =
        "https://api.mymemory.translated.net/get?q=" +
        encodeURIComponent(text) +
        "&langpair=" +
        encodeURIComponent(dir);
      fetch(url)
        .then(function (r) {
          if (!r.ok) throw new Error("HTTP " + r.status);
          return r.json();
        })
        .then(function (data) {
          var translated =
            data.responseData && data.responseData.translatedText;
          if (!translated) throw new Error("нет перевода");
          out.textContent = "Перевод: " + translated;
        })
        .catch(function () {
          out.textContent = "Перевод недоступен — сервис MyMemory не ответил.";
        });
    });
  })();

  /* 4. QR-код — goqr.me (картинка, без ключа) */
  (function () {
    var btn = byId("qr-btn");
    if (!btn) return;
    var out = byId("qr-out");

    btn.addEventListener("click", function () {
      var text = byId("qr-text").value.trim();
      if (!text) {
        out.textContent = "Введите текст или ссылку для QR-кода.";
        return;
      }
      var src =
        "https://api.qrserver.com/v1/create-qr-code/?size=140x140&data=" +
        encodeURIComponent(text);
      out.innerHTML =
        '<img class="widget-qr" width="140" height="140" alt="QR-код" src="' +
        src +
        '"><span class="widget-qr-note">QR-код готов — наведите камеру телефона.</span>';
    });
  })();

  /* 5. Рабочий день или выходной — isdayoff.ru (простой текст, без ключа) */
  (function () {
    var btn = byId("wd-btn");
    if (!btn) return;
    var out = byId("wd-out");

    btn.addEventListener("click", function () {
      out.textContent = "Проверяем…";
      fetch("https://isdayoff.ru/today")
        .then(function (r) {
          if (!r.ok) throw new Error("HTTP " + r.status);
          return r.text();
        })
        .then(function (code) {
          var t = code.trim();
          var text;
          if (t === "0") {
            text =
              "Сегодня рабочий день — офис работает по обычному графику (8:00–22:00).";
          } else if (t === "1") {
            text =
              "Сегодня нерабочий день — офис закрыт или работает по праздничному графику.";
          } else if (t === "2") {
            text =
              "Сегодня сокращённый рабочий день — уточните график на ресепшене.";
          } else if (t === "4") {
            text = "Сегодня выходной — офис работает по праздничному графику.";
          } else {
            text = "Не удалось распознать статус дня (код «" + t + "»).";
          }
          out.innerHTML = "<strong>" + text + "</strong>";
        })
        .catch(function () {
          out.textContent =
            "Не удалось проверить день — сервис isdayoff.ru не ответил.";
        });
    });
  })();

  /* 6. Время в филиалах — timeapi.io (JSON, без ключа) */
  (function () {
    var btn = byId("wt-btn");
    if (!btn) return;
    var out = byId("wt-out");

    var zones = [
      { label: "Москва", zone: "Europe/Moscow" },
      { label: "Пекин", zone: "Asia/Shanghai" },
      { label: "Лондон", zone: "Europe/London" },
    ];

    btn.addEventListener("click", function () {
      out.textContent = "Загружаем время…";
      Promise.all(
        zones.map(function (z) {
          return fetch(
            "https://timeapi.io/api/Time/current/zone?timeZone=" +
              encodeURIComponent(z.zone),
          )
            .then(function (r) {
              if (!r.ok) throw new Error("HTTP " + r.status);
              return r.json();
            })
            .then(function (d) {
              return z.label + " — " + pad(d.hour) + ":" + pad(d.minute);
            });
        }),
      )
        .then(function (rows) {
          out.innerHTML = rows.join("<br>");
        })
        .catch(function () {
          out.textContent = "Время недоступно — сервис timeapi.io не ответил.";
        });
    });
  })();

  /* 7. Форма обратной связи — FormSubmit (POST на чужой бэкенд) */
  (function () {
    var form = byId("fb-form");
    if (!form) return;
    var out = byId("fb-out");
    var btn = byId("fb-btn");

    // Адрес получателя заявок: замените на свою почту.
    // Первая отправка на новый адрес активируется письмом от FormSubmit
    // (одна кнопка в письме) — после этого форма работает без настройки.
    var ENDPOINT = "https://formsubmit.co/ajax/CHANGE_ME@example.com";

    form.addEventListener("submit", function (event) {
      event.preventDefault();
      btn.disabled = true;
      out.textContent = "Отправляем…";

      fetch(ENDPOINT, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Accept: "application/json",
        },
        body: JSON.stringify({
          name: byId("fb-name").value,
          email: byId("fb-email").value,
          message: byId("fb-message").value,
          _subject: "Заявка с сайта «Место»",
        }),
      })
        .then(function (r) {
          return r.json().catch(function () {
            return {};
          });
        })
        .then(function (data) {
          if (String(data.success) === "true") {
            out.textContent =
              "Спасибо! Заявка отправлена — ответим на вашу почту.";
            form.reset();
          } else if (
            data.message &&
            data.message.indexOf("Activation") !== -1
          ) {
            out.textContent =
              "Форма ещё не активирована: подтвердите адрес по ссылке в письме от FormSubmit.";
          } else {
            out.textContent =
              "Не удалось отправить. Проверьте поля и попробуйте снова.";
          }
        })
        .catch(function () {
          out.textContent =
            "Не удалось отправить — сервис FormSubmit недоступен.";
        })
        .then(function () {
          btn.disabled = false;
        });
    });
  })();
})();
