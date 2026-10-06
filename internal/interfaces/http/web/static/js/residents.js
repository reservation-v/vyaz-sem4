(() => {
  const list = document.querySelector('#residents-list');
  const search = document.querySelector('#resident-search');
  const count = document.querySelector('#resident-result-count');
  const source = document.querySelector('#xml-source-code');
  if (!list || !search || !count || !source) return;

  const fields = ['surname', 'name', 'patronymic', 'birthDate', 'birthPlace', 'profession', 'company', 'membership', 'keywords'];
  let residents = [];

  const value = (node, field) => node.querySelector(field)?.textContent.trim() || '';
  const render = () => {
    const query = search.value.trim().toLocaleLowerCase('ru');
    const visible = residents.filter((resident) => fields.some((field) => resident[field].toLocaleLowerCase('ru').includes(query)));
    list.innerHTML = visible.length ? visible.map((resident) => `
      <tr><td><strong>${resident.surname} ${resident.name} ${resident.patronymic}</strong></td>
      <td>${resident.birthDate}<br><span class="muted">${resident.birthPlace}</span></td>
      <td>${resident.profession}</td><td>${resident.company}</td><td>${resident.membership}</td><td>${resident.keywords}</td></tr>`).join('') : '<tr><td colspan="6">По вашему запросу ничего не найдено.</td></tr>';
    count.textContent = `Найдено записей: ${visible.length} из ${residents.length}`;
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
})();
