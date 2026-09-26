(() => {
  const datesContainer = document.querySelector('.upcoming .dates');
  const calendarElement = document.querySelector('.upcoming .calendar');
  if (!datesContainer || !calendarElement) return;

  const locales = {ru:'ru-RU',uk:'uk-UA',cs:'cs-CZ',en:'en-GB',pl:'pl-PL',hr:'hr-HR'};
  let events = [];
  const now = new Date();
  let calendarYear = now.getFullYear();
  let calendarMonth = now.getMonth();

  function currentLocale() {
    return locales[document.querySelector('#language')?.value] || 'ru-RU';
  }

  function parseDate(value) {
    const [year, month, day] = value.split('-').map(Number);
    return new Date(year, month - 1, day);
  }

  function element(tag, className, text) {
    const node = document.createElement(tag);
    if (className) node.className = className;
    if (text !== undefined) node.textContent = text;
    return node;
  }

  function renderCards() {
    datesContainer.replaceChildren();
    if (!events.length) {
      datesContainer.append(element('p', 'events-empty', 'Предстоящих мероприятий пока нет.'));
      return;
    }
    const locale = currentLocale();
    events.forEach(item => {
      const date = parseDate(item.date);
      const article = element('article');
      const time = element('time');
      time.dateTime = item.date;
      time.append(element('small', '', new Intl.DateTimeFormat(locale, {month:'short'}).format(date).replace('.', '').toUpperCase()));
      time.append(document.createTextNode(String(date.getDate())));
      article.append(time, element('h3', '', item.title));
      if (item.location) article.append(element('b', '', `⌖ ${item.location}`));
      article.append(element('p', '', item.description));
      datesContainer.append(article);
    });
  }

  function renderCalendar() {
    const locale = currentLocale();
    const focus = new Date(calendarYear, calendarMonth, 1);
    const eventsByDay = new Map();
    events.filter(item => {
      const date = parseDate(item.date);
      return date.getFullYear() === calendarYear && date.getMonth() === calendarMonth;
    }).forEach(item => {
      const day = parseDate(item.date).getDate();
      const items = eventsByDay.get(day) || [];
      items.push(item);
      eventsByDay.set(day, items);
    });
    const heading = calendarElement.querySelector('h3');
    const monthName = new Intl.DateTimeFormat(locale, {month:'long', year:'numeric'}).format(focus);
    heading.textContent = `${monthName.charAt(0).toUpperCase()}${monthName.slice(1)}`;
    const labels = calendarElement.querySelector('.labels');
    labels.replaceChildren();
    const monday = new Date(2024, 0, 1);
    for (let index = 0; index < 7; index++) {
      const day = new Date(monday);
      day.setDate(monday.getDate() + index);
      labels.append(element('b', '', new Intl.DateTimeFormat(locale, {weekday:'short'}).format(day).replace('.', '')));
    }
    const days = calendarElement.querySelector('.days');
    days.replaceChildren();
    const firstOffset = (new Date(calendarYear, calendarMonth, 1).getDay() + 6) % 7;
    for (let index = 0; index < firstOffset; index++) days.append(element('i'));
    const count = new Date(calendarYear, calendarMonth + 1, 0).getDate();
    for (let day = 1; day <= count; day++) {
      const dayEvents = eventsByDay.get(day) || [];
      const isToday = day === now.getDate() && calendarMonth === now.getMonth() && calendarYear === now.getFullYear();
      const className = [dayEvents.length ? 'selected' : '', isToday ? 'today' : ''].filter(Boolean).join(' ');
      const dayElement = element('i', className, String(day));
      if (dayEvents.length) {
        const eventNames = dayEvents.map(item => item.title).join('\n');
        dayElement.title = eventNames;
        dayElement.setAttribute('aria-label', `${day} — ${eventNames}`);
      } else if (isToday) {
        dayElement.setAttribute('aria-label', `${day}, сегодня`);
      }
      days.append(dayElement);
    }
  }

  function render() {
    renderCards();
    renderCalendar();
  }

  async function loadEvents() {
    try {
      const response = await fetch('/api/events', {cache:'no-store'});
      if (!response.ok) throw new Error();
      events = await response.json();
      render();
    } catch {
      datesContainer.replaceChildren(element('p', 'events-error', 'Не удалось загрузить мероприятия. Обновите страницу позже.'));
    }
  }

  function moveCalendar(monthOffset) {
    const target = new Date(calendarYear, calendarMonth + monthOffset, 1);
    calendarYear = target.getFullYear();
    calendarMonth = target.getMonth();
    renderCalendar();
  }

  calendarElement.querySelector('.calendar-prev').addEventListener('click', () => moveCalendar(-1));
  calendarElement.querySelector('.calendar-next').addEventListener('click', () => moveCalendar(1));
  document.querySelector('#language')?.addEventListener('change', () => queueMicrotask(render));
  loadEvents();
  setInterval(() => { if (!document.hidden) loadEvents(); }, 10000);
  window.addEventListener('focus', loadEvents);
})();
