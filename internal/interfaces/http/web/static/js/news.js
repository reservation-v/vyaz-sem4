// Страница «Новости»:
// - переключение двух блоков кнопками: внешние новости и собственные новости БД;
// - внешние новости страница забирает у своего же сервера (/api/news/external).
//   RSS-ленты чужого приложения читает сервер: браузеру мешает CORS, а серверу - нет.
(function () {
  "use strict";

  var app = document.getElementById("news-app");
  if (!app) {
    return;
  }

  /* ---------- Переключение вкладок ---------- */

  var TAB_ORDER = ["external", "own"];

  function activateTab(name) {
    if (TAB_ORDER.indexOf(name) < 0) {
      name = "external";
    }

    app.querySelectorAll("[data-panel]").forEach(function (panel) {
      panel.hidden = panel.getAttribute("data-panel") !== name;
    });

    app.querySelectorAll("[data-tab]").forEach(function (button) {
      var on = button.getAttribute("data-tab") === name;
      button.classList.toggle("is-active", on);
      button.setAttribute("aria-selected", on ? "true" : "false");
    });
  }

  app.querySelectorAll("[data-tab]").forEach(function (button) {
    button.addEventListener("click", function () {
      activateTab(button.getAttribute("data-tab"));
    });
  });

  /* ---------- Внешние новости (JSON с нашего сервера) ---------- */

  var ENDPOINT = "/api/news/external";

  var list = document.getElementById("external-news");
  var status = document.getElementById("external-status");
  var refresh = document.getElementById("external-refresh");
  if (!list || !status) {
    return;
  }

  function escapeHtml(value) {
    return String(value)
      .replace(/&/g, "&amp;")
      .replace(/</g, "&lt;")
      .replace(/>/g, "&gt;")
      .replace(/"/g, "&quot;");
  }

  function formatDate(value) {
    var parsed = Date.parse(value);
    if (isNaN(parsed)) {
      return "";
    }
    return new Date(parsed).toLocaleDateString("ru", {
      day: "numeric",
      month: "long",
      year: "numeric",
    });
  }

  function metaLine(item) {
    var parts = [formatDate(item.published), item.source].filter(Boolean);
    return escapeHtml(parts.join(" · "));
  }

  function render(items) {
    if (!items.length) {
      status.textContent = "Внешние новости сейчас недоступны. Загляните позже или посмотрите «Наши новости».";
      list.innerHTML = "";
      return;
    }

    status.textContent = "Показано " + items.length + " свежих новостей из внешних источников.";
    list.innerHTML = items
      .map(function (item) {
        var title = item.title ? escapeHtml(item.title) : "Без заголовка";
        var heading = item.link
          ? '<a href="' + escapeHtml(item.link) + '" target="_blank" rel="noopener">' + title + "</a>"
          : title;
        var body = item.summary ? "<p>" + escapeHtml(item.summary) + "</p>" : "";
        var meta = metaLine(item);
        var metaBlock = meta ? '<p class="news-date">' + meta + "</p>" : "";

        return '<article class="news-item">' + metaBlock + "<h2>" + heading + "</h2>" + body + "</article>";
      })
      .join("");
  }

  function loadExternalNews() {
    status.textContent = "Загружаем новости из внешних источников…";
    list.innerHTML = "";
    if (refresh) {
      refresh.disabled = true;
    }

    fetch(ENDPOINT, { headers: { Accept: "application/json" } })
      .then(function (response) {
        if (!response.ok) {
          throw new Error("статус " + response.status);
        }
        return response.json();
      })
      .then(function (items) {
        render(items || []);
      })
      .catch(function () {
        render([]);
      })
      .then(function () {
        if (refresh) {
          refresh.disabled = false;
        }
      });
  }

  if (refresh) {
    refresh.addEventListener("click", loadExternalNews);
  }

  loadExternalNews();
})();
