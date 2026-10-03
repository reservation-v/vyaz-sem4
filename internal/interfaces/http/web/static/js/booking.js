// Страница «Бронирование»:
// - живой расчёт стоимости по пространству, тарифу и длительности;
// - периодные тарифы (день/неделя/месяц) только для деск-мест;
// - длительность почасовой брони ограничена закрытием офиса (22:00);
// - отправка формы и отмена брони через fetch — страница не перезагружается.
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
  }

  function bindForm(form) {
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

    form.addEventListener("submit", function (event) {
      event.preventDefault();
      var submit = form.querySelector('button[type="submit"]');
      submit.disabled = true;

      fetch(form.action, { method: "POST", body: new FormData(form) })
        .then(function (response) {
          return response.text();
        })
        .then(replaceApp)
        .catch(function () {
          submit.disabled = false;
          form.submit();
        });
    });
  }

  function init() {
    app = document.getElementById("booking-app");
    if (!app) {
      return;
    }

    bindForm(app.querySelector("#booking-form"));

    // Отмена брони: делегирование на весь блок, кнопки «Отменить» переживают перерисовку.
    app.addEventListener("submit", function (event) {
      var cancelForm = event.target.closest("form[data-cancel]");
      if (!cancelForm) {
        return;
      }
      event.preventDefault();

      fetch(cancelForm.action, { method: "POST" })
        .then(function (response) {
          return response.text();
        })
        .then(replaceApp)
        .catch(function () {
          cancelForm.submit();
        });
    });
  }

  init();
})();