(() => {
  const list = document.querySelector('#residents-list');
  const search = document.querySelector('#resident-search');
  const count = document.querySelector('#resident-result-count');
  const source = document.querySelector('#xml-source-code');
  const reset = document.querySelector('#resident-reset');
  const sortButtons = [...document.querySelectorAll('[data-sort]')];
  if (!list || !search || !count || !source) return;

  const fields = ['surname', 'name', 'patronymic', 'birthDate', 'birthPlace', 'profession', 'company', 'membership', 'keywords'];
  let residents = [];
  let sort = null;

  const value = (node, field) => node.querySelector(field)?.textContent.trim() || '';
  const fullName = (resident) => `${resident.surname} ${resident.name} ${resident.patronymic}`;
  const sortValue = (resident, field) => (field === 'name' ? fullName(resident) : resident[field]) || '';
  const render = () => {
    const terms = search.value.trim().toLocaleLowerCase('ru').split(/\s+/).filter(Boolean);
    const visible = residents.filter((resident) => {
      const values = fields.map((field) => resident[field].toLocaleLowerCase('ru'));
      return terms.every((term) => values.some((fieldValue) => fieldValue.includes(term)));
    });
    if (sort) {
      const collator = new Intl.Collator('ru', { sensitivity: 'base' });
      visible.sort((a, b) => collator.compare(sortValue(a, sort.field), sortValue(b, sort.field)) * sort.direction);
    }
    list.innerHTML = visible.length ? visible.map((resident) => `
      <tr><td><strong>${resident.surname} ${resident.name} ${resident.patronymic}</strong></td>
      <td>${resident.birthDate}<br><span class="muted">${resident.birthPlace}</span></td>
      <td>${resident.profession}</td><td>${resident.company}</td><td>${resident.membership}</td><td>${resident.keywords}</td></tr>`).join('') : '<tr><td colspan="6">По вашему запросу ничего не найдено.</td></tr>';
    count.textContent = `Найдено записей: ${visible.length} из ${residents.length}`;
    sortButtons.forEach((button) => {
      const active = sort?.field === button.dataset.sort;
      const header = button.closest('th');
      button.classList.toggle('is-active', active);
      button.querySelector('.sort-arrow').classList.toggle('is-ascending', active && sort.direction === 1);
      button.setAttribute('aria-label', `Сортировать по ${button.dataset.label || button.dataset.sort}${active ? (sort.direction === 1 ? ', по возрастанию' : ', по убыванию') : ''}`);
      header.setAttribute('aria-sort', active ? (sort.direction === 1 ? 'ascending' : 'descending') : 'none');
    });
  };

  fetch('/static/data/residents.xml')
    .then((response) => { if (!response.ok) throw new Error('XML недоступен'); return response.text(); })
    .then((text) => {
      source.textContent = text;
      const xml = new DOMParser().parseFromString(text, 'application/xml');
      if (xml.querySelector('parsererror')) throw new Error('XML содержит ошибку');
      residents = [...xml.querySelectorAll('resident')].map((node) => Object.fromEntries(fields.map((field) => [field, value(node, field)])));
      render();
    })
    .catch((error) => { count.textContent = error.message; list.innerHTML = '<tr><td colspan="6">Не удалось загрузить XML-файл.</td></tr>'; source.textContent = 'Ошибка загрузки XML-файла.'; });

  search.addEventListener('input', render);
  sortButtons.forEach((button) => button.addEventListener('click', () => {
    sort = sort?.field === button.dataset.sort
      ? { field: sort.field, direction: sort.direction * -1 }
      : { field: button.dataset.sort, direction: -1 };
    render();
  }));
  reset?.addEventListener('click', () => {
    search.value = '';
    sort = null;
    render();
    search.focus();
  });
})();
