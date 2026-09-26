(() => {
  const slogan = document.querySelector('.gzhel-hero .slogan');
  const language = document.querySelector('#language');
  if (!slogan || !language) return;

  const phrases = {
    ru: [
      '«Сохраняем славянские традиции\nчерез музыку».',
      '«Объединяем сердца\nголосами предков».',
      '«Дарим народной песне\nновую жизнь».'
    ],
    uk: [
      '«Зберігаємо слов’янські традиції\nчерез музику».',
      '«Об’єднуємо серця\nголосами предків».',
      '«Даруємо народній пісні\nнове життя».'
    ],
    cs: [
      '„Uchováváme slovanské tradice\nprostřednictvím hudby.“',
      '„Spojujeme srdce\nhlasy našich předků.“',
      '„Dáváme lidové písni\nnový život.“'
    ],
    en: [
      '“Preserving Slavic traditions\nthrough music.”',
      '“Uniting hearts\nwith the voices of our ancestors.”',
      '“Giving folk songs\na new life.”'
    ],
    pl: [
      '„Chronimy słowiańskie tradycje\npoprzez muzykę.”',
      '„Łączymy serca\ngłosami przodków.”',
      '„Dajemy pieśni ludowej\nnowe życie.”'
    ],
    hr: [
      '„Čuvamo slavensku tradiciju\nkroz glazbu.”',
      '„Povezujemo srca\nglasovima predaka.”',
      '„Dajemo narodnoj pjesmi\nnovi život.”'
    ]
  };

  const reducedMotion = matchMedia('(prefers-reduced-motion: reduce)').matches;
  let phraseIndex = 0;
  let characterIndex = 0;
  let deleting = false;
  let timeout;
  let runID = 0;

  function build() {
    slogan.replaceChildren();
    const visible = document.createElement('span');
    visible.className = 'slogan-typing';
    visible.setAttribute('aria-hidden', 'true');
    const accessible = document.createElement('span');
    accessible.className = 'sr-only slogan-accessible';
    const cursor = document.createElement('span');
    cursor.className = 'slogan-cursor';
    cursor.setAttribute('aria-hidden', 'true');
    slogan.append(visible, accessible, cursor);
    return {visible, accessible};
  }

  function start() {
    clearTimeout(timeout);
    runID++;
    phraseIndex = 0;
    characterIndex = 0;
    deleting = false;
    const nodes = build();
    const localized = phrases[language.value] || phrases.ru;
    nodes.accessible.textContent = localized[0].replace('\n', ' ');
    if (reducedMotion) {
      nodes.visible.textContent = localized[0];
      return;
    }
    type(nodes, localized, runID);
  }

  function type(nodes, localized, currentRun) {
    if (currentRun !== runID) return;
    const characters = Array.from(localized[phraseIndex]);
    characterIndex += deleting ? -1 : 1;
    nodes.visible.textContent = characters.slice(0, Math.max(0, characterIndex)).join('');

    let delay = deleting ? 28 : 53;
    if (!deleting && characterIndex >= characters.length) {
      nodes.accessible.textContent = localized[phraseIndex].replace('\n', ' ');
      deleting = true;
      delay = 3100;
    } else if (deleting && characterIndex <= 0) {
      deleting = false;
      phraseIndex = (phraseIndex + 1) % localized.length;
      nodes.accessible.textContent = localized[phraseIndex].replace('\n', ' ');
      delay = 520;
    }
    timeout = setTimeout(() => type(nodes, localized, currentRun), delay);
  }

  language.addEventListener('change', () => queueMicrotask(start));
  document.addEventListener('visibilitychange', () => {
    if (document.hidden) clearTimeout(timeout);
    else start();
  });
  start();
})();
