// Страница «Новости»:
// - кнопка «Обновить» забирает общую ленту у нашего же сервера (/api/news);
// - кнопка «RSS нашего сайта» открывает предпросмотр собственной ленты поверх
//   страницы, откуда её можно скачать.
// RSS-ленты читает сервер, поэтому браузеру не мешает CORS.
(function () {
  "use strict";

  var app = document.getElementById("news-app");
  var list = document.getElementById("news-list");
  var status = document.getElementById("news-status");
  var refresh = document.getElementById("news-refresh");

  /* ---------- Меню страницы ---------- */

  var TAB_ORDER = ["feed", "add-news", "add-source"];

  function activateTab(name) {
    if (TAB_ORDER.indexOf(name) < 0) {
      name = "feed";
    }
    if (!app) {
      return;
    }

    app.setAttribute("data-tab", name);
    app.querySelectorAll("[data-panel]").forEach(function (panel) {
      panel.hidden = panel.getAttribute("data-panel") !== name;
    });
    app.querySelectorAll(".db-menu-btn[data-tab]").forEach(function (button) {
      var on = button.getAttribute("data-tab") === name;
      button.classList.toggle("is-active", on);
      button.setAttribute("aria-selected", on ? "true" : "false");
    });
  }

  if (app) {
    app.querySelectorAll(".db-menu-btn[data-tab]").forEach(function (button) {
      button.addEventListener("click", function () {
        activateTab(button.getAttribute("data-tab"));
      });
    });

    var initialTab = app.getAttribute("data-tab") || "feed";
    if (location.hash === "#feed" || location.hash === "#add-news" || location.hash === "#add-source") {
      initialTab = location.hash.slice(1);
    }
    activateTab(initialTab);
  }

  if (!list) {
    return;
  }

  var MONTHS = [
    "января", "февраля", "марта", "апреля", "мая", "июня",
    "июля", "августа", "сентября", "октября", "ноября", "декабря",
  ];

  function escapeHtml(value) {
    return String(value)
      .replace(/&/g, "&amp;")
      .replace(/</g, "&lt;")
      .replace(/>/g, "&gt;")
      .replace(/"/g, "&quot;");
  }

  function formatDate(value) {
    var date = new Date(value);
    if (isNaN(date.getTime())) {
      return "";
    }
    return date.getDate() + " " + MONTHS[date.getMonth()] + " " + date.getFullYear();
  }

  function render(items) {
    if (!items.length) {
      list.innerHTML = '<p class="table-note">Новостей пока нет. Добавьте свою новость или подключите RSS-ленту ниже.</p>';
      return;
    }

    list.innerHTML = items
      .map(function (item) {
        var title = item.title ? escapeHtml(item.title) : "Без заголовка";
        var heading = item.link
          ? '<a href="' + escapeHtml(item.link) + '" target="_blank" rel="noopener">' + title + "</a>"
          : title;

        var date = formatDate(item.published);
        var meta =
          '<span class="news-source news-source-' + (item.own ? "own" : "external") + '">' +
          escapeHtml(item.source) + "</span>" +
          (date ? '<span class="news-date">' + date + "</span>" : "");

        var summary = item.summary ? "<p>" + escapeHtml(item.summary) + "</p>" : "";
        var actions = item.own
          ? '<div class="news-item-actions"><form class="inline-form" method="post" action="/news/' +
            item.id +
            '/delete"><button class="btn-mini" type="submit">Удалить</button></form></div>'
          : "";

        return (
          '<article class="news-item" id="news-' + item.id + '">' +
          '<div class="news-meta">' + meta + "</div>" +
          "<h2>" + heading + "</h2>" +
          summary +
          actions +
          "</article>"
        );
      })
      .join("");
  }

  function load() {
    if (refresh) {
      refresh.disabled = true;
    }
    if (status) {
      status.textContent = "Обновляем ленты…";
    }

    fetch("/api/news", { headers: { Accept: "application/json" } })
      .then(function (response) {
        if (!response.ok) {
          throw new Error("статус " + response.status);
        }
        return response.json();
      })
      .then(function (payload) {
        var items = (payload && payload.items) || [];
        var sources = (payload && payload.sources) || [];
        render(items);
        if (status) {
          status.textContent = "Показано " + items.length + " новостей. Источников: " + sources.length + ".";
        }
      })
      .catch(function () {
        if (status) {
          status.textContent = "Не удалось обновить новости. Попробуйте позже.";
        }
      })
      .then(function () {
        if (refresh) {
          refresh.disabled = false;
        }
      });
  }

  if (refresh) {
    refresh.addEventListener("click", load);
  }

  /* ---------- Предпросмотр и скачивание своей RSS-ленты ---------- */

  var rssModal = document.getElementById("rss-modal");
  var rssOpen = document.getElementById("rss-open");
  var rssClose = document.getElementById("rss-close");
  var rssContent = document.getElementById("rss-content");
  var rssDownload = document.getElementById("rss-download");

  if (!rssModal || !rssOpen || !rssClose || !rssContent) {
    return;
  }

  var downloadURL = null;

  function openRssModal() {
    rssModal.hidden = false;
    document.body.classList.add("schema-locked");
    rssClose.focus();
    loadRss();
  }

  function closeRssModal() {
    rssModal.hidden = true;
    document.body.classList.remove("schema-locked");
    rssOpen.focus();
  }

  function loadRss() {
    rssContent.textContent = "Загружаем RSS-ленту…";

    fetch("/news/rss.xml", {
      headers: { Accept: "application/rss+xml, application/xml;q=0.9, text/xml;q=0.9" },
    })
      .then(function (response) {
        if (!response.ok) {
          throw new Error("статус " + response.status);
        }
        return response.text();
      })
      .then(function (text) {
        rssContent.textContent = text;

        // Blob гарантирует скачивание файла с нужным именем, а не открытие XML
        // в новой вкладке. Если Blob недоступен, остаётся обычная ссылка.
        if (rssDownload && window.Blob && window.URL) {
          if (downloadURL) {
            window.URL.revokeObjectURL(downloadURL);
          }
          downloadURL = window.URL.createObjectURL(
            new Blob([text], { type: "application/rss+xml;charset=utf-8" })
          );
          rssDownload.href = downloadURL;
        }
      })
      .catch(function () {
        rssContent.textContent = "Не удалось загрузить RSS-ленту. Попробуйте позже.";
      });
  }

  rssOpen.addEventListener("click", openRssModal);
  rssClose.addEventListener("click", closeRssModal);

  // Клик по затемнённому фону закрывает окно, клик внутри диалога - нет.
  rssModal.addEventListener("click", function (event) {
    if (event.target === rssModal) {
      closeRssModal();
    }
  });

  document.addEventListener("keydown", function (event) {
    if (event.key === "Escape" && !rssModal.hidden) {
      closeRssModal();
    }
  });
})();
