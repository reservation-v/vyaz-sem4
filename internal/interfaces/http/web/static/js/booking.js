// Страница «Бронирование»:
// - живой расчёт стоимости по пространству, тарифу и длительности;
// - периодные тарифы (день/неделя/месяц) только для деск-мест;
// - длительность почасовой брони ограничена закрытием офиса (22:00);
// - формы ввода, редактирования, отмены и поиска отправляются через fetch
//   и заменяют блок приложения без перезагрузки страницы, чтобы не уезжать
//   в начало страницы.
(function () {
  "use strict";

  var app = document.getElementById("booking-app");
  if (!app) {
    return;
  }

  var PERIOD_PRICES = { day: 900, week: 3500, month: 12000 };
  var PERIOD_LABELS = { day: "1 день", week: "7 дней", month: "1 месяц" };
  var CLOSE_HOUR = 22;
  var MAX_HOURS = 8;

  function formatMoney(value) {
    return String(value).replace(/\B(?=(\d{3})+(?!\d))/g, " ");
  }

  // Пересобирает форму под выбранный тариф и пространство.
  function syncRateUI(form) {
    var rate = form.querySelector("#bf-rate");
    var space = form.querySelector("#bf-space");
    var hourRow = form.querySelector("#bf-hour-row");
    var periodNote = form.querySelector("#bf-period-note");
    if (!rate || !hourRow) {
      return;
    }

    var spaceOption = space && space.options[space.selectedIndex];
    var spaceType = spaceOption ? spaceOption.getAttribute("data-type") : "";
    var isDesk = spaceType === "desk";
    var isHour = rate.value === "hour";

    // Периодные тарифы доступны только деск-местам.
    Array.prototype.forEach.call(rate.options, function (option) {
      var isPeriod = option.value !== "hour";
      option.disabled = isPeriod && !isDesk;
      if (option.disabled && rate.value === option.value) {
        rate.value = "hour";
        isHour = true;
      }
    });

    hourRow.hidden = !isHour;
    if (periodNote) {
      periodNote.hidden = isHour;
    }

    if (isHour) {
      limitDurationToClosing(form);
    }

    updateTotal(form);
  }

  // Не даёт выбрать почасовую бронь, выходящую за 22:00.
  function limitDurationToClosing(form) {
    var start = form.querySelector("#bf-start");
    var duration = form.querySelector("#bf-duration");
    if (!start || !duration) {
      return;
    }

    var startHour = parseInt(start.value, 10) || minStartHour();
    var maxHours = Math.min(MAX_HOURS, CLOSE_HOUR - startHour);
    if (maxHours < 1) {
      maxHours = 1;
    }

    Array.prototype.forEach.call(duration.options, function (option) {
      var hours = parseInt(option.value, 10) || 0;
      option.disabled = hours > maxHours;
      if (option.disabled && parseInt(duration.value, 10) === hours) {
        duration.value = String(maxHours);
      }
    });
  }

  function minStartHour() {
    var first = app.querySelector("#bf-start option");
    return first ? parseInt(first.value, 10) || 8 : 8;
  }

  function updateTotal(form) {
    var rate = form.querySelector("#bf-rate");
    var space = form.querySelector("#bf-space");
    var duration = form.querySelector("#bf-duration");
    var total = form.querySelector("#bf-total");
    if (!rate || !space || !total) {
      return;
    }

    var option = space.options[space.selectedIndex];
    var pricePerHour = option ? parseInt(option.getAttribute("data-price"), 10) || 0 : 0;
    var deposit = option ? parseInt(option.getAttribute("data-deposit"), 10) || 0 : 0;

    var rent;
    var period;
    if (rate.value === "hour") {
      var hours = duration ? parseInt(duration.value, 10) || 1 : 1;
      rent = pricePerHour * hours;
      period = hours + " ч";
    } else {
      rent = PERIOD_PRICES[rate.value] || 0;
      period = PERIOD_LABELS[rate.value] || "";
    }

    var text = "<strong>" + formatMoney(rent) + " ₽</strong> за " + period;
    if (deposit > 0) {
      text += " + депозит " + formatMoney(deposit) + " ₽ (вернётся после встречи)";
    }
    total.innerHTML = text;
  }

  // Заменяет блок приложения новым HTML с сервера (без перезагрузки страницы).
  function replaceApp(html) {
    var doc = new DOMParser().parseFromString(html, "text/html");
    var next = doc.getElementById("booking-app");
    if (!next) {
      window.location.reload();
      return;
    }
    app.outerHTML = next.outerHTML;
    init();
    revealFlash();
  }

  // Сообщение о результате рендерится в начале блока, а после подмены
  // пользователь остаётся у таблицы ниже: без прокрутки кажется, что действие
  // не сработало (например, отказ удалить клиента, у которого есть брони).
  function revealFlash() {
    var flash = app.querySelector(".flash-err");
    if (!flash) {
      return;
    }

    var box = flash.getBoundingClientRect();
    if (box.top >= 0 && box.bottom <= window.innerHeight) {
      return;
    }

    var instant = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
    flash.scrollIntoView({ block: "nearest", behavior: instant ? "auto" : "smooth" });
  }

  // Загружает страницу по адресу и подменяет блок приложения.
  function fetchInto(url) {
    return fetch(url)
      .then(function (response) {
        return response.text();
      })
      .then(replaceApp);
  }

  // Переход по внутренней ссылке без перезагрузки (с прокруткой к якорю).
  function navigate(url, hash) {
    window.history.pushState(null, "", url);
    fetchInto(url).then(function () {
      if (hash) {
        var target = document.querySelector(hash);
        if (target) {
          target.scrollIntoView({ block: "start" });
        }
      }
    });
  }

  function bindLiveCalc(form) {
    if (!form) {
      return;
    }

    syncRateUI(form);

    form.addEventListener("input", function (event) {
      if (event.target.id === "bf-space" || event.target.id === "bf-rate") {
        syncRateUI(form);
      } else {
        limitDurationToClosing(form);
        updateTotal(form);
      }
    });
  }

  // Отправляет форму через fetch и подменяет блок приложения ответом.
  // Работает и для POST (бронь, клиент), и для GET (поиск).
  function submitAjax(form) {
    var method = (form.getAttribute("method") || "get").toLowerCase();
    var button = form.querySelector('button[type="submit"]');
    if (button) {
      button.disabled = true;
    }

    var request;
    if (method === "post") {
      request = fetch(form.action, { method: "POST", body: new FormData(form) });
    } else {
      var params = new URLSearchParams(new FormData(form)).toString();
      var url = params ? form.action + "?" + params : form.action;
      window.history.pushState(null, "", url);
      request = fetch(url);
    }

    request
      .then(function (response) {
        // После POST сервер делает редирект (PRG) — приводим адрес к итоговому.
        if (method === "post" && response.redirected) {
          window.history.replaceState(null, "", response.url);
        }
        return response.text();
      })
      .then(replaceApp)
      .catch(function () {
        if (button) {
          button.disabled = false;
        }
        form.submit();
      });
  }

  // Открывает и закрывает окно со схемой БД поверх страницы.
  function bindSchemaModal() {
    var modal = app.querySelector("[data-schema-modal]");
    var opener = app.querySelector("[data-schema-open]");
    if (!modal || !opener) {
      return;
    }

    opener.addEventListener("click", function () {
      modal.hidden = false;
      document.body.classList.add("schema-locked");
      var closeBtn = modal.querySelector("[data-schema-close]");
      if (closeBtn) {
        closeBtn.focus();
      }
    });

    modal.addEventListener("click", function (event) {
      var isBackdrop = event.target === modal;
      var isClose = event.target.closest && event.target.closest("[data-schema-close]");
      if (isBackdrop || isClose) {
        closeSchemaModal(modal, opener);
      }
    });
  }

  function closeSchemaModal(modal, opener) {
    modal.hidden = true;
    document.body.classList.remove("schema-locked");
    if (opener) {
      opener.focus();
    }
  }

  // Второй список поиска зависит от клиента: при смене клиента форма
  // отправляется сразу, и сервер отдаёт только его типы пространств.
  function bindSynchronizedSearch() {
    var client = app.querySelector("#bf-sync-client");
    if (client && client.form && client.form.hasAttribute("data-ajax")) {
      client.addEventListener("change", function () {
        submitAjax(client.form);
      });
    }
  }

  function init() {
    app = document.getElementById("booking-app");
    if (!app) {
      return;
    }

    bindLiveCalc(app.querySelector("#booking-form"));
    bindSchemaModal();
    bindSynchronizedSearch();

    // Делегирование: обработчик переживает подмену блока после fetch.
    app.addEventListener("submit", function (event) {
      var form = event.target;

      // Подтверждение для необратимых действий (удаление клиента).
      var confirmText = form.getAttribute && form.getAttribute("data-confirm");
      if (confirmText && !window.confirm(confirmText)) {
        event.preventDefault();
        return;
      }

      if (form.hasAttribute("data-cancel")) {
        event.preventDefault();
        fetch(form.action, { method: "POST" })
          .then(function (response) {
            return response.text();
          })
          .then(replaceApp)
          .catch(function () {
            form.submit();
          });
        return;
      }

      if (form.hasAttribute("data-ajax")) {
        event.preventDefault();
        submitAjax(form);
      }
    });

    // Внутренние ссылки (сброс поиска, редактирование, отмена) - без перезагрузки.
    app.addEventListener("click", function (event) {
      var link = event.target.closest ? event.target.closest("a[data-ajax]") : null;
      if (!link) {
        return;
      }
      event.preventDefault();
      navigate(link.getAttribute("href"), link.hash);
    });
  }

  window.addEventListener("popstate", function () {
    fetchInto(window.location.href);
  });

  // Esc закрывает схему БД (обработчик ставится один раз и переживает подмену блока).
  document.addEventListener("keydown", function (event) {
    if (event.key !== "Escape") {
      return;
    }
    var modal = document.querySelector("[data-schema-modal]:not([hidden])");
    if (modal) {
      closeSchemaModal(modal, document.querySelector("[data-schema-open]"));
    }
  });

  init();
})();
