(() => {
  const section = document.querySelector('.gallery-showcase');
  if (!section) return;
  const slideshow = section.querySelector('.slideshow');
  const stage = section.querySelector('.slideshow-stage');
  const dots = section.querySelector('.slide-dots');
  const counter = section.querySelector('.slide-counter');
  const reduceMotion = matchMedia('(prefers-reduced-motion: reduce)').matches;
  let images = [];
  let current = 0;
  let timer;

  function show(index, userInitiated = false) {
    if (!images.length) return;
    current = (index + images.length) % images.length;
    stage.querySelectorAll('.gallery-slide').forEach((slide, position) => {
      const active = position === current;
      slide.classList.toggle('is-active', active);
      slide.setAttribute('aria-hidden', String(!active));
    });
    dots.querySelectorAll('.slide-dot').forEach((dot, position) => {
      const active = position === current;
      dot.classList.toggle('is-active', active);
      dot.setAttribute('aria-current', active ? 'true' : 'false');
    });
    counter.textContent = `${current + 1} / ${images.length}`;
    if (userInitiated) restart();
  }

  function restart() {
    clearInterval(timer);
    if (!reduceMotion && images.length > 1) timer = setInterval(() => show(current + 1), 6500);
  }

  function render() {
    clearInterval(timer);
    stage.replaceChildren();
    dots.replaceChildren();
    section.hidden = !images.length;
    if (!images.length) return;
    current = Math.min(current, images.length - 1);
    images.forEach((item, index) => {
      const figure = document.createElement('figure');
      figure.className = 'gallery-slide';
      figure.setAttribute('aria-roledescription', 'слайд');
      figure.setAttribute('aria-label', `${index + 1} из ${images.length}`);
      const image = document.createElement('img');
      image.src = `/uploads/${encodeURIComponent(item.file)}`;
      image.alt = item.caption || `Фотография ансамбля ${index + 1}`;
      image.loading = index === 0 ? 'eager' : 'lazy';
      image.decoding = 'async';
      figure.append(image);
      if (item.caption) {
        const caption = document.createElement('figcaption');
        caption.textContent = item.caption;
        figure.append(caption);
      }
      const dot = document.createElement('button');
      dot.type = 'button';
      dot.className = 'slide-dot';
      dot.setAttribute('aria-label', `Показать фотографию ${index + 1}`);
      dot.addEventListener('click', () => show(index, true));
      stage.append(figure); dots.append(dot);
    });
    slideshow.classList.toggle('is-single', images.length === 1);
    show(current);
    restart();
  }

  async function load() {
    try {
      const response = await fetch('/api/gallery', {cache:'no-store'});
      if (!response.ok) throw new Error();
      const updated = await response.json();
      if (JSON.stringify(updated) !== JSON.stringify(images)) {
        images = updated;
        render();
      }
    } catch { section.hidden = true; }
  }

  section.querySelector('.slide-prev').addEventListener('click', () => show(current - 1, true));
  section.querySelector('.slide-next').addEventListener('click', () => show(current + 1, true));
  slideshow.addEventListener('mouseenter', () => clearInterval(timer));
  slideshow.addEventListener('mouseleave', restart);
  slideshow.addEventListener('focusin', () => clearInterval(timer));
  slideshow.addEventListener('focusout', restart);
  document.addEventListener('visibilitychange', () => document.hidden ? clearInterval(timer) : restart());
  load();
  setInterval(() => { if (!document.hidden) load(); }, 10000);
})();
