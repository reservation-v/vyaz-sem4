// Поиск по сайту: фильтр страниц и сервисов, переход к найденному блоку.
// Бургер-меню «Остальное тут»: раскрытие списка всех страниц.
// Анимация появления секций при скролле.
// Всё уважает prefers-reduced-motion: анимация полностью отключается.
(function () {
  "use strict";

  var prefersReducedMotion = window.matchMedia(
    "(prefers-reduced-motion: reduce)",
  ).matches;

  /* ---------- Бургер-меню ---------- */

  var burger = document.getElementById("burger-btn");
  var burgerMenu = document.getElementById("burger-menu");

  if (burger && burgerMenu) {
    function closeBurger() {
      burgerMenu.hidden = true;
      burger.setAttribute("aria-expanded", "false");
      burger.classList.remove("is-open");
    }

    burger.addEventListener("click", function () {
      if (burgerMenu.hidden) {
        burgerMenu.hidden = false;
        burger.setAttribute("aria-expanded", "true");
        burger.classList.add("is-open");
      } else {
        closeBurger();
      }
    });

    document.addEventListener("click", function (event) {
      if (
        !burger.contains(event.target) &&
        !burgerMenu.contains(event.target)
      ) {
        closeBurger();
      }
    });

    document.addEventListener("keydown", function (event) {
      if (event.key === "Escape") {
        closeBurger();
      }
    });

    burgerMenu.addEventListener("click", closeBurger);
  }

  /* ---------- Поиск по сайту ---------- */

  var pages = [
    {
      title: "Главная",
      url: "/",
      keywords: "коворкинг сервисы столы переговорки кабинеты",
    },
    {
      title: "О коворкинге",
      url: "/about",
      keywords: "идея для кого фрилансеры стартапы команды мероприятия",
    },
    {
      title: "Офис",
      url: "/office",
      keywords:
        "план этаж зоны деск место кабинет переговорка лаунж москва сити",
    },
    {
      title: "Тарифы",
      url: "/tariffs",
      keywords: "цены час день неделя месяц абонемент депозит опции проектор",
    },
    {
      title: "Новости",
      url: "/news",
      keywords: "анонсы события митапы скидка открытие",
    },
    {
      title: "Достижения",
      url: "/achievements",
      keywords: "резиденты бронирования мероприятия рейтинг",
    },
    {
      title: "Контакты",
      url: "/contacts",
      keywords: "адрес телефон почта метро парковка часы работы",
    },
    {
      title: "Правила",
      url: "/rules",
      keywords: "правила посещения тишина оплата безопасность гости",
    },
    {
      title: "Вопросы&Ответы",
      url: "/faq",
      keywords: "вопросы ответы faq бронь отмена депозит абонемент",
    },
    {
      title: "Бронирование",
      url: "/booking",
      keywords: "приложение база данных бронь оплата",
    },
    {
      title: "Онлайн-бронирование",
      url: "/#svc-booking",
      keywords: "сервис приложение база данных расчёт стоимость оплата",
    },
    {
      title: "Погода у офиса",
      url: "/#svc-weather",
      keywords: "сервис погода прогноз wttr температура ветер осадки",
    },
    {
      title: "Курс валют",
      url: "/#svc-currency",
      keywords: "сервис курс валют доллар евро юань цб банк россии",
    },
    {
      title: "Переводчик RU↔EN",
      url: "/#svc-translate",
      keywords: "сервис переводчик перевод английский mymemory",
    },
    {
      title: "QR-код",
      url: "/#svc-qr-code",
      keywords: "сервис qr код ссылка генератор goqr",
    },
    {
      title: "Рабочий день или выходной",
      url: "/#svc-workday",
      keywords: "сервис рабочий день выходной график isdayoff",
    },
    {
      title: "Время в филиалах",
      url: "/#svc-world-time",
      keywords: "сервис время часовой пояс москва пекин лондон timeapi",
    },
    {
      title: "Форма обратной связи",
      url: "/#svc-feedback",
      keywords: "сервис форма обратная связь заявка вопрос formsubmit",
    },
  ];

  var input = document.getElementById("site-search");
  var list = document.getElementById("search-results");

  if (input && list) {
    function normalize(value) {
      return value.toLowerCase().replace(/ё/g, "е");
    }

    function renderSearch(query) {
      var words = normalize(query.trim()).split(/\s+/).filter(Boolean);
      list.innerHTML = "";

      if (!words.length) {
        list.hidden = true;
        return;
      }

      var hits = pages.filter(function (page) {
        var haystack = normalize(page.title + " " + page.keywords);
        return words.every(function (word) {
          return haystack.indexOf(word) !== -1;
        });
      });

      if (!hits.length) {
        var empty = document.createElement("li");
        empty.className = "sr-muted";
        empty.textContent = "Ничего не нашлось";
        list.appendChild(empty);
      } else {
        hits.forEach(function (page) {
          var item = document.createElement("li");
          var link = document.createElement("a");
          link.href = page.url;
          link.textContent = page.title;
          item.appendChild(link);
          list.appendChild(item);
        });
      }

      list.hidden = false;
    }

    input.addEventListener("input", function () {
      renderSearch(input.value);
    });

    input.addEventListener("keydown", function (event) {
      if (event.key === "Enter") {
        var first = list.querySelector("a");
        if (first) {
          window.location.href = first.getAttribute("href");
        }
      }
      if (event.key === "Escape") {
        list.hidden = true;
        input.blur();
      }
    });

    input.addEventListener("focus", function () {
      if (input.value) {
        renderSearch(input.value);
      }
    });

    input.addEventListener("blur", function () {
      setTimeout(function () {
        list.hidden = true;
      }, 150);
    });
  }

  /* ---------- Появление секций при скролле ---------- */

  if (prefersReducedMotion || !("IntersectionObserver" in window)) {
    document.querySelectorAll(".reveal").forEach(function (el) {
      el.classList.add("is-visible");
    });
    return;
  }

  var observer = new IntersectionObserver(
    function (entries) {
      entries.forEach(function (entry) {
        if (!entry.isIntersecting) {
          return;
        }
        entry.target.classList.add("is-visible");
        observer.unobserve(entry.target);
      });
    },
    { threshold: 0.12, rootMargin: "0px 0px -40px 0px" },
  );

  document.querySelectorAll(".reveal").forEach(function (el) {
    observer.observe(el);
  });
})();
