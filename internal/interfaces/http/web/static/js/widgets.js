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
      room12: { hour: 1800, day: 10000 }
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
      out.innerHTML = parts.join(" + ") + " = <strong>" + money(total) + " ₽</strong>";
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
        { label: "месячный абонемент", cost: 12000 }
      ];
      options.sort(function (a, b) {
        return a.cost - b.cost;
      });
      var best = options[0];
      out.innerHTML =
        "При " + hours + " ч/нед выгоднее " + best.label +
        " — <strong>" + money(best.cost) + " ₽</strong>";
    });
  })();

  /* 3. Свободно сейчас (демо-данные) */
  (function () {
    var btn = byId("free-btn");
    if (!btn) return;
    var out = byId("free-out");
    var zones = [
      { name: "Окна", total: 6 },
      { name: "Атриум", total: 6 },
      { name: "Тишина", total: 6 },
      { name: "Терраса", total: 6 }
    ];

    btn.addEventListener("click", function () {
      var free = 0;
      var parts = zones.map(function (zone) {
        var f = 1 + Math.floor(Math.random() * zone.total);
        free += f;
        return zone.name + " " + f + "/" + zone.total;
      });
      out.textContent =
        parts.join(" · ") + ". Всего свободно " + free + " из 24 (демо-данные).";
    });
  })();

  /* 4. Ближайшая переговорка */
  (function () {
    var btn = byId("nr-btn");
    if (!btn) return;
    var out = byId("nr-out");
    var rooms = [
      { name: "«Встреча»", cap: 4, price: 700 },
      { name: "«Проект»", cap: 6, price: 1000 },
      { name: "«Амфитеатр»", cap: 12, price: 1800 }
    ];

    btn.addEventListener("click", function () {
      var people = parseInt(byId("nr-people").value, 10) || 0;
      if (people < 1) {
        out.textContent = "Укажите число участников.";
        return;
      }
      var room = rooms.filter(function (r) {
        return r.cap >= people;
      })[0];
      if (!room) {
        out.textContent = "Для такой компании подойдёт зал на мероприятие — спросите на ресепшене.";
        return;
      }
      out.innerHTML =
        "Ближе всего: " + room.name + " — " + room.cap +
        " мест, <strong>" + money(room.price) + " ₽/час</strong>.";
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

  /* 6. Стол дня */
  (function () {
    var btn = byId("dd-btn");
    if (!btn) return;
    var out = byId("dd-out");
    var desks = [
      "Деск 1 · зона «Окна», у кабинета",
      "Деск 3 · зона «Окна», вид на Москва-Сити",
      "Деск 7 · зона «Тишина», у стены",
      "Деск 9 · зона «Атриум», центр",
      "Деск 12 · зона «Атриум», у колонны",
      "Деск 16 · зона «Тишина», у окна",
      "Деск 19 · зона «Терраса», у лаунжа",
      "Деск 24 · зона «Терраса», в углу"
    ];

    btn.addEventListener("click", function () {
      var desk = desks[Math.floor(Math.random() * desks.length)];
      out.textContent = "Попробуйте: " + desk + ". Свободен — проверьте на ресепшене.";
    });
  })();

  /* 7. Загрузка офиса по часам */
  (function () {
    var wrap = byId("load-bars");
    if (!wrap) return;
    var hours = [8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20];
    var load = [15, 30, 50, 70, 90, 85, 75, 65, 55, 45, 35, 25, 15];

    hours.forEach(function (hour, i) {
      var bar = document.createElement("span");
      bar.className = "load-bar";
      bar.style.height = Math.max(8, load[i]) + "%";
      bar.title = hour + ":00 — загрузка " + load[i] + "%";
      wrap.appendChild(bar);
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
        "Сейчас у башни «Око»: +" + temp + " °C, ветер " + wind +
        " м/с, " + note + " (демо-данные).";
    });
  })();
})();