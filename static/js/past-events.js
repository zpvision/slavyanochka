(() => {
  const grid=document.querySelector('.past .event-grid');
  const previous=document.querySelector('.past-prev');
  const next=document.querySelector('.past-next');
  if(!grid||!previous||!next)return;
  const locales={ru:'ru-RU',uk:'uk-UA',cs:'cs-CZ',en:'en-GB',pl:'pl-PL',hr:'hr-HR'};
  let events=[];

  function locale(){return locales[document.querySelector('#language')?.value]||'ru-RU'}
  function parseDate(value){const [year,month,day]=value.split('-').map(Number);return new Date(year,month-1,day)}
  function updateControls(){
    const scrollable=grid.scrollWidth>grid.clientWidth+2;
    previous.hidden=!scrollable;next.hidden=!scrollable;
    previous.disabled=grid.scrollLeft<=2;
    next.disabled=grid.scrollLeft+grid.clientWidth>=grid.scrollWidth-2;
  }
  function render(){
    grid.replaceChildren();
    if(!events.length){const empty=document.createElement('p');empty.className='past-empty';empty.textContent='Прошедших мероприятий пока нет.';grid.append(empty);updateControls();return;}
    const formatter=new Intl.DateTimeFormat(locale(),{month:'long',year:'numeric'});
    events.forEach(item=>{
      const article=document.createElement('article');
      const image=document.createElement('img');image.src=item.image;image.alt=item.title;image.loading='lazy';image.decoding='async';
      const title=document.createElement('h3');title.textContent=item.title;
      const date=document.createElement('b');const value=formatter.format(parseDate(item.date));date.textContent=value.charAt(0).toUpperCase()+value.slice(1);
      const description=document.createElement('p');description.textContent=item.description;
      article.append(image,title,date,description);grid.append(article);
    });
    grid.scrollLeft=0;requestAnimationFrame(updateControls);
  }
  async function load(){
    try{const response=await fetch('/api/past-events',{cache:'no-store'});if(!response.ok)throw new Error();const updated=await response.json();if(JSON.stringify(updated)!==JSON.stringify(events)){events=updated;render();}}
    catch{grid.replaceChildren();const error=document.createElement('p');error.className='past-error';error.textContent='Не удалось загрузить мероприятия.';grid.append(error);updateControls();}
  }
  function scroll(direction){const card=grid.querySelector('article');const amount=card?card.getBoundingClientRect().width+20:grid.clientWidth;grid.scrollBy({left:direction*amount,behavior:'smooth'});}
  previous.addEventListener('click',()=>scroll(-1));next.addEventListener('click',()=>scroll(1));grid.addEventListener('scroll',updateControls,{passive:true});
  new ResizeObserver(updateControls).observe(grid);
  document.querySelector('#language')?.addEventListener('change',()=>queueMicrotask(render));
  load();setInterval(()=>{if(!document.hidden)load()},10000);
})();
