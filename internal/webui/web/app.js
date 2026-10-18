'use strict';

const $ = (selector, root = document) => root.querySelector(selector);
const $$ = (selector, root = document) => [...root.querySelectorAll(selector)];
const paths = {
  sync: 'M4 8h16m-4-4 4 4-4 4M20 16H4m4 4-4-4 4-4',
  layers: 'm12 3 9 5-9 5-9-5 9-5Zm-9 9 9 5 9-5M3 16l9 5 9-5',
  plus: 'M12 5v14M5 12h14', close: 'm6 6 12 12M6 18 18 6',
  activity: 'M3 12h4l3-8 4 16 3-8h4', alert: 'm12 3 10 18H2L12 3Zm0 5v6m0 3v1',
  shield: 'm12 3 8 3v6c0 5-8 9-8 9s-8-4-8-9V6l8-3Zm-4 9 3 3 5-6',
  refresh: 'M20 7v5h-5M4 17v-5h5m-5 0a8 8 0 0 1 14-5l2 2M4 15l2 2a8 8 0 0 0 14-5',
  folder: 'M3 7a2 2 0 0 1 2-2h5l2 2h7a2 2 0 0 1 2 2v10H3V7Z',
  file: 'M6 3h8l4 4v14H6V3Zm8 0v5h4M9 12h6m-6 4h6',
  server: 'M4 4h16v6H4V4Zm0 10h16v6H4v-6Zm3-7h.01M7 17h.01M11 7h6m-6 10h6',
  key: 'M14 4a6 6 0 1 1-3 11l-3 3H5v3H2v-5l7-7a6 6 0 0 1 5-5Zm2 4h.01',
  search: 'M10.5 3a7.5 7.5 0 1 1 0 15 7.5 7.5 0 0 1 0-15ZM16 16l5 5',
  'arrow-right': 'M4 12h16m-6-6 6 6-6 6', 'arrow-left': 'M20 12H4m6-6-6 6 6 6', 'arrow-up': 'M12 20V4m-6 6 6-6 6 6',
  chevron: 'm9 5 7 7-7 7', info: 'M12 3a9 9 0 1 1 0 18 9 9 0 0 1 0-18Zm0 7v7m0-10v.01',
  route: 'M5 5a2 2 0 1 1 0 4 2 2 0 0 1 0-4Zm14 10a2 2 0 1 1 0 4 2 2 0 0 1 0-4ZM7 7h9a3 3 0 0 1 0 6H8a2 2 0 0 0 0 4h9',
  play: 'm7 4 13 8-13 8V4Z', pause: 'M7 4v16M17 4v16', check: 'm5 12 4 4L19 6',
};
function icon(name) {
  const span = document.createElement('span');
  span.innerHTML = `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.65" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="${paths[name] || paths.folder}"/></svg>`;
  return span;
}
$$('[data-icon]').forEach(node => node.append(icon(node.dataset.icon).firstElementChild));
function el(tag, className = '', text = '') {
  const node = document.createElement(tag); node.className = className; node.textContent = text; return node;
}
function button(text, name, className = 'button') {
  const node = el('button', className); node.type = 'button'; if (name) node.append(icon(name)); node.append(document.createTextNode(text)); return node;
}
function toast(message, error = false) {
  const node = el('div', 'toast' + (error ? ' error' : ''), message); $('#toasts').append(node); setTimeout(() => node.remove(), error ? 10000 : 5000);
}
const fragment = new URLSearchParams(location.hash.slice(1));
if (fragment.get('token')) { sessionStorage.setItem('xsync-token', fragment.get('token')); history.replaceState(null, '', location.pathname + location.search); }
const token = sessionStorage.getItem('xsync-token') || '';
async function api(url, options = {}) {
  const headers = { Authorization: `Bearer ${token}`, ...options.headers };
  if (options.body !== undefined) { headers['Content-Type'] = 'application/json'; options.body = JSON.stringify(options.body); }
  const response = await fetch('/api/' + url, { ...options, headers });
  let result; try { result = await response.json(); } catch { throw new Error('Không đọc được phản hồi từ xsync.'); }
  if (!response.ok) throw new Error(result.error || `Lỗi ${response.status}`);
  return result;
}
const state = { info: null, sessions: [], step: 0, connection: '', host: '', selected: new Map(), browsers: {}, busy: false, previewOK: false, previewVersion: 0, flow: null, detail: '', lastList: '', page: 'sessions', serverBusy: new Set(), serverFeedback: new Map() };
const basename = value => value.replaceAll('\\', '/').split('/').filter(Boolean).at(-1) || value;
const join = (parent, name) => parent.replace(/\/$/, '') + '/' + name;
function parentPath(value) { if (value === '/' || /^[A-Za-z]:\/$/.test(value)) return value; return value.replace(/\/$/, '').replace(/\/[^/]*$/, '') || '/'; }
const timeLabel = value => !value || value.startsWith('0001') ? 'Chưa đồng bộ' : new Intl.DateTimeFormat('vi', { hour: '2-digit', minute: '2-digit', second: '2-digit', day: '2-digit', month: '2-digit' }).format(new Date(value));
function phaseLabel(session) {
  if (!session.running) return ['Đã dừng', ''];
  const phase = session.status.phase;
  return ({ checking: ['Đang kiểm tra thay đổi', 'busy'], syncing: ['Đang đồng bộ', 'busy'], watching: ['Chờ thay đổi', 'good'], retrying: ['Đang thử lại', 'error'], starting: ['Đang khởi động', 'busy'] })[phase] || ['Đang chạy', 'busy'];
}
function badge(session) { const [text, type] = phaseLabel(session); const node = el('span', 'badge ' + type); node.append(el('i'), document.createTextNode(text)); return node; }

function serverMetadata(host) { return state.info?.servers?.find(s => s.host === host) || { host, password_saved: Boolean(state.info?.saved_passwords?.[host]), password_source: state.info?.saved_passwords?.[host] }; }
function authBadges(host) {
  const server = serverMetadata(host), badges = el('span', 'auth-badges');
  if (server.key_verified_at) {
    const key = el('span', 'auth-tag verified', 'Đã dùng key'); key.title = 'Xác thực thành công: ' + timeLabel(server.key_verified_at); badges.append(key);
  }
  if (server.password_saved) badges.append(el('span', 'auth-tag', 'Đã lưu mật khẩu'));
  return badges;
}
async function refreshInfo() {
  state.info = await api('info');
  renderServers(); renderHosts(); updateAuthFields();
}
async function showPage(page) {
  if (page === 'new') { openWizard(); return; }
  if (state.page === 'new') discardWizard();
  if (page !== 'details') state.detail = '';
  setPage(page);
  if (page === 'servers') { try { await refreshInfo(); } catch (err) { toast(err.message, true); } }
}
function setPage(page) {
  state.page = page;
  for (const [id, name] of [['sessions-page', 'sessions'], ['servers-page', 'servers'], ['wizard', 'new'], ['details', 'details']]) $('#' + id).hidden = page !== name;
  $('#nav-sessions').classList.toggle('active', page === 'sessions' || page === 'details');
  $('#nav-new-session').classList.toggle('active', page === 'new'); $('#nav-servers').classList.toggle('active', page === 'servers');
  document.title = 'xsync · ' + (({ new: 'Tạo phiên đồng bộ', details: 'Chi tiết phiên', servers: 'Máy chủ' })[page] || 'Phiên đồng bộ');
  const url = new URL(location.href); if (page === 'sessions') url.searchParams.delete('page'); else url.searchParams.set('page', page);
  if (page === 'details') url.searchParams.set('session', state.detail); else url.searchParams.delete('session');
  history.replaceState(null, '', url.pathname + url.search);
}
function renderServers() {
  const list = $('#server-settings');
  const drafts = new Map($$('input[data-server-password]', list).map(input => [input.dataset.serverPassword, input.value]));
  list.replaceChildren();
  const hosts = state.info?.hosts || [];
  if (!hosts.length) { list.append(el('p', 'browser-message', 'Thêm Host vào ~/.ssh/config rồi làm mới.')); return; }
  for (const [index, host] of hosts.entries()) {
    const server = serverMetadata(host), card = el('article', 'server-card'), heading = el('div', 'server-card-heading'), title = el('div');
    card.dataset.host = host;
    title.append(el('h2', '', host), authBadges(host)); heading.append(icon('server'), title); card.append(heading);
    card.append(el('p', 'muted server-key-history', server.key_verified_at ? 'Lần xác thực key thành công: ' + timeLabel(server.key_verified_at) : 'Chưa ghi nhận xác thực bằng key.'));
    const check = button('Kiểm tra key', 'key'); check.disabled = state.serverBusy.has(host);
    check.onclick = () => updateServer(host, check, null, 'key');
    const connection = button('Kiểm tra kết nối', 'activity'); connection.disabled = state.serverBusy.has(host);
    connection.onclick = () => updateServer(host, connection, null, 'connection');
    const checks = el('div', 'server-actions'); checks.append(connection, check); card.append(checks);
    card.append(el('p', 'muted small server-check-hint', server.password_saved ? 'Ưu tiên key; thử mật khẩu đã lưu nếu key không kết nối được.' : 'Kiểm tra kết nối bằng SSH key.'));
    const form = el('form', 'server-password-form'), label = el('label', 'field-label', server.password_saved ? 'Thay mật khẩu' : 'Lưu mật khẩu'), input = el('input', 'text-input');
    input.id = 'server-password-' + index; input.type = 'password'; input.autocomplete = 'new-password'; input.required = true; input.dataset.serverPassword = host; input.value = drafts.get(host) || ''; input.disabled = state.serverBusy.has(host);
    input.placeholder = server.password_saved ? 'Nhập mật khẩu mới để thay thế' : 'Nhập mật khẩu của tài khoản trên server'; label.htmlFor = input.id;
    const source = el('p', 'muted small', server.password_saved ? 'Đang dùng mật khẩu từ: ' + server.password_source : 'Chưa có mật khẩu được lưu.');
    const actions = el('div', 'server-actions'), save = button('Lưu mật khẩu', null, 'button primary'); save.type = 'submit'; save.disabled = state.serverBusy.has(host); actions.append(save);
    if (server.password_saved) {
      const clear = button('Ngừng dùng mật khẩu', null, 'button subtle'); clear.disabled = state.serverBusy.has(host);
      clear.onclick = () => updateServer(host, clear, ''); actions.append(clear);
    }
    form.append(label, input, source, actions); form.onsubmit = event => { event.preventDefault(); if (input.value) updateServer(host, save, input.value); };
    card.append(form, el('p', 'server-feedback')); applyServerFeedback(card, state.serverFeedback.get(host)); list.append(card);
  }
  list.append(el('p', 'browser-message server-no-matches', 'Không có máy chủ khớp tìm kiếm.'));
  filterServers();
}
function filterServers() {
  const query = $('#server-search').value.toLowerCase(), cards = $$('.server-card');
  cards.forEach(card => { card.hidden = !card.dataset.host.toLowerCase().includes(query); });
  const empty = $('.server-no-matches'); if (empty) empty.hidden = cards.some(card => !card.hidden);
}
function applyServerFeedback(card, result) {
  const feedback = $('.server-feedback', card);
  feedback.textContent = result?.message || '';
  feedback.classList.toggle('error', Boolean(result?.failed)); feedback.classList.toggle('warning', Boolean(result?.warning));
  feedback.setAttribute('role', result?.failed ? 'alert' : 'status');
}
async function updateServer(host, node, password, check = '') {
  if (state.serverBusy.has(host)) return;
  state.serverBusy.add(host);
  const card = node.closest('.server-card');
  $$('button,input', card).forEach(control => { control.disabled = true; });
  state.serverFeedback.set(host, { message: check === 'connection' ? 'Đang kiểm tra kết nối SSH và rsync…' : check ? 'Đang kiểm tra SSH key…' : 'Đang lưu…' });
  applyServerFeedback(card, state.serverFeedback.get(host));
  let message, failed = false, warning = false;
  try {
    const path = 'servers/' + encodeURIComponent(host);
    const result = await api(path + (check === 'connection' ? '/check-connection' : check ? '/check-key' : ''), { method: check ? 'POST' : 'PUT', ...(check ? {} : { body: { password } }) });
    if (!check) $('input[data-server-password]', card).value = '';
    if (check === 'connection') {
      failed = !result.ssh_ok; warning = result.ssh_ok && (!result.sync_ready || Boolean(result.warning));
      message = result.ssh_ok ? 'SSH: kết nối thành công bằng ' + (result.auth === 'key' ? 'key.' : 'mật khẩu đã lưu.') + '\n' + (result.sync_ready ? 'Rsync: sẵn sàng đồng bộ.' : 'Rsync: chưa sẵn sàng đồng bộ. ' + result.sync_error) : 'SSH: kết nối thất bại.\n' + result.error;
      if (result.warning) message += '\n' + result.warning;
    } else {
      message = check ? 'Đã xác thực bằng key và ghi nhớ máy chủ.' : password === '' ? 'Đã ngừng dùng mật khẩu cho kết nối mới.' : 'Đã lưu mật khẩu. Mật khẩu chưa được kiểm tra với server.';
    }
  } catch (err) { message = err.message; failed = true; }
  finally {
    state.serverBusy.delete(host);
    state.serverFeedback.set(host, { message, failed, warning });
    try { await refreshInfo(); } catch (err) { toast(err.message, true); $$('button,input', card).forEach(control => { control.disabled = false; }); }
    const current = $$('.server-card').find(c => c.dataset.host === host);
    if (current) applyServerFeedback(current, state.serverFeedback.get(host));
  }
}

function setLocalConnection(connected) {
  const label = $('#server-state'), status = label.parentElement;
  const text = connected ? 'Đã kết nối local' : 'Mất kết nối';
  if (label.textContent !== text) label.textContent = text;
  status.dataset.state = connected ? 'connected' : 'disconnected';
  status.title = text + '. Kết nối giữa trình duyệt và xsync trên máy này.';
}
async function poll() {
  try {
    state.sessions = await api('sessions');
    setLocalConnection(true);
    const snapshot = JSON.stringify(state.sessions);
    if (snapshot !== state.lastList) { state.lastList = snapshot; renderSessions(); }
    if (state.detail && !$('#details').hidden) { renderDetail(); await refreshLogs(); }
  } catch (err) { setLocalConnection(false); if (!state.info) renderLoadError(err.message); }
}
function renderLoadError(message) {
  const panel = el('div', 'empty-state'); panel.append(el('h3', '', 'Chưa kết nối được với xsync'), el('p', '', message)); $('#session-list').replaceChildren(panel);
}
function renderSessions() {
  $('#stat-total').textContent = state.sessions.length; $('#nav-count').textContent = state.sessions.length;
  $('#stat-active').textContent = state.sessions.filter(s => s.running).length;
  $('#stat-errors').textContent = state.sessions.filter(s => s.running && s.status.phase === 'retrying').length;
  const list = $('#session-list'); list.replaceChildren();
  if (!state.sessions.length) {
    const empty = el('div', 'empty-state'), mark = el('div', 'empty-icon'); mark.append(icon('folder'));
    const create = button('Tạo phiên đầu tiên', 'plus', 'button primary'); create.onclick = openWizard;
    empty.append(mark, el('h3', '', 'Chưa có phiên đồng bộ'), create); list.append(empty); return;
  }
  for (const session of state.sessions) {
    const row = el('article', 'session-row'), main = el('div', 'session-main'), folder = el('span', 'session-folder'); folder.append(icon('folder'));
    const title = el('div'), source = el('p', '', session.local_dir); source.title = session.local_dir;
    title.append(el('h3', '', basename(session.local_dir)), source); main.append(folder, title);
    const host = el('div', 'session-host'); host.append(el('strong', '', session.host), el('small', '', session.auth === 'key' ? 'Xác thực bằng SSH key' : 'Xác thực bằng mật khẩu'));
    const status = el('div', 'session-status'); status.append(badge(session));
    const progress = syncProgress(session); if (progress) status.append(progress);
    status.append(el('small', 'last-sync', timeLabel(session.status.last_success)));
    const actions = el('div', 'row-actions'), toggle = button(session.running ? 'Dừng' : 'Bắt đầu', session.running ? 'pause' : 'play');
    toggle.onclick = () => controlSession(session, toggle);
    const details = button('Chi tiết', null, 'button subtle'); details.onclick = () => openDetails(session.id);
    actions.append(toggle, details); row.append(main, host, status, actions); list.append(row);
  }
}
async function controlSession(session, node, autostart = false) {
  node.disabled = true;
  try {
    let password = '';
    if (!session.running && session.auth === 'password' && !session.password_saved && !autostart) {
      password = await promptText('Mật khẩu SSH', 'Mật khẩu chỉ giữ trong bộ nhớ cho lần chạy này.', 'password'); if (password === null) return;
    }
    await api(`sessions/${session.id}/${session.running ? 'stop' : 'start'}`, { method: 'POST', body: session.running ? {} : { autostart, password } });
    toast(session.running ? 'Đã dừng phiên đồng bộ.' : 'Đã khởi chạy phiên đồng bộ.'); await poll();
  } catch (err) { toast(err.message, true); } finally { node.disabled = false; }
}

function discardWizard() {
  state.flow?.abort(); state.previewVersion++;
  Object.values(state.browsers).forEach(b => b.dispose());
  if (state.connection) api(`connections/${state.connection}`, { method: 'DELETE' }).catch(() => {});
  state.connection = ''; state.flow = null; state.browsers = {}; $('#ssh-password').value = '';
}
function closeWizard() {
  discardWizard(); showPage('sessions'); $('#new-session').focus();
}
function openWizard() {
  if (!state.info) { toast('Đợi kết nối với xsync hoàn tất.', true); return; }
  if (state.page === 'new') return;
  discardWizard(); state.flow = new AbortController(); state.busy = false; state.step = 0; state.host = ''; state.selected = new Map(); state.previewOK = false;
  setPage('new'); $('#wizard-error').hidden = true; $('#host-search').value = ''; $('#ssh-password').value = '';
  $('input[name=auth][value=key]').checked = true; $('#password-fields').hidden = true; $('#save-password').checked = true;
  $('#select-all').checked = false; $('#autostart').checked = true; $('#start-now').checked = true; $('#confirm-mirror').checked = false; $('#connect-result').hidden = true;
  $('input[name="selection-mode"][value="include"]').checked = true;
  setPane('connect-panes', 'hosts'); setPane('folder-panes', 'local'); setPane('review-panes', 'settings');
  renderHosts(); updateAuthFields(); renderStep(); $('#host-search').focus();
}
function renderHosts() {
  const list = $('#host-list'); list.replaceChildren();
  const names = (state.info.hosts || []).filter(h => h.toLowerCase().includes($('#host-search').value.toLowerCase()));
  if (!names.length) { list.append(el('p', 'browser-message', state.info.hosts?.length ? 'Không có server khớp tìm kiếm.' : 'Thêm Host vào ~/.ssh/config rồi làm mới giao diện.')); return; }
  for (const host of names) {
    const node = button(host, 'server', 'host-option' + (state.host === host ? ' selected' : ''));
    node.append(authBadges(host));
    if (state.host === host) node.append(icon('check'));
    node.onclick = () => {
      if (state.host !== host) {
        resetConnection(); $('#ssh-password').value = ''; state.host = host;
        const saved = Boolean(state.info.saved_passwords?.[host]);
        $('input[name=auth][value=' + (saved && !serverMetadata(host).key_verified_at ? 'password' : 'key') + ']').checked = true;
        $('#use-saved-password').checked = saved;
      }
      updateAuthFields(); renderHosts();
    }; list.append(node);
  }
}
function updateAuthFields() {
  const source = state.info.saved_passwords?.[state.host];
  const passwordMode = $('input[name=auth]:checked').value === 'password';
  $('#password-fields').hidden = !passwordMode;
  $('#saved-password-option').hidden = !source; $('#saved-password-source').hidden = !source;
  $('#saved-password-source').textContent = source ? 'Đã lưu trong ' + source + '. Mật khẩu được đọc trực tiếp trên máy này.' : '';
  if (!source) $('#use-saved-password').checked = false;
  const useSaved = Boolean(source) && $('#use-saved-password').checked;
  $('#ssh-password').disabled = useSaved;
  $('#ssh-password').placeholder = useSaved ? 'Đang dùng mật khẩu đã lưu' : 'Nhập mật khẩu';
  updateStartupHint();
}
function resetConnection() {
  if (state.connection) api(`connections/${state.connection}`, { method: 'DELETE' }).catch(() => {});
  state.connection = ''; state.previewOK = false; Object.values(state.browsers).forEach(b => b.dispose()); state.browsers = {}; $('#connect-result').hidden = true;
}
function renderStep() {
  $('#wizard').dataset.step = state.step;
  $$('[data-page]').forEach(page => page.hidden = Number(page.dataset.page) !== state.step);
  $$('[data-step]').forEach(step => { step.classList.toggle('current', Number(step.dataset.step) === state.step); step.classList.toggle('complete', Number(step.dataset.step) < state.step); if (Number(step.dataset.step) === state.step) step.setAttribute('aria-current', 'step'); else step.removeAttribute('aria-current'); });
  $('#wizard-back').textContent = state.step ? 'Quay lại' : 'Hủy'; $('#step-caption').textContent = `Bước ${state.step + 1} trên 4`;
  $('#wizard-route').hidden = state.step !== 1;
  const next = $('#wizard-next'); next.replaceChildren(document.createTextNode(['Kết nối và tiếp tục', 'Chọn dữ liệu', 'Xem trước thay đổi', 'Tạo phiên đồng bộ'][state.step]), icon('arrow-right')); next.disabled = state.busy;
  if (state.step === 1 && !state.browsers.local) {
    state.browsers.local = new Browser($('#local-browser'), { title: 'Thư mục local A', subtitle: 'Trên máy này', remote: false, dirsOnly: true, onChange: updateRoute });
    state.browsers.remote = new Browser($('#remote-browser'), { title: 'Thư mục chứa B', subtitle: state.host, remote: true, dirsOnly: true, onChange: updateRoute });
    state.browsers.local.go(state.info.cwd); state.browsers.remote.go('');
  }
  if (state.step === 2) {
    const root = state.browsers.local.path;
    if (!state.browsers.selection || state.browsers.selection.root !== root) {
      state.selected.clear(); state.browsers.selection?.dispose();
      state.browsers.selection = new Browser($('#selection-browser'), { title: basename(root), subtitle: 'Dữ liệu trong thư mục nguồn', remote: false, root, selectable: true }); state.browsers.selection.go(root);
    }
    renderSelections();
  }
  if (state.step === 3) {
    $('#review-source').textContent = state.browsers.local.path; $('#review-host').textContent = state.host;
    $('#review-target').textContent = join(state.browsers.remote.path, basename(state.browsers.local.path)); updateStartupHint();
    $('#review-scope').textContent = scopeDescription(selectionMode(), [...state.selected.keys()]);
    $('#scope-notice').textContent = selectionMode() === 'exclude' ? 'Đồng bộ cả mục mới và thao tác xóa ngoài danh sách loại trừ. Dữ liệu bị loại trừ trên remote được giữ nguyên.' : 'Chỉ đồng bộ và phản chiếu thao tác xóa trong mục đã chọn. Mục mới ngoài phạm vi này không được đồng bộ.';
  }
}
function draft() {
  return { connection: state.connection, local_dir: state.browsers.local.path, remote_parent: state.browsers.remote.path, selections: [...state.selected].map(([path, directory]) => ({ path, directory })), selection_mode: selectionMode(), save_password: $('#save-password').checked, start: $('#start-now').checked, autostart: $('#autostart').checked };
}
async function nextStep() {
  if (state.busy) return;
  const flow = state.flow;
  let attemptedKey = false;
  state.busy = true; $('#wizard-next').disabled = true; $('#wizard-error').hidden = true;
  try {
    if (state.step === 0) {
      if (!state.host) throw new Error('Chọn server bạn muốn kết nối.');
      if (!state.connection) {
        const auth = $('input[name=auth]:checked').value;
        attemptedKey = auth === 'key';
        if (auth === 'password' && !$('#use-saved-password').checked && !$('#ssh-password').value) throw new Error('Nhập mật khẩu SSH để kết nối.');
        $('#wizard-next').replaceChildren(el('span', 'spinner'), document.createTextNode('Đang kết nối…'));
        const result = await api('connections', { method: 'POST', body: { host: state.host, auth, password: auth === 'password' && !$('#use-saved-password').checked ? $('#ssh-password').value : '', use_saved_password: auth === 'password' && $('#use-saved-password').checked }, signal: flow.signal });
        if (flow !== state.flow || flow.signal.aborted) { api(`connections/${result.id}`, { method: 'DELETE' }).catch(() => {}); return; }
        state.connection = result.id; $('#connect-result').hidden = false; $('#connect-result').textContent = 'Đã kết nối với ' + state.host;
      }
      state.step = 1; renderStep();
    } else if (state.step === 1) {
      if (!state.browsers.local.path || !state.browsers.remote.path || state.browsers.local.loading || state.browsers.remote.loading || state.browsers.local.error || state.browsers.remote.error) throw new Error('Cần tải thành công cả hai thư mục trước khi tiếp tục.');
      if (parentPath(state.browsers.local.path) === state.browsers.local.path) throw new Error('Chọn một thư mục bên dưới gốc ổ đĩa.');
      state.step = 2; renderStep();
    } else if (state.step === 2) {
      if (selectionMode() === 'include' && !state.selected.size) throw new Error('Chọn ít nhất một file/thư mục, hoặc chọn toàn bộ thư mục A.');
      state.step = 3; state.previewOK = false; $('#confirm-mirror').checked = false; renderStep(); await preview();
    } else {
      if (!state.previewOK) throw new Error('Cần xem trước thay đổi thành công trước khi tạo phiên.');
      if (!$('#confirm-mirror').checked) throw new Error('Xác nhận phạm vi phản chiếu thao tác xóa để tiếp tục.');
      $('#wizard-next').replaceChildren(el('span', 'spinner'), document.createTextNode('Đang tạo phiên…'));
      const result = await api('sessions', { method: 'POST', body: draft(), signal: state.flow.signal });
      closeWizard(); toast(result.warning || 'Đã tạo phiên đồng bộ.', Boolean(result.warning)); await poll();
    }
  } catch (err) { if (err.name !== 'AbortError') { $('#wizard-error').textContent = err.message; $('#wizard-error').hidden = false; } }
  finally {
    if (attemptedKey && !flow.signal.aborted) { try { await refreshInfo(); } catch (err) { toast(err.message, true); } }
    if (flow === state.flow) { state.busy = false; if (!$('#wizard').hidden) renderStep(); }
  }
}
async function preview() {
  const version = ++state.previewVersion; state.previewOK = false; $('#preview-output').textContent = 'Đang kết nối và quét thay đổi…'; $('#refresh-preview').disabled = true;
  try {
    const result = await api('preview', { method: 'POST', body: draft(), signal: state.flow.signal });
    if (version !== state.previewVersion) return;
    $('#preview-output').textContent = result.output.trim() || 'Không có thay đổi cần đồng bộ.'; state.previewOK = true;
  } catch (err) { if (err.name !== 'AbortError') { $('#preview-output').textContent = err.message; throw err; } }
  finally { $('#refresh-preview').disabled = false; }
}
function updateRoute() {
  const a = state.browsers.local?.path, b = state.browsers.remote?.path;
  const route = $('#route-text');
  route.textContent = a && b ? `${a} → ${state.host}:${join(b, basename(a))}` : 'Chọn hai thư mục để xem đường dẫn đích';
  route.title = route.textContent;
}
function updateStartupHint() {
  const auth = $('input[name=auth]:checked').value;
  const allowed = auth === 'key' || $('#save-password').checked;
  $('#autostart').disabled = !allowed || !$('#start-now').checked;
  if (!allowed || !$('#start-now').checked) $('#autostart').checked = false;
  $('#startup-hint').textContent = !allowed ? 'Không lưu mật khẩu: phiên chạy khi tiến trình giao diện web còn hoạt động, kể cả khi đóng tab.' : state.info.platform === 'linux' ? 'Tự khởi động dùng systemd của tài khoản và bật linger. Nếu bỏ chọn, phiên chạy khi giao diện web còn hoạt động.' : 'Tự chạy khi đăng nhập tài khoản. Nếu bỏ chọn, phiên chạy khi giao diện web còn hoạt động.';
}
function isCovered(relative) { for (const [selected, dir] of state.selected) if (selected === '.' || selected === relative || (dir && relative.startsWith(selected + '/'))) return true; return false; }
function toggleSelection(relative, dir) {
  if (state.selected.has(relative)) state.selected.delete(relative);
  else if (isCovered(relative)) toast('Bỏ chọn thư mục cha trước nếu muốn chọn riêng các mục con.');
  else { if (dir) for (const old of state.selected.keys()) if (relative === '.' || old.startsWith(relative + '/')) state.selected.delete(old); state.selected.set(relative, dir); }
  renderSelections(); state.browsers.selection?.paintList();
}
function renderSelections() {
  const excluded = selectionMode() === 'exclude';
  $('#select-all').parentElement.hidden = excluded;
  $('#select-all').checked = state.selected.has('.'); $('#selection-count').textContent = excluded ? (state.selected.size ? `${state.selected.size} mục loại trừ` : 'Không loại trừ mục nào: đồng bộ toàn bộ') : `${state.selected.size} mục đã chọn`;
  const chips = $('#selected-chips'); chips.replaceChildren();
  for (const [path] of state.selected) { const chip = el('span', 'chip', path === '.' ? 'Toàn bộ thư mục A' : path); const remove = button('×', null); remove.setAttribute('aria-label', 'Bỏ chọn ' + path); remove.onclick = () => { state.selected.delete(path); renderSelections(); state.browsers.selection?.paintList(); }; chip.append(remove); chips.append(chip); }
}
function selectionMode() { return $('input[name="selection-mode"]:checked').value; }
function scopeDescription(mode, paths) {
  if (mode === 'exclude') return paths.length ? 'Đồng bộ tất cả, trừ: ' + paths.join(', ') : 'Đồng bộ toàn bộ, kể cả mục mới';
  return 'Chỉ đồng bộ: ' + paths.map(path => path === '.' ? 'Toàn bộ thư mục' : path).join(', ');
}

let browserSequence = 0;
class Browser {
  constructor(container, options) {
    this.container = container; this.options = options; this.path = ''; this.root = options.root || ''; this.entries = []; this.cache = new Map(); this.serial = 0; this.loading = false; this.filter = ''; this.error = ''; this.controller = null; this.filtered = []; this.window = null;
    this.rowHeight = parseFloat(getComputedStyle(container).getPropertyValue('--file-row-height')) || 52;
    container.replaceChildren();
    const heading = el('div', 'browser-title'); heading.append(icon(options.remote ? 'server' : 'folder'), document.createTextNode(options.title), el('small', '', options.subtitle));
    const tools = el('div', 'browser-tools'); const up = button('', 'arrow-up', 'icon-button'); up.title = 'Lên thư mục cha'; up.setAttribute('aria-label', 'Lên thư mục cha ' + options.title); up.onclick = () => { if (this.path !== this.root) this.go(parentPath(this.path)); };
    this.pathInput = el('input', 'browser-path'); this.pathInput.setAttribute('aria-label', 'Đường dẫn ' + options.title); this.pathInput.title = 'Nhập đường dẫn và nhấn Enter';
    this.pathInput.addEventListener('keydown', event => { if (event.key === 'Enter') { event.preventDefault(); this.go(this.pathInput.value.trim()); } });
    const refresh = button('', 'refresh', 'icon-button'); refresh.title = 'Tải lại'; refresh.setAttribute('aria-label', 'Tải lại ' + options.title); refresh.onclick = () => this.go(this.path, true); tools.append(up, this.pathInput, refresh);
    if (options.remote) {
      const mkdir = button('Tạo thư mục', 'plus'); mkdir.title = 'Tạo thư mục remote'; mkdir.setAttribute('aria-label', 'Tạo thư mục remote'); mkdir.onclick = () => this.toggleMkdir(); heading.append(mkdir);
      this.mkdirForm = el('form', 'inline-folder-form'); this.mkdirForm.hidden = true;
      const label = el('label', '', 'Tên thư mục mới'); this.mkdirInput = el('input', 'text-input'); this.mkdirInput.id = 'new-folder-' + ++browserSequence; this.mkdirInput.required = true; this.mkdirInput.autocomplete = 'off'; this.mkdirInput.placeholder = 'Nhập tên thư mục'; label.htmlFor = this.mkdirInput.id;
      const fields = el('div', 'inline-folder-fields'), create = button('Tạo thư mục', null, 'button primary'), cancel = button('Hủy', null, 'button subtle'); create.type = 'submit';
      cancel.onclick = () => { this.mkdirForm.hidden = true; this.mkdirInput.value = ''; mkdir.focus(); };
      this.mkdirError = el('p', 'inline-folder-error'); this.mkdirError.setAttribute('role', 'alert');
      fields.append(this.mkdirInput, create, cancel); this.mkdirForm.append(label, fields, this.mkdirError);
      this.mkdirForm.onsubmit = event => { event.preventDefault(); this.mkdir(); };
      this.mkdirInput.onkeydown = event => { if (event.key === 'Escape') { event.preventDefault(); cancel.click(); } };
    }
    const search = el('label', 'browser-search'); const input = el('input'); input.placeholder = 'Tìm trong thư mục này…'; input.setAttribute('aria-label', 'Tìm trong ' + options.title); input.oninput = () => { this.filter = input.value.toLowerCase(); this.list.scrollTop = 0; this.paintList(); }; search.append(icon('search'), input);
    this.progress = el('div', 'loading-line'); this.progress.hidden = true; this.list = el('div', 'browser-list'); this.footer = el('div', 'browser-footer');
    this.list.tabIndex = 0; this.list.setAttribute('role', 'region'); this.list.setAttribute('aria-label', 'Danh sách ' + options.title.toLowerCase());
    this.list.addEventListener('scroll', () => { if (this.window) this.paintViewport(); });
    container.append(heading, tools); if (this.mkdirForm) container.append(this.mkdirForm); container.append(search, this.progress, this.list, this.footer);
    if (typeof ResizeObserver !== 'undefined') { this.observer = new ResizeObserver(() => { if (this.window) this.paintViewport(); }); this.observer.observe(this.list); }
  }
  abort() { this.controller?.abort(); this.serial++; }
  dispose() { this.abort(); this.mkdirController?.abort(); this.observer?.disconnect(); }
  async go(directory, force = false) {
    if (this.mkdirForm) { this.mkdirForm.hidden = true; this.mkdirInput.value = ''; }
    const serial = ++this.serial; this.controller?.abort(); this.controller = new AbortController(); this.loading = true; this.error = ''; this.progress.hidden = false; this.paintList();
    try {
      let listing = this.cache.get(directory);
      if (!listing || force) {
        const params = new URLSearchParams({ path: directory }); if (this.root) params.set('root', this.root); if (this.options.remote) params.set('connection', state.connection);
        listing = await api((this.options.remote ? 'remote?' : 'local?') + params, { signal: this.controller.signal });
        if (serial !== this.serial) return;
        if (this.cache.size >= 128) this.cache.clear(); this.cache.set(listing.Path, listing);
      }
      if (serial !== this.serial) return;
      this.path = listing.Path; this.pathInput.value = this.path; this.entries = listing.Entries || []; this.list.scrollTop = 0; this.options.onChange?.();
    } catch (err) { if (err.name !== 'AbortError' && serial === this.serial) { this.error = err.message; this.pathInput.value = this.path; } }
    finally { if (serial === this.serial) { this.loading = false; this.progress.hidden = true; this.paintList(); } }
  }
  paintList() {
    const scroll = this.list.scrollTop; this.window = null;
    this.list.replaceChildren();
    if (this.loading) { this.list.append(el('div', 'browser-message', 'Đang tải thư mục…')); return; }
    if (this.error) { this.list.append(el('div', 'browser-message error', this.error)); return; }
    this.filtered = this.entries.filter(item => (!this.options.dirsOnly || item.Dir) && item.Name.toLowerCase().includes(this.filter));
    if (!this.filtered.length) this.list.append(el('div', 'browser-message', this.filter ? 'Không có mục khớp tìm kiếm.' : 'Thư mục này đang trống.'));
    else {
      const space = el('div', 'browser-virtual'); space.style.height = `${this.filtered.length * this.rowHeight}px`;
      this.window = el('div', 'browser-window'); space.append(this.window); this.list.append(space); this.list.scrollTop = scroll; this.paintViewport();
    }
    this.footer.replaceChildren(el('span', '', `${this.filtered.length} mục`), el('span', '', this.options.selectable ? (selectionMode() === 'exclude' ? 'Chọn ô để loại trừ · Mở thư mục để chọn sâu hơn' : 'Chọn ô để đồng bộ · Mở thư mục để chọn sâu hơn') : 'Thư mục đang mở được dùng làm đích chọn'));
  }
  paintViewport() {
    const start = Math.max(0, Math.floor(this.list.scrollTop / this.rowHeight) - 4), end = Math.min(this.filtered.length, start + Math.ceil((this.list.clientHeight || 420) / this.rowHeight) + 10);
    this.window.style.transform = `translateY(${start * this.rowHeight}px)`;
    const fragment = document.createDocumentFragment();
    for (const item of this.filtered.slice(start, end)) {
      const row = el('div', 'file-row');
      if (this.options.selectable) {
        const checkbox = el('input'); checkbox.type = 'checkbox'; const relative = join(this.path, item.Name).slice(this.root.replace(/\/$/, '').length + 1); checkbox.checked = isCovered(relative); checkbox.setAttribute('aria-label', 'Chọn ' + item.Name); checkbox.onchange = () => toggleSelection(relative, item.Dir); row.append(checkbox);
      }
      row.append(icon(item.Dir ? 'folder' : 'file')); const name = el('span', 'file-name', item.Name); name.title = item.Name; row.append(name);
      if (item.Dir) { const open = button('', 'chevron', 'file-enter'); open.setAttribute('aria-label', 'Mở ' + item.Name); open.onclick = () => this.go(join(this.path, item.Name)); row.append(open); row.tabIndex = 0; row.setAttribute('aria-label', 'Thư mục ' + item.Name); row.addEventListener('keydown', e => { if (e.key === 'Enter') this.go(join(this.path, item.Name)); }); row.onclick = e => { if (!e.target.closest('input,button')) this.go(join(this.path, item.Name)); }; }
      fragment.append(row);
    }
    this.window.replaceChildren(fragment);
  }
  toggleMkdir() {
    if (!this.path || this.loading || this.error) return;
    this.mkdirForm.hidden = !this.mkdirForm.hidden; this.mkdirError.textContent = ''; this.mkdirInput.value = '';
    if (!this.mkdirForm.hidden) this.mkdirInput.focus();
  }
  async mkdir() {
    if (!this.path || this.creating) return;
    const name = this.mkdirInput.value;
    if (!name.trim() || name === '.' || name === '..' || /[/\\\r\n\x00]/.test(name)) { this.mkdirError.textContent = 'Nhập tên thư mục, không chứa dấu / hoặc \\.'; return; }
    this.creating = true; this.mkdirController = new AbortController(); this.mkdirError.textContent = '';
    $$('input,button', this.mkdirForm).forEach(node => { node.disabled = true; });
    try {
      const directory = join(this.path, name);
      await api('remote/mkdir', { method: 'POST', body: { connection: state.connection, path: directory }, signal: this.mkdirController.signal });
      if (this.mkdirController.signal.aborted) return;
      this.cache.clear(); await this.go(directory, true); this.pathInput.focus(); toast('Đã tạo thư mục.');
    } catch (err) { if (err.name !== 'AbortError') this.mkdirError.textContent = err.message; }
    finally { this.creating = false; $$('input,button', this.mkdirForm).forEach(node => { node.disabled = false; }); }
  }
}

function openDetails(id) { if (state.page === 'new') discardWizard(); state.detail = id; setPage('details'); renderDetail(); $('#details-title').focus(); refreshLogs().catch(err => toast(err.message, true)); }
function syncProgress(session, detailed = false) {
  const p = session.status.progress; if (!p) return null;
  const labels = { connecting: 'Đang kết nối', scanning: 'Đang quét thay đổi', renaming: 'Đang xử lý đổi tên', transferring: 'Đang đồng bộ', finishing: 'Đang hoàn tất', completed: 'Đã đồng bộ' };
  const interrupted = session.status.phase === 'retrying' || (!session.running && p.stage !== 'completed');
  const label = interrupted ? (session.running ? 'Chờ thử lại' : 'Đã dừng') : (labels[p.stage] || 'Đang chuẩn bị');
  const percent = Number.isFinite(p.percent) ? Math.max(0, Math.min(p.stage === 'completed' ? 100 : 99, p.percent)) : null;
  const box = el('div', `sync-progress${detailed ? ' detailed' : ''}${interrupted ? ' interrupted' : ''}`);
  const heading = el('div', 'progress-heading');
  heading.append(el('span', '', label), el('span', 'progress-percent', percent === null ? '' : `${p.estimated ? '~' : ''}${percent}%`)); box.append(heading);
  const track = el('div', `progress-track${percent === null ? ' indeterminate' : ''}`); track.setAttribute('role', 'progressbar'); track.setAttribute('aria-label', label); track.setAttribute('aria-valuemin', '0'); track.setAttribute('aria-valuemax', '100');
  if (percent !== null) track.setAttribute('aria-valuenow', String(percent));
  const fill = el('span', 'progress-fill'); if (percent !== null) fill.style.width = `${percent}%`; track.append(fill); box.append(track);
  const parts = [];
  if (p.bytes_processed > 0) {
    let bytes = p.bytes_processed, unit = 0; const units = ['B', 'KiB', 'MiB', 'GiB', 'TiB'];
    while (bytes >= 1024 && unit < units.length - 1) { bytes /= 1024; unit++; }
    parts.push(`${bytes.toLocaleString('vi-VN', { maximumFractionDigits: unit ? 1 : 0 })} ${units[unit]} đã xử lý`);
  }
  if (!interrupted && p.stage === 'transferring') { if (p.rate) parts.push(p.rate); if (p.eta) parts.push(`Còn khoảng ${p.eta}`); }
  if (p.stage === 'renaming' && p.total > 0) parts.push(`${p.checked}/${p.total} mục`);
  if (detailed && p.transfers > 0) parts.push(`${p.transfers} tệp đã truyền`);
  if (parts.length) box.append(el('small', 'progress-metrics', parts.join(' · ')));
  return box;
}
function renderDetail() {
  const session = state.sessions.find(s => s.id === state.detail); if (!session) return;
  $('#details-title').textContent = basename(session.local_dir);
  const content = $('#detail-content'); content.replaceChildren();
  const progress = syncProgress(session, true); if (progress) content.append(progress);
  const grid = el('dl', 'detail-grid');
  const pairs = [['Server', session.host], ['Trạng thái', phaseLabel(session)[0]], ['Thư mục local', session.local_dir], ['Thư mục remote', session.target], ['Lần đồng bộ thành công', timeLabel(session.status.last_success)], ['Kiểm tra thay đổi gần nhất', timeLabel(session.status.last_checked)], ['Cách xác thực', session.auth === 'key' ? 'SSH key / agent' : session.password_saved ? 'Mật khẩu đã lưu' : 'Mật khẩu không lưu'], ['Phạm vi', scopeDescription(session.selection_mode || 'include', (session.selections || []).map(s => s.path))], ['Mã phiên', session.id]];
  for (const [key, value] of pairs) { const group = el('div'); group.append(el('dt', '', key), el('dd', '', value)); grid.append(group); } content.append(grid);
  if (session.status.last_error) content.append(el('div', 'detail-error', session.status.last_error));
  if (!session.running && (session.auth === 'key' || session.password_saved)) { const actions = el('div', 'log-heading'), start = button('Chạy nền và bật tự khởi động', 'play'); start.onclick = () => controlSession(session, start, true); actions.append(start); content.append(actions); }
}
async function refreshLogs() { if (!state.detail || $('#details').hidden) return; const result = await api(`sessions/${state.detail}/logs`); const pre = $('#log-output'); const stick = pre.scrollHeight - pre.scrollTop - pre.clientHeight < 40; pre.textContent = result.output || 'Chưa có hoạt động được ghi lại.'; if (stick) pre.scrollTop = pre.scrollHeight; }
function promptText(title, label, type = 'text') {
  return new Promise(resolve => {
    $('#prompt-title').textContent = title; $('#prompt-label').textContent = label; const input = $('#prompt-input'); input.type = type; input.value = ''; input.autocomplete = 'off'; $('#prompt-modal').hidden = false; input.focus();
    const done = value => { $('#prompt-modal').hidden = true; input.value = ''; $('#prompt-form').onsubmit = null; $('#prompt-cancel').onclick = null; resolve(value); };
    $('#prompt-form').onsubmit = event => { event.preventDefault(); done(input.value); }; $('#prompt-cancel').onclick = () => done(null);
  });
}

function setPane(id, pane) {
  $('#' + id).dataset.activePane = pane;
  $$(`[data-pane-switch="${id}"] button`).forEach(button => button.setAttribute('aria-pressed', String(button.dataset.pane === pane)));
}
$$('[data-pane-switch] button').forEach(button => { button.onclick = () => setPane(button.parentElement.dataset.paneSwitch, button.dataset.pane); });
$('#new-session').onclick = openWizard; $('#close-wizard').onclick = closeWizard; $('#host-search').oninput = renderHosts;
$('#nav-new-session').onclick = openWizard;
$('#nav-sessions').onclick = () => showPage('sessions');
$('#nav-servers').onclick = () => showPage('servers');
$('#manage-servers').onclick = () => { closeWizard(); showPage('servers'); };
$('#server-search').oninput = filterServers;
$('#refresh-servers').onclick = async () => { try { await refreshInfo(); } catch (err) { toast(err.message, true); } };
$('#refresh-sessions').onclick = () => poll(); $('#wizard-next').onclick = nextStep;
$('#wizard-back').onclick = () => { if (state.busy) return; if (!state.step) closeWizard(); else { state.step--; renderStep(); } };
$$('input[name=auth]').forEach(input => input.onchange = () => { resetConnection(); updateAuthFields(); });
$('#ssh-password').oninput = resetConnection;
$('#use-saved-password').onchange = () => { resetConnection(); $('#ssh-password').value = ''; updateAuthFields(); };
$('#select-all').onchange = () => { if ($('#select-all').checked) { state.selected.clear(); state.selected.set('.', true); } else state.selected.delete('.'); renderSelections(); state.browsers.selection?.paintList(); };
$$('input[name="selection-mode"]').forEach(input => input.onchange = () => {
  state.selected.clear(); state.previewOK = false; $('#confirm-mirror').checked = false;
  renderSelections(); state.browsers.selection?.paintList();
});
$('#start-now').onchange = updateStartupHint; $('#save-password').onchange = updateStartupHint;
$('#refresh-preview').onclick = () => preview().catch(err => toast(err.message, true));
$('#close-details').onclick = () => showPage('sessions'); $('#refresh-logs').onclick = () => refreshLogs().catch(err => toast(err.message, true));
document.addEventListener('keydown', event => {
  if (event.key === 'Escape' && !$('#prompt-modal').hidden) $('#prompt-cancel').click();
  if (event.key === 'Tab') {
    const modal = $('#prompt-modal').hidden ? null : $('#prompt-modal');
    if (!modal) return;
    const focusable = $$('button:not(:disabled), input:not(:disabled), [tabindex="0"]', modal).filter(node => node.getClientRects().length);
    const first = focusable[0], last = focusable.at(-1);
    if (event.shiftKey && (document.activeElement === first || !modal.contains(document.activeElement))) { event.preventDefault(); last?.focus(); }
    else if (!event.shiftKey && (document.activeElement === last || !modal.contains(document.activeElement))) { event.preventDefault(); first?.focus(); }
  }
});
(async function boot() { try { const params = new URLSearchParams(location.search); await refreshInfo(); await poll(); if (params.get('page') === 'details' && state.sessions.some(s => s.id === params.get('session'))) openDetails(params.get('session')); else await showPage(['servers', 'new'].includes(params.get('page')) ? params.get('page') : 'sessions'); } catch (err) { renderLoadError(err.message); } setInterval(poll, 2000); })();
