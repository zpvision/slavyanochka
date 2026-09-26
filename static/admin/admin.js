(() => {
  const loginPanel = document.querySelector('#login-panel');
  const loginForm = document.querySelector('#login-form');
  const dashboard = document.querySelector('#dashboard');
  const headerActions = document.querySelector('.header-actions');
  const eventForm = document.querySelector('#event-form');
  const eventList = document.querySelector('#event-list');
  const cancelEdit = document.querySelector('#cancel-edit');
  const deleteDialog = document.querySelector('#delete-dialog');
  const galleryForm = document.querySelector('#gallery-form');
  const galleryList = document.querySelector('#gallery-list');
  const pastEventForm = document.querySelector('#past-event-form');
  const pastEventList = document.querySelector('#past-event-list');
  const cancelPastEdit = document.querySelector('#cancel-past-edit');
  let events = [];
  let gallery = [];
  let pastEvents = [];
  let pendingDelete = null;

  async function request(url, options = {}) {
    const headers = options.body instanceof FormData ? {...(options.headers || {})} : {'Content-Type':'application/json', ...(options.headers || {})};
    const response = await fetch(url, {cache:'no-store', ...options, headers});
    if (response.status === 204) return null;
    const body = await response.json().catch(() => ({}));
    if (!response.ok) throw new Error(body.error || 'Не удалось выполнить запрос');
    return body;
  }

  function showDashboard(authenticated) {
    loginPanel.hidden = authenticated;
    dashboard.hidden = !authenticated;
    headerActions.hidden = !authenticated;
  }

  function message(form, text, success = false) {
    const node = form.querySelector('.form-message');
    node.textContent = text;
    node.classList.toggle('success', success);
  }

  function parseDate(value) {
    const [year, month, day] = value.split('-').map(Number);
    return new Date(year, month - 1, day);
  }

  function render() {
    eventList.replaceChildren();
    document.querySelector('#event-count').textContent = `${events.length} ${events.length === 1 ? 'мероприятие' : 'мероприятия'}`;
    if (!events.length) {
      const empty = document.createElement('p');
      empty.className = 'empty';
      empty.textContent = 'Добавьте первое мероприятие — оно сразу появится на сайте.';
      eventList.append(empty);
      return;
    }
    events.forEach(item => {
      const card = document.createElement('article');
      card.className = 'event-card';
      const date = parseDate(item.date);
      const dateBox = document.createElement('time');
      dateBox.className = 'event-date';
      dateBox.dateTime = item.date;
      const month = new Intl.DateTimeFormat('ru-RU',{month:'short'}).format(date).replace('.','').toUpperCase();
      dateBox.innerHTML = `<strong>${date.getDate()}</strong><span>${month} ${date.getFullYear()}</span>`;
      const info = document.createElement('div');
      info.className = 'event-info';
      const title = document.createElement('h3'); title.textContent = item.title; info.append(title);
      if (item.location) { const location = document.createElement('p'); location.className='location'; location.textContent=`⌖ ${item.location}`; info.append(location); }
      const description = document.createElement('p'); description.textContent=item.description; info.append(description);
      const actions = document.createElement('div'); actions.className='event-actions';
      const edit = document.createElement('button'); edit.type='button'; edit.className='button button-light'; edit.textContent='Изменить'; edit.addEventListener('click',()=>startEdit(item));
      const remove = document.createElement('button'); remove.type='button'; remove.className='button button-light'; remove.textContent='Удалить'; remove.addEventListener('click',()=>askDelete(item));
      actions.append(edit,remove); card.append(dateBox,info,actions); eventList.append(card);
    });
  }

  async function loadEvents() {
    events = await request('/api/events');
    render();
  }

  function renderGallery() {
    galleryList.replaceChildren();
    document.querySelector('#gallery-count').textContent = `${gallery.length} фото`;
    if (!gallery.length) {
      const empty=document.createElement('p'); empty.className='empty'; empty.textContent='Загрузите первую фотографию — после этого слайд-шоу появится на сайте.'; galleryList.append(empty); return;
    }
    gallery.forEach((item,index)=>{
      const card=document.createElement('article'); card.className='gallery-admin-card';
      const image=document.createElement('img'); image.src=`/uploads/${encodeURIComponent(item.file)}`; image.alt=item.caption || `Фотография ${index+1}`; image.loading='lazy';
      const info=document.createElement('div'); info.className='gallery-admin-card-info';
      const caption=document.createElement('p'); caption.textContent=item.caption || 'Без подписи';
      const actions=document.createElement('div'); actions.className='gallery-order-actions';
      const up=document.createElement('button'); up.type='button'; up.className='button button-light'; up.textContent='←'; up.title='Переместить раньше'; up.setAttribute('aria-label','Переместить фотографию раньше'); up.disabled=index===0; up.addEventListener('click',()=>movePhoto(index,-1));
      const down=document.createElement('button'); down.type='button'; down.className='button button-light'; down.textContent='→'; down.title='Переместить позже'; down.setAttribute('aria-label','Переместить фотографию позже'); down.disabled=index===gallery.length-1; down.addEventListener('click',()=>movePhoto(index,1));
      const remove=document.createElement('button'); remove.type='button'; remove.className='button button-light remove-photo'; remove.textContent='Удалить'; remove.addEventListener('click',()=>removePhoto(item));
      actions.append(up,down,remove); info.append(caption,actions); card.append(image,info); galleryList.append(card);
    });
  }

  async function loadGallery() {
    gallery=await request('/api/gallery');
    renderGallery();
  }

  function renderPastEvents() {
    pastEventList.replaceChildren();
    document.querySelector('#past-event-count').textContent=`${pastEvents.length} ${pastEvents.length===1?'мероприятие':'мероприятия'}`;
    if(!pastEvents.length){const empty=document.createElement('p');empty.className='empty';empty.textContent='Архив прошедших мероприятий пока пуст.';pastEventList.append(empty);return;}
    pastEvents.forEach(item=>{
      const card=document.createElement('article');card.className='event-card';
      const image=document.createElement('img');image.className='past-event-thumb';image.src=item.image;image.alt='';image.loading='lazy';
      const info=document.createElement('div');info.className='event-info';
      const title=document.createElement('h3');title.textContent=item.title;
      const date=document.createElement('p');date.className='location';const formatted=new Intl.DateTimeFormat('ru-RU',{day:'numeric',month:'long',year:'numeric'}).format(parseDate(item.date));date.textContent=formatted;
      const description=document.createElement('p');description.textContent=item.description;info.append(title,date,description);
      const actions=document.createElement('div');actions.className='event-actions';
      const edit=document.createElement('button');edit.type='button';edit.className='button button-light';edit.textContent='Изменить';edit.addEventListener('click',()=>startPastEdit(item));
      const remove=document.createElement('button');remove.type='button';remove.className='button button-light';remove.textContent='Удалить';remove.addEventListener('click',()=>removePastEvent(item));
      actions.append(edit,remove);card.append(image,info,actions);pastEventList.append(card);
    });
  }

  async function loadPastEvents(){pastEvents=await request('/api/past-events');renderPastEvents()}

  function resetPastForm(){
    pastEventForm.reset();pastEventForm.elements.id.value='';pastEventForm.elements.image.required=true;
    document.querySelector('#past-image-hint').textContent='(обязательно)';document.querySelector('#past-form-title').textContent='Новое прошедшее мероприятие';
    pastEventForm.querySelector('[type=submit]').textContent='Добавить в архив';cancelPastEdit.hidden=true;document.querySelector('#past-description-count').value=0;message(pastEventForm,'');
  }

  function startPastEdit(item){
    pastEventForm.elements.id.value=item.id;pastEventForm.elements.title.value=item.title;pastEventForm.elements.date.value=item.date;pastEventForm.elements.description.value=item.description;
    pastEventForm.elements.image.required=false;document.querySelector('#past-image-hint').textContent='(оставьте пустым, чтобы сохранить текущую)';
    document.querySelector('#past-form-title').textContent='Редактирование прошедшего мероприятия';pastEventForm.querySelector('[type=submit]').textContent='Сохранить изменения';
    cancelPastEdit.hidden=false;document.querySelector('#past-description-count').value=item.description.length;message(pastEventForm,'');document.querySelector('.past-editor-panel').scrollIntoView({behavior:'smooth'});
  }

  async function removePastEvent(item){
    if(!confirm(`Удалить мероприятие «${item.title}»?`))return;
    try{await request(`/api/admin/past-events/${encodeURIComponent(item.id)}`,{method:'DELETE'});await loadPastEvents();}
    catch(error){alert(error.message)}
  }

  async function movePhoto(index,direction) {
    const target=index+direction;
    if(target<0||target>=gallery.length)return;
    [gallery[index],gallery[target]]=[gallery[target],gallery[index]];
    renderGallery();
    try { gallery=await request('/api/admin/gallery/order',{method:'PUT',body:JSON.stringify({ids:gallery.map(item=>item.id)})}); renderGallery(); }
    catch(error){ await loadGallery(); alert(error.message); }
  }

  async function removePhoto(item) {
    if(!confirm(`Удалить фотографию${item.caption ? ` «${item.caption}»` : ''}?`))return;
    try { await request(`/api/admin/gallery/${encodeURIComponent(item.id)}`,{method:'DELETE'}); await loadGallery(); }
    catch(error){ alert(error.message); }
  }

  function resetForm() {
    eventForm.reset();
    eventForm.elements.id.value='';
    document.querySelector('#form-title').textContent='Новое мероприятие';
    eventForm.querySelector('[type=submit]').textContent='Опубликовать';
    cancelEdit.hidden=true;
    document.querySelector('#description-count').value=0;
    message(eventForm,'');
  }

  function startEdit(item) {
    eventForm.elements.id.value=item.id;
    eventForm.elements.title.value=item.title;
    eventForm.elements.date.value=item.date;
    eventForm.elements.location.value=item.location || '';
    eventForm.elements.description.value=item.description;
    document.querySelector('#description-count').value=item.description.length;
    document.querySelector('#form-title').textContent='Редактирование мероприятия';
    eventForm.querySelector('[type=submit]').textContent='Сохранить изменения';
    cancelEdit.hidden=false;
    message(eventForm,'');
    document.querySelector('.editor-panel').scrollIntoView({behavior:'smooth'});
  }

  function askDelete(item) {
    pendingDelete=item;
    document.querySelector('#delete-name').textContent=item.title;
    deleteDialog.showModal();
  }

  loginForm.addEventListener('submit', async event => {
    event.preventDefault(); message(loginForm,'');
    const button=loginForm.querySelector('button'); button.disabled=true;
    try {
      await request('/api/admin/login',{method:'POST',body:JSON.stringify({Username:loginForm.elements.username.value,Password:loginForm.elements.password.value})});
      showDashboard(true); loginForm.reset(); await Promise.all([loadEvents(),loadGallery(),loadPastEvents()]);
    } catch(error) { message(loginForm,error.message); }
    finally { button.disabled=false; }
  });

  eventForm.addEventListener('submit', async event => {
    event.preventDefault(); message(eventForm,'');
    const button=eventForm.querySelector('[type=submit]'); button.disabled=true;
    const id=eventForm.elements.id.value;
    const payload={title:eventForm.elements.title.value,date:eventForm.elements.date.value,location:eventForm.elements.location.value,description:eventForm.elements.description.value};
    try {
      await request(id?`/api/admin/events/${encodeURIComponent(id)}`:'/api/admin/events',{method:id?'PUT':'POST',body:JSON.stringify(payload)});
      resetForm(); await loadEvents(); message(eventForm,id?'Изменения сохранены.':'Мероприятие опубликовано.',true);
    } catch(error) {
      if (error.message.includes('вход')) showDashboard(false);
      message(eventForm,error.message);
    } finally { button.disabled=false; }
  });

  galleryForm.addEventListener('submit',async event=>{
    event.preventDefault(); message(galleryForm,'');
    const button=galleryForm.querySelector('button'); button.disabled=true;
    const formData=new FormData(galleryForm);
    try { await request('/api/admin/gallery',{method:'POST',body:formData}); galleryForm.reset(); await loadGallery(); message(galleryForm,'Фотография добавлена в слайд-шоу.',true); }
    catch(error){ if(error.message.includes('вход'))showDashboard(false); message(galleryForm,error.message); }
    finally{button.disabled=false;}
  });

  pastEventForm.addEventListener('submit',async event=>{
    event.preventDefault();message(pastEventForm,'');const button=pastEventForm.querySelector('[type=submit]');button.disabled=true;
    const id=pastEventForm.elements.id.value;const formData=new FormData(pastEventForm);formData.delete('id');
    if(!pastEventForm.elements.image.files.length)formData.delete('image');
    try{await request(id?`/api/admin/past-events/${encodeURIComponent(id)}`:'/api/admin/past-events',{method:id?'PUT':'POST',body:formData});resetPastForm();await loadPastEvents();message(pastEventForm,id?'Изменения сохранены.':'Мероприятие добавлено в архив.',true);}
    catch(error){if(error.message.includes('вход'))showDashboard(false);message(pastEventForm,error.message)}finally{button.disabled=false}
  });

  document.querySelector('#confirm-delete').addEventListener('click', async event => {
    event.preventDefault();
    if (!pendingDelete) return;
    try { await request(`/api/admin/events/${encodeURIComponent(pendingDelete.id)}`,{method:'DELETE'}); deleteDialog.close(); pendingDelete=null; await loadEvents(); }
    catch(error) { deleteDialog.close(); alert(error.message); }
  });
  deleteDialog.addEventListener('close',()=>{pendingDelete=null});
  cancelEdit.addEventListener('click',resetForm);
  cancelPastEdit.addEventListener('click',resetPastForm);
  eventForm.elements.description.addEventListener('input',event=>{document.querySelector('#description-count').value=event.target.value.length});
  pastEventForm.elements.description.addEventListener('input',event=>{document.querySelector('#past-description-count').value=event.target.value.length});
  document.querySelectorAll('.admin-tab').forEach(tab=>tab.addEventListener('click',()=>{
    document.querySelectorAll('.admin-tab').forEach(item=>{const active=item===tab;item.classList.toggle('active',active);item.setAttribute('aria-selected',String(active))});
    document.querySelectorAll('.admin-tab-panel').forEach(panel=>{const active=panel.dataset.panel===tab.dataset.tab;panel.hidden=!active;panel.classList.toggle('active',active)});
  }));
  document.querySelector('#logout').addEventListener('click',async()=>{await request('/api/admin/logout',{method:'POST'});resetForm();showDashboard(false)});

  request('/api/admin/session').then(async state=>{showDashboard(state.authenticated);if(state.authenticated)await Promise.all([loadEvents(),loadGallery(),loadPastEvents()])}).catch(()=>showDashboard(false));
})();
