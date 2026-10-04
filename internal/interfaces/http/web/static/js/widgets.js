// Виджеты сервисов на главной странице.
(function () {
  "use strict";

  function byId(id) {
    return document.getElementById(id);
  }

  function money(n) {
    return n.toLocaleString("ru-RU");
  }

  /* 1. Калькулятор аренды */
  (function () {
    var btn = byId("rc-btn");
    if (!btn) return;
    var out = byId("rc-out");
    var rates = {
      desk: { hour: 200, day: 900 },
      cabin2: { hour: 350, day: 2500 },
      cabin4: { hour: 500, day: 4000 },
      room4: { hour: 700, day: 4000 },
      room6: { hour: 1000, day: 6000 },
      room12: { hour: 1800, day: 10000 },
    };

    btn.addEventListener("click", function () {
      var hours = parseInt(byId("rc-hours").value, 10) || 0;
      var days = parseInt(byId("rc-days").value, 10) || 0;
      if (hours < 1 && days < 1) {
        out.textContent = "Укажите часы или дни.";
        return;
      }
      var rate = rates[byId("rc-type").value];
      var projector = byId("rc-projector").checked;
      var total = hours * rate.hour + days * rate.day;
      var parts = [];
      if (hours) {
        parts.push(hours + " ч × " + money(rate.hour) + " ₽");
      }
      if (days) {
        parts.push(days + " дн × " + money(rate.day) + " ₽");
      }
      if (projector && hours) {
        total += hours * 500;
        parts.push("проектор " + money(hours * 500) + " ₽");
      }
      out.innerHTML =
        parts.join(" + ") + " = <strong>" + money(total) + " ₽</strong>";
    });
  })();

  /* 2. Сравнение тарифов */
  (function () {
    var btn = byId("tc-btn");
    if (!btn) return;
    var out = byId("tc-out");

    btn.addEventListener("click", function () {
      var hours = parseInt(byId("tc-hours").value, 10) || 0;
      if (hours < 1 || hours > 80) {
        out.textContent = "Укажите от 1 до 80 часов.";
        return;
      }
      var options = [
        { label: "почасово", cost: hours * 200 },
        { label: "по дням", cost: Math.ceil(hours / 8) * 900 },
        { label: "неделями", cost: Math.ceil(hours / 40) * 3500 },
        { label: "месячный абонемент", cost: 12000 },
      ];
      options.sort(function (a, b) {
        return a.cost - b.cost;
      });
      var best = options[0];
      out.innerHTML =
        "При " +
        hours +
        " ч/нед выгоднее " +
        best.label +
        " — <strong>" +
        money(best.cost) +
        " ₽</strong>";
    });
  })();

  /* 3. Гостевой Wi-Fi */
  (function () {
    var btn = byId("wifi-btn");
    if (!btn) return;
    var out = byId("wifi-out");

    btn.addEventListener("click", function () {
      out.innerHTML =
        "Сеть <strong>MESTO-Guest</strong> · код доступа выдаёт ресепшен или приходит в подтверждении брони. " +
        "Сессия 3 часа, до 50 Мбит/с. Продлить доступ — на ресепшене.";
    });
  })();

  /* 4. Чек-лист подготовки переговорки */
  (function () {
    var list = byId("mc-list");
    if (!list) return;
    var out = byId("mc-out");
    var reset = byId("mc-reset");
    var boxes = Array.prototype.slice.call(
      list.querySelectorAll("input[type=checkbox]"),
    );

    function update() {
      var done = boxes.filter(function (box) {
        return box.checked;
      }).length;
      if (done === boxes.length) {
        out.innerHTML =
          "<strong>Переговорка готова</strong> — все " +
          boxes.length +
          " пунктов отмечены.";
      } else {
        out.textContent =
          "Готово " +
          done +
          " из " +
          boxes.length +
          ". Осталось пунктов: " +
          (boxes.length - done) +
          ".";
      }
    }

    list.addEventListener("change", update);
    reset.addEventListener("click", function () {
      boxes.forEach(function (box) {
        box.checked = false;
      });
      update();
    });
  })();

  /* 5. Таймер до конца брони */
  (function () {
    var btn = byId("tm-btn");
    if (!btn) return;
    var out = byId("tm-out");
    var timerId = null;

    btn.addEventListener("click", function () {
      var minutes = parseInt(byId("tm-min").value, 10) || 0;
      if (minutes < 1 || minutes > 180) {
        out.textContent = "Укажите от 1 до 180 минут.";
        return;
      }
      if (timerId) {
        clearInterval(timerId);
      }
      var left = minutes * 60;
      var render = function () {
        var m = Math.floor(left / 60);
        var s = left % 60;
        out.textContent = "Осталось " + m + ":" + (s < 10 ? "0" : "") + s;
      };
      render();
      timerId = setInterval(function () {
        left -= 1;
        if (left <= 0) {
          clearInterval(timerId);
          out.textContent = "Бронь завершена. Продление — на ресепшене.";
          return;
        }
        render();
      }, 1000);
    });
  })();

  /* 6. Квиз дня */
  (function () {
    var btn = byId("qz-btn");
    if (!btn) return;
    var out = byId("qz-out");
    var answerBtn = byId("qz-answer");
    var questions = [
      {
        q: "Какая минимальная бронь деск-места?",
        a: "Один час — 200 ₽. Дальше день, неделя или месяц с доплатой по разнице тарифов.",
      },
      {
        q: "За сколько отменяют бронь без штрафа?",
        a: "За 2 часа до начала отмена бесплатна, позже — удержание одной оплаченной ставки.",
      },
      {
        q: "Что входит в стоимость деск-места?",
        a: "Стол, стул, быстрый Wi-Fi, кофе и чай, лаунж. Проектор и кофе-брейк — по дополнительному тарифу.",
      },
      {
        q: "Какой депозит у переговорки «Встреча»?",
        a: "2 000 ₽ — возвращаются в течение часа после брони, если переговорка в порядке.",
      },
      {
        q: "Сколько стоит гостевой доступ на день?",
        a: "500 ₽; до 30 минут гость — бесплатно, но его нужно зарегистрировать на ресепшене.",
      },
      {
        q: "Сколько мест в переговорке «Амфитеатр»?",
        a: "12 — это самая большая переговорка «Места».",
      },
    ];
    var current = -1;

    btn.addEventListener("click", function () {
      var next = Math.floor(Math.random() * questions.length);
      if (next === current) {
        next = (next + 1) % questions.length;
      }
      current = next;
      out.textContent = questions[current].q;
      answerBtn.hidden = false;
    });

    answerBtn.addEventListener("click", function () {
      if (current < 0) return;
      out.textContent = questions[current].a;
      answerBtn.hidden = true;
    });
  })();

  /* 7. Кофе-станция */
  (function () {
    var btn = byId("cs-btn");
    if (!btn) return;
    var out = byId("cs-out");
    var brews = [
      "Бразилия Серрадо — шоколад и орех",
      "Эфиопия Иргачефф — цитрус и ягоды",
      "Колумбия Уила — карамель",
      "Коста-Рика Тарраззу — мёд и ваниль",
    ];

    btn.addEventListener("click", function () {
      var brew = brews[Math.floor(Math.random() * brews.length)];
      out.innerHTML =
        "Сегодня варят: <strong>" +
        brew +
        "</strong>. Кофе и чай к деск-месту включены, кофе-брейк на человека — 300 ₽. Кофемашина работает, зерно свежее с утра.";
    });
  })();

  /* 8. Тест «Формат работы» */
  (function () {
    var btn = byId("ft-btn");
    if (!btn) return;
    var out = byId("ft-out");

    btn.addEventListener("click", function () {
      var score =
        parseInt(byId("ft-meets").value, 10) +
        parseInt(byId("ft-noise").value, 10);
      var text;
      if (score <= 3) {
        text = "Деск-место в зоне «Тишина»: минимум отвлечений.";
      } else if (score === 4) {
        text = "Деск-место в зоне «Атриум»: работа и лёгкое общение.";
      } else if (score === 5) {
        text = "Кабинет на 2–3: звонки и встречи без помех.";
      } else {
        text = "Переговорка или лаунж: вам нужно пространство для людей.";
      }
      out.textContent = "Ваш формат — " + text;
    });
  })();

  /* 9. Погода у офиса (демо-данные) */
  (function () {
    var btn = byId("w-btn");
    if (!btn) return;
    var out = byId("w-out");

    btn.addEventListener("click", function () {
      var temp = 14 + Math.floor(Math.random() * 11);
      var wind = 2 + Math.floor(Math.random() * 6);
      var note = Math.random() < 0.25 ? "мокрый снег к вечеру" : "без осадков";
      out.textContent =
        "Сейчас у башни «Око»: +" +
        temp +
        " °C, ветер " +
        wind +
        " м/с, " +
        note +
        " (демо-данные).";
    });
  })();
})();
