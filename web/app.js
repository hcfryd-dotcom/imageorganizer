// ImageOrganizer UI. One Alpine component; view state lives in the URL hash so
// a reload (including an automatic one after a deploy) lands where you were.

const icons = {
  photos: '<svg viewBox="0 0 24 24"><rect x="3" y="3" width="18" height="18" rx="4"/><circle cx="9" cy="9" r="2"/><path d="M21 15l-5-5L5 21"/></svg>',
  map: '<svg viewBox="0 0 24 24"><path d="M9 4L3 6v14l6-2 6 2 6-2V4l-6 2-6-2z"/><path d="M9 4v14M15 6v14"/></svg>',
  table: '<svg viewBox="0 0 24 24"><rect x="3" y="4" width="18" height="16" rx="3"/><path d="M3 10h18M3 15h18M9 10v10"/></svg>',
  heart: '<svg viewBox="0 0 24 24"><path d="M12 20s-7-4.4-9.2-9A5 5 0 0112 5.6 5 5 0 0121.2 11C19 15.6 12 20 12 20z"/></svg>',
  heartFill: '<svg viewBox="0 0 24 24" class="fill"><path d="M12 20s-7-4.4-9.2-9A5 5 0 0112 5.6 5 5 0 0121.2 11C19 15.6 12 20 12 20z"/></svg>',
  folder: '<svg viewBox="0 0 24 24"><path d="M3 7a2 2 0 012-2h4l2 2h8a2 2 0 012 2v8a2 2 0 01-2 2H5a2 2 0 01-2-2z"/></svg>',
  search: '<svg viewBox="0 0 24 24"><circle cx="11" cy="11" r="7"/><path d="M20 20l-3.5-3.5"/></svg>',
  rotate: '<svg viewBox="0 0 24 24"><path d="M20 12a8 8 0 1 1-2.3-5.6"/><path d="M20 4v5h-5"/></svg>',
  rotateLeft: '<svg viewBox="0 0 24 24"><path d="M4 12a8 8 0 1 0 2.3-5.6"/><path d="M4 4v5h5"/></svg>',
  gear: '<svg viewBox="0 0 24 24"><circle cx="12" cy="12" r="3.2"/><path d="M12 2.8v2.4M12 18.8v2.4M2.8 12h2.4M18.8 12h2.4M5.5 5.5l1.7 1.7M16.8 16.8l1.7 1.7M5.5 18.5l1.7-1.7M16.8 7.2l1.7-1.7"/><circle cx="12" cy="12" r="6.6"/></svg>',
  edit: '<svg viewBox="0 0 24 24"><path d="M4 20h4L19 9l-4-4L4 16v4z"/><path d="M14 6l4 4"/></svg>',
  grid: '<svg viewBox="0 0 24 24"><rect x="4" y="4" width="7" height="7" rx="1.5"/><rect x="13" y="4" width="7" height="7" rx="1.5"/><rect x="4" y="13" width="7" height="7" rx="1.5"/><rect x="13" y="13" width="7" height="7" rx="1.5"/></svg>',
  close: '<svg viewBox="0 0 24 24"><path d="M6 6l12 12M18 6L6 18"/></svg>',
  left: '<svg viewBox="0 0 24 24"><path d="M15 5l-7 7 7 7"/></svg>',
  right: '<svg viewBox="0 0 24 24"><path d="M9 5l7 7-7 7"/></svg>',
  up: '<svg viewBox="0 0 24 24"><path d="M12 19V5M5 12l7-7 7 7"/></svg>',
  info: '<svg viewBox="0 0 24 24"><circle cx="12" cy="12" r="9"/><path d="M12 11v6M12 7.5v.5"/></svg>',
  trash: '<svg viewBox="0 0 24 24"><path d="M4 7h16M9 7V4.5h6V7M6.5 7l1 13h9l1-13M10 11v6M14 11v6"/></svg>',
  download: '<svg viewBox="0 0 24 24"><path d="M12 4v11M7 10l5 5 5-5M5 20h14"/></svg>',
  camera: '<svg viewBox="0 0 24 24"><path d="M4 8h3l2-3h6l2 3h3v11H4z"/><circle cx="12" cy="13" r="3.5"/></svg>',
  image: '<svg viewBox="0 0 24 24"><rect x="3" y="3" width="18" height="18" rx="3"/><path d="M21 15l-5-5L5 21"/></svg>',
  server: '<svg viewBox="0 0 24 24"><rect x="3" y="4" width="18" height="7" rx="2"/><rect x="3" y="13" width="18" height="7" rx="2"/><path d="M7 7.5h.01M7 16.5h.01"/></svg>',
  people: '<svg viewBox="0 0 24 24"><circle cx="9" cy="8" r="3.5"/><path d="M2.5 20a6.5 6.5 0 0113 0"/><circle cx="17" cy="9" r="2.5"/><path d="M15.5 14.2A5 5 0 0121.5 19"/></svg>',
  discover: '<svg viewBox="0 0 24 24"><circle cx="12" cy="12" r="9"/><path d="M15.5 8.5l-2 5-5 2 2-5z"/></svg>',
  tree: '<svg viewBox="0 0 24 24"><circle cx="12" cy="5" r="2.5"/><circle cx="6" cy="19" r="2.5"/><circle cx="18" cy="19" r="2.5"/><path d="M12 7.5V12M6 16.5V12h12v4.5"/></svg>',
  lanes: '<svg viewBox="0 0 24 24"><circle cx="5" cy="7" r="2.2"/><circle cx="5" cy="17" r="2.2"/><path d="M10 7h11M10 17h11"/></svg>',
  findFace: '<svg viewBox="0 0 24 24"><circle cx="10" cy="8" r="3.2"/><path d="M4 19a6 6 0 0 1 9-5.2"/><circle cx="17" cy="16" r="3"/><path d="M19.2 18.2L21 20"/></svg>',
  match: '<svg viewBox="0 0 24 24"><circle cx="12" cy="6.5" r="3"/><circle cx="12" cy="17.5" r="3"/><path d="M5 12h3M16 12h3"/></svg>',
  check: '<svg viewBox="0 0 24 24"><path d="M5 12.5l4.5 4.5L19 7"/></svg>',
  strip: '<svg viewBox="0 0 24 24"><rect x="2.5" y="7" width="5" height="10" rx="1.2"/><rect x="9.5" y="5" width="5" height="14" rx="1.2"/><rect x="16.5" y="7" width="5" height="10" rx="1.2"/></svg>',
  pin: '<svg viewBox="0 0 24 24"><path d="M12 21s-7-6.2-7-11a7 7 0 0114 0c0 4.8-7 11-7 11z"/><circle cx="12" cy="10" r="2.5"/></svg>',
};

const PAGE = 300;
const GAP = 4;
const MONTHS = ['January','February','March','April','May','June','July','August','September','October','November','December'];

document.addEventListener('alpine:init', () => {
  Alpine.data('app', () => ({
    icons,
    // navCount is the number shown next to a sidebar item (0: none).
    navCount(n) {
      const lib = (this.stats && this.stats.library) || {};
      switch (n.id) {
        case 'photos': return lib.images || 0;
        case 'favorites': return lib.favorites || 0;
        case 'people': return this.peopleData.people.length;
        case 'review': return (this.jobs && this.jobs.review) || 0;
        // The server's count, less the questions it still has that were
        // answered or skipped here (it counts them till it has learnt).
        case 'discover': return Math.max(0, ((this.stats && this.stats.discover) || 0) - this.discover.skipped - this.discover.since);
      }
      return 0;
    },
    nav: [
      {id: 'photos', label: 'Photos', icon: 'photos'},
      {id: 'favorites', label: 'Favorites', icon: 'heart', view: 'photos', fav: true},
      {id: 'discover', label: 'Discover', icon: 'discover', badge: true},
      {id: 'people', label: 'People', icon: 'people'},
      {id: 'review', label: 'Review', icon: 'check', badge: true},
      {id: 'family', label: 'Family', icon: 'tree'},
      {id: 'find', label: 'Find face', icon: 'findFace'},
      {id: 'map', label: 'Map', icon: 'map'},
      {id: 'table', label: 'Table', icon: 'table'},
      {id: 'folders', label: 'Folders', icon: 'folder'},
    ],
    ready: false,
    view: 'photos',
    q: '',
    sort: 'taken_desc',
    fav: false,
    rowHeight: 220,
    items: [],
    total: 0,
    base: 0,            // where items start in the whole list: a jump to a far year starts the grid there
    semantic: false,
    hadQ: false,        // the last search had text (see reload: relevance order)
    dateRange: '',
    searchText: '',
    searchPeople: [],      // people recognised in the search text
    searchSpecial: [],     // "known" / "unknown" face filters in effect
    tokens: [],            // people picked in the search box, shown as chips: {id, name}
    suggest: {items: [], i: 0, from: 0, to: 0},
    loading: false,
    reqSeq: 0,
    sections: [],
    years: [],
    currentYear: null,
    stats: {library: {images: 0, files: 0, geo: 0, bytes: 0, lost: 0}},
    scan: {running: false},
    jobs: null,
    person: 0,          // photos filtered to this person
    personName: '',
    peopleData: {people: [], groups: []},
    openPersonId: 0,    // person page in the People view
    personGroups: [],
    personFaces: [],   // the open person's faces, newest first
    personFacesShow: 240,
    // Person page, "Is this also <name>?" one face at a time: {pid, items,
    // answered, show, loading}
    faceCands: newFaceCands(),
    doubts: {pid: 0, items: [], show: 30}, // "Is this really …?" on a person's page
    kinForm: {kind: 'parent', name: ''}, // person page: a relative being added
    peopleFilter: '',   // People list: the quick filter
    // The Match tab: unknown faces, each under who it most likely is,
    // answered yes, no or skip. answered holds face ids (yes) and
    // "face:person" (no: the face may be someone else); a skipped face goes
    // to the back of the queue, on the server.
    matcher: {items: [], answered: new Set(), loading: false},
    matchFilter: {minAge: '', maxAge: '', sort: ''}, // Match tab: the unknown face's age, and the order
    // Lane Match (experiment): a row per person with the faces likeliest
    // theirs. gone: faces answered (fading out); auto: faces filed by
    // themselves per person, for an undo.
    lanes: {people: [], faces: {}, gone: {}, auto: {}, add: '', timer: null, autoOn: false}, // autoOn: file near-certain faces by themselves (opt-in)
    // Pictures on the wall: per person, faces of them at an age long
    // passed (and others on the same wall); picView: one being looked at.
    pictures: {},
    // Discover (experiment): people who matter, not named yet.
    // Discover's queue: the questions as served (the most worth first), the
    // faces unticked (off), the questions answered or skipped (done: key ->
    // what was asked, so a new batch for the same person still shows).
    // learn: what the server is learning in the background after an answer
    // (pushed as it goes): filing faces that are now near certain, then
    // listing the next questions.
    learn: {busy: false, phase: '', pass: 0, filed: 0, unfiled: 0},
    discover: {queue: [], total: 0, names: {}, off: {}, done: {}, skipped: 0, since: 0, whoSort: '', ageMin: 0, ageMax: 90, minMatch: 0, loading: false, busy: false, dating: '', recent: [], paint: {}, tab: 'all', kinds: {}},
    treeHover: 0,       // the person hovered in the family tree: everyone else shows what they are to them
    picView: null,      // {pid, set, tick: {face: true}, with: [{person, faces}], busy}
    // Find face: a picture from outside the library (pasted, dropped or
    // chosen), its faces, and the library's most alike for the one picked.
    finder: {file: null, url: '', busy: false, error: '', faces: [], pick: 0, sel: {}, names: {}, tagName: '', saving: false, drag: false},
    reviewItems: [],
    reviewGroup: 0,     // a group opened from a photo, shown first in Review
    reviewFocus: null,
    reviewTotal: 0,
    reviewFaces: 0,     // faces in the review groups
    reviewOrder: 'size',
    // Filing every Review group whose best suggestion is at least min% alike.
    bulk: {min: 75, preview: null, busy: false, loading: false, showPeople: false},
    peopleOrder: 'name',
    reviewLoading: false,
    names: {},          // group id -> typed name (drafts persist across reloads)
    busyGroup: {},      // group id -> request in flight
    mergeInto: '',
    progress: [], // recent {t, left} samples for the ETA
    faceProgress: [], // the same, for faces waiting for age and gender
    folders: [],
    add: {kind: 'file', path: '', host: '', user: '', password: '', busy: false},
    folderError: '',
    browser: {open: false, location: '', parent: '', dirs: [], loading: false, error: ''},
    lightbox: {open: false, idx: 0, info: true, faces: false, card: 0, strip: false, back: null}, // card: the Review card a photo was opened from
    // The photos around the open one (the strip under it): {id, by, items,
    // more: [earlier, later]} — more says whether the library goes on.
    around: {id: 0, by: '', items: [], more: [false, false]},
    assign: {face: null, group: null, name: '', busy: false},
    // Copying a person's photos out into a folder, one subfolder per year.
    exportForm: {open: false, dir: '', confirmedOnly: false, byPlace: '', busy: false, status: null},
    // Folders page: the whole library copied into one tidy folder.
    tidy: {dir: '', from: '', preview: null, busy: false, status: null},
    // Deleting the open photo: its files move to a deleted-photos folder.
    deleteAsk: {open: false, trash: '', busy: false},
    also: newAlso(),
    groupView: {g: null, faces: [], loading: false},  // one group opened from a person
    groupSel: {},       // person page: selected group ids
    moveTo: '',
    moving: false,
    rotating: false,
    settings: null,     // the Settings page state from the server
    form: {},           // the settings being edited
    testing: false,
    testResult: null,
    savingSettings: false,
    restarting: false,
    zoom: {s: 1, x: 0, y: 0, drag: null, moved: false}, // lightbox zoom and pan
    dateEdit: null,     // the photo date being corrected (datetime-local value)
    eventHide: null,    // "strangers at this event": {id, ev, busy, skip: {image: true}, answers: {face: 'yes'|'no'}}
    peek: null,         // face crop shown large on hover: {f, x, y}   // "Is this also <name>?" after naming a face
    detail: null,
    restoring: false,   // showing a history entry (Back/Forward): add none
    geoCount: 0,
    map: null,
    mapLayer: null,
    mapFocus: null,     // the photo "Show on map" came from: {id, lat, lon, hash}
    mapFocusMarker: null,
    mapReturn: null,    // where "Back to photo" goes: {view, id}
    // Select in Photos: the photos picked (id -> true), and the last one
    // clicked, for shift-click runs.
    sel: {on: false, ids: {}, last: null},
    // Setting the place of photos: the town search and the spot picked.
    pick: {open: false, ids: [], q: '', results: [], lat: null, lon: null, label: '', busy: false},
    // Map: photos without a place borrowing one from photos taken close in time.
    borrow: {open: false, hours: 3, count: null, busy: false, sure: false},
    toast: {show: false, text: '', timer: null, action: null},
    conn: {online: false, everConnected: false, dev: false, buildID: null, es: null, lastMsg: 0, retry: 500, timer: null},

    // ---------- lifecycle ----------
    init() {
      this.readHash();
      try { this.rowHeight = +localStorage.getItem('rowHeight') || this.rowHeight; } catch (e) {}
      try { this.lightbox.faces = localStorage.getItem('showFaces') === '1'; } catch (e) {}
      try { this.lightbox.strip = localStorage.getItem('showStrip') === '1'; } catch (e) {}
      try { this.reviewOrder = localStorage.getItem('reviewOrder') || 'size'; } catch (e) {}
      try { this.peopleOrder = localStorage.getItem('peopleOrder') || 'name'; } catch (e) {}
      try { this.bulk.min = +localStorage.getItem('bulkMin') || 75; } catch (e) {}
      try { Object.assign(this.matchFilter, JSON.parse(localStorage.getItem('matchFilter') || '{}')); } catch (e) {}
      this.$watch('rowHeight', v => { try { localStorage.setItem('rowHeight', v); } catch (e) {} });
      window.addEventListener('hashchange', () => this.onHashChange());
      new ResizeObserver(() => this.layout()).observe(this.$refs.photos);
      const io = new IntersectionObserver(es => { if (es.some(e => e.isIntersecting)) this.loadMore(); }, {rootMargin: '1200px'});
      io.observe(this.$refs.sentinel);
      io.observe(this.$refs.tableSentinel);
      this.connect();
      this.refreshStats();
      this.loadFolders();
      this.loadDrafts();
      this.loadPeople();
      this.loadSettings();
      this.reload().then(() => {
        this.ready = true;
        this.scrollBack(this.restoreScroll);
        if (this.pendingImage) this.openImageById(this.pendingImage);
      });
      this.enterView();
    },

    // ---------- URL state ----------
    readHash() {
      const p = new URLSearchParams(location.hash.slice(1));
      this.view = p.get('view') || 'photos';
      this.q = p.get('q') || '';
      this.tokens = (p.get('who') || '').split(',').filter(Boolean).map(t => {
        const i = t.indexOf(':'), id = t.slice(0, i);
        return /^\d+$/.test(id) ? {id: +id, name: t.slice(i + 1)} : {id, name: t.slice(i + 1), special: true};
      });
      this.sort = p.get('sort') || 'taken_desc';
      this.hadQ = !!this.q.trim();
      this.fav = p.get('fav') === '1';
      this.person = +p.get('person') || 0;
      this.openPersonId = +p.get('p') || 0;
      this.reviewGroup = +p.get('g') || 0;
      this.pendingImage = +p.get('img') || null;
      this.restoreScroll = readScroll('scroll:' + location.hash);
    },
    writeHash() {
      const p = new URLSearchParams();
      if (this.view !== 'photos') p.set('view', this.view);
      if (this.q) p.set('q', this.q);
      if (this.tokens.length) p.set('who', this.tokens.map(t => t.id + ':' + t.name).join(','));
      if (this.sort !== 'taken_desc') p.set('sort', this.sort);
      if (this.fav) p.set('fav', '1');
      if (this.person) p.set('person', this.person);
      if (this.view === 'people' && this.openPersonId) p.set('p', this.openPersonId);
      if (this.view === 'review' && this.reviewGroup) p.set('g', this.reviewGroup);
      if (this.lightbox.open && this.current()) p.set('img', this.current().id);
      const h = p.toString();
      if (h === location.hash.slice(1)) return;
      // A new page (view, person, group, search) or an opened photo gets a
      // history entry, so Back and Forward work; stepping between photos
      // and other small changes replace the current one.
      const cur = new URLSearchParams(location.hash.slice(1));
      const page = q => { const c = new URLSearchParams(q); c.delete('img'); return c.toString(); };
      const url = h ? '#' + h : location.pathname;
      const state = p.has('img') ? {photo: true} : null; // Back closes the photo
      if (!this.restoring && (page(cur) !== page(p) || (p.has('img') && !cur.has('img')))) history.pushState(state, '', url);
      else history.replaceState(state, '', url);
    },
    // topTile is the first photo (tile or table row) whose bottom is below
    // y on screen.
    topTile(el, y) {
      let list;
      if (this.view === 'table') {
        list = el.querySelectorAll('tr[data-id]');
      } else {
        const sec = [...el.querySelectorAll('.section')].find(s => s.getBoundingClientRect().bottom > y);
        list = sec ? sec.querySelectorAll('.tile[data-id]') : [];
      }
      let lo = 0, hi = list.length; // in screen order: binary search
      while (lo < hi) {
        const mid = (lo + hi) >> 1;
        if (list[mid].getBoundingClientRect().bottom > y) hi = mid; else lo = mid + 1;
      }
      return list[lo] || null;
    },
    // scrollBack returns the photo list to where it was (see onScroll):
    // it loads photos until the one that was at the top is back, then lines
    // it up again.
    async scrollBack(at) {
      if (!at || !at.y) return;
      const far = () => {
        if (at.id) return this.items.some(im => im.id === at.id);
        const el = this.scroller();
        return !el || el.scrollHeight - el.clientHeight >= at.y;
      };
      for (let i = 0; i < 20 && this.moreLater(); i++) {
        await Promise.resolve(); // after Alpine's redraw, queued before this
        if (far()) break;
        await this.loadMoreNow();
      }
      await Promise.resolve();
      const el = this.scroller();
      if (!el) return;
      el.scrollTop = at.y;
      // Rows near it are measured once on screen; line up on the photo,
      // twice, as lining up brings more rows on screen.
      const sel = this.view === 'table' ? 'tr' : '.tile';
      for (let i = 0; i < 2 && at.id; i++) {
        await new Promise(r => { requestAnimationFrame(r); setTimeout(r, 100); });
        const t = el.querySelector(`${sel}[data-id="${at.id}"]`);
        if (!t) break;
        el.scrollTop += t.getBoundingClientRect().top - el.getBoundingClientRect().top - at.off;
      }
    },
    // onHashChange shows what the address says, after Back, Forward or an
    // edited URL; nothing it does adds history.
    onHashChange() {
      this.restoring = true;
      try {
        const before = [this.view, this.q, this.tokenIDs(), this.sort, this.fav, this.person].join('|');
        const view = this.view, pid = this.openPersonId, gid = this.reviewGroup;
        this.readHash();
        const img = this.pendingImage;
        if (this.lightbox.open && (!img || this.current()?.id !== img)) this.closeLightbox();
        if ([this.view, this.q, this.tokenIDs(), this.sort, this.fav, this.person].join('|') !== before) {
          const st = this.restoreScroll;
          this.reload().then(() => this.scrollBack(st));
        }
        if (this.view !== view) this.enterView();
        else if (this.view === 'people' && this.openPersonId !== pid) this.openPersonId ? this.openPerson({id: this.openPersonId}) : this.loadPeople();
        else if (this.view === 'review' && this.reviewGroup !== gid) { this.reviewFocus = null; this.loadReview(true); }
        if (img && !this.lightbox.open) this.openImageById(img);
      } finally {
        this.restoring = false;
      }
    },

    // ---------- navigation ----------
    isNav(n) {
      const v = n.view || n.id;
      return this.view === v && !!n.fav === this.fav && (v !== 'photos' || !this.person);
    },
    goNav(n) {
      this.clearMapFocus();
      this.fav = !!n.fav;
      this.person = 0;
      if (n.id === 'people') this.openPersonId = 0;
      if (n.id === 'review') { this.reviewGroup = 0; this.reviewFocus = null; }
      this.setView(n.view || n.id);
      if ((n.view || n.id) === 'photos' || n.id === 'table') this.reload();
    },
    setView(v) {
      if (v !== 'photos') this.endSelect();
      if (v !== 'map') this.borrow.open = false;
      this.view = v;
      this.writeHash();
      this.enterView();
    },
    // enterView loads what the current view shows. It runs however the
    // view was reached: sidebar, URL, reload or live reload.
    enterView() {
      const v = this.view;
      if (v === 'map') this.$nextTick(() => this.showMap());
      if (v === 'folders') { this.loadFolders(); this.loadTidy(); }
      if (v === 'settings') this.loadSettings();
      if (v === 'people') this.loadPeople();
      if (v === 'review') { this.loadPeople(); this.loadReview(true); }
      if (v === 'match') this.openMatcher();
      if (v === 'lanes') this.openLanes();
      if (v === 'discover') this.loadDiscover();
      if (v === 'family') this.loadPeople();
      if (v === 'photos') this.$nextTick(() => this.layout());
    },
    isPhotoView() { return ['photos', 'table', 'map'].includes(this.view); },
    scroller() { return this.view === 'table' ? this.$refs.table : this.$refs.photos; },

    // ---------- data ----------
    query(extra = {}) {
      const p = new URLSearchParams({sort: this.sort, ...extra});
      if (this.q) p.set('q', this.q);
      if (this.tokens.length) p.set('people', this.tokenIDs());
      if (this.fav) p.set('fav', '1');
      if (this.person) p.set('person', this.person);
      return p.toString();
    },
    async reload() {
      // A new text search starts best match first (unless another order
      // was picked); without text there's nothing to be relevant to.
      const has = !!this.q.trim();
      if (has && !this.hadQ && this.sort === 'taken_desc') this.sort = 'relevance';
      if (!has && this.sort === 'relevance') this.sort = 'taken_desc';
      this.hadQ = has;
      this.writeHash();
      const seq = ++this.reqSeq;
      this.loading = true;
      try {
        const [res, tl] = await Promise.all([
          api('/api/images?' + this.query({offset: 0, limit: PAGE})),
          api('/api/timeline?' + this.query()),
        ]);
        if (seq !== this.reqSeq) return;
        this.base = 0;
        this.items = res.items;
        this.total = res.total;
        this.semantic = !!res.semantic;
        this.dateRange = res.range || '';
        this.searchText = res.text || '';
        this.searchPeople = res.people || [];
        this.searchSpecial = res.special || [];
        this.years = yearsFrom(tl);
        this.layout();
        if (this.view === 'map' && this.map) this.loadGeo();
      } finally {
        if (seq === this.reqSeq) this.loading = false;
      }
    },
    // softRefresh re-fetches what is loaded without losing scroll position.
    async softRefresh() {
      if (this.loading) return;
      const seq = ++this.reqSeq;
      const n = Math.max(this.items.length, PAGE);
      const res = await api('/api/images?' + this.query({offset: this.base, limit: Math.min(n, 2000)}));
      if (seq !== this.reqSeq) return;
      this.items = res.items;
      this.total = res.total;
      this.layout();
    },
    async loadMore() {
      if (this.loading || !this.moreLater() || !this.ready) return;
      if (this.view !== 'photos' && this.view !== 'table') return;
      const seq = this.reqSeq;
      this.loading = true;
      try {
        const res = await api('/api/images?' + this.query({offset: this.base + this.items.length, limit: PAGE}));
        if (seq !== this.reqSeq) return;
        this.items = this.items.concat(res.items);
        this.total = res.total;
        this.layout();
      } finally {
        this.loading = false;
        setTimeout(() => this.checkEarlier());
      }
    },
    // peopleChanged re-reads the counts (the Review badge) after any
    // decision about people; auto-filing can change them too.
    peopleChanged() {
      clearTimeout(this._statsTimer);
      clearTimeout(this._statsTimer2);
      this._statsTimer = setTimeout(() => this.refreshStats().catch(() => {}), 300);
      // The Discover badge is counted again in the background: once more
      // when that's done.
      this._statsTimer2 = setTimeout(() => this.refreshStats().catch(() => {}), 2500);
    },
    async refreshStats() {
      const s = await api('/api/stats');
      this.stats = s;
      this.scan = s.scan;
      const j = s.jobs || null;
      const wasBusy = this.busy();
      this.jobs = j;
      // Self-heal: an inbox with a count but no cards reloads its cards.
      if (this.view === 'review' && j && j.review > 0 && !this.reviewItems.length && !this.reviewLoading) this.loadReview(true);
      // Keep two minutes of "photos left" samples to estimate the rate.
      const now = Date.now(), left = this.remaining();
      this.progress = this.progress.filter(p => now - p.t < 120000 && p.left >= left);
      this.progress.push({t: now, left});
      const fl = this.facesLeft();
      this.faceProgress = this.faceProgress.filter(p => now - p.t < 120000 && p.left >= fl);
      this.faceProgress.push({t: now, left: fl});
      if (wasBusy && !this.busy()) {
        this.loadFolders();
        this.softRefresh();
      }
    },
    busy() { const j = this.jobs; return !!(this.scan.running || (j && (j.pending > 0 || j.embedded < j.embeddable || j.look_done < j.look_total))); },
    // One number for all per-photo processing: files waiting to be read
    // plus photos whose search embedding is still missing (backfill).
    remaining() {
      const j = this.jobs;
      if (!j) return 0;
      return (j.pending || 0) + Math.max(0, (j.embeddable || 0) - (j.embedded || 0)) + Math.max(0, (j.look_total || 0) - (j.look_done || 0));
    },
    processedPct() {
      const done = this.stats.library.images || 0, left = this.remaining();
      return done + left ? Math.min(100, 100 * done / (done + left)) : 0;
    },
    processingActive() { const j = this.jobs; return !!(j && ((j.import && j.import.running) || (j.embed && j.embed.running) || (j.look && j.look.running))); },
    processingNote() {
      const j = this.jobs;
      if (!j) return '';
      if (j.import && j.import.paused) return 'Paused';
      if (j.import && !j.import.online) return 'Source unreachable — will resume';
      if (j.pending === 0 && j.embed && !j.embed.online) return 'Search server offline — will resume';
      return '';
    },
    // ETA from the rate over the samples kept in refreshStats.
    // Faces waiting for the age stage (age, and gender with the gender
    // model): new photos' faces, or all faces once after the model arrived.
    facesLeft() { const j = this.jobs; return j && j.ageable ? Math.max(0, j.ageable - (j.aged || 0)) : 0; },
    facesPct() { const j = this.jobs; return j && j.ageable ? Math.min(100, 100 * (j.aged || 0) / j.ageable) : 0; },
    facesActive() { const j = this.jobs; return !!(j && j.age && j.age.running); },
    facesNote() { const j = this.jobs; return j && j.age && j.age.paused ? 'Paused' : 'Age and gender, from the face crops'; },
    etaLabel() { return this.eta(this.progress, this.processingActive()); },
    faceEtaLabel() { return this.eta(this.faceProgress, this.facesActive()); },
    // eta estimates the time left from recent {t, left} samples.
    eta(p, active) {
      if (p.length < 3) return 'estimating…';
      const a = p[0], b = p[p.length - 1];
      const rate = (a.left - b.left) / ((b.t - a.t) / 1000); // per second
      if (!(rate > 0)) return active ? 'estimating…' : '';
      const s = b.left / rate;
      if (s < 90) return 'about a minute';
      if (s < 3600) return `about ${Math.round(s / 60)} min`;
      const h = Math.floor(s / 3600), m = Math.round((s % 3600) / 60);
      return m ? `about ${h} h ${m} min` : `about ${h} h`;
    },

    // ---------- justified layout ----------
    layout() {
      const el = this.$refs.photos;
      if (!el || this.view !== 'photos') return;
      const cs = getComputedStyle(el);
      const width = el.clientWidth - parseFloat(cs.paddingLeft) - parseFloat(cs.paddingRight);
      if (width <= 0) return;
      const byMonth = this.sort.startsWith('taken');
      const target = this.rowHeight;
      const sections = [];
      let sec = null;
      this.items.forEach((im, idx) => {
        const key = byMonth ? monthKey(im.taken) : 'all';
        if (!sec || sec.key !== key) {
          sec = {key, title: byMonth ? monthTitle(im.taken) : '', list: []};
          sections.push(sec);
        }
        sec.list.push({im, idx, ar: aspect(im)});
      });
      const done = !this.moreLater();
      this.sections = sections.map((s, si) => {
        const rows = [];
        let row = [], sum = 0;
        const flush = (last) => {
          const gaps = GAP * (row.length - 1);
          let h = (width - gaps) / sum;
          if (last && h > target * 1.3) h = target; // don't blow up a short final row
          rows.push({key: s.key + ':' + rows.length, h: Math.round(h), items: row.map(it => ({...it, w: Math.floor(it.ar * h)}))});
          row = []; sum = 0;
        };
        for (const it of s.list) {
          row.push(it);
          sum += it.ar;
          if (sum * target + GAP * (row.length - 1) >= width) flush(false);
        }
        if (row.length) flush(si < sections.length - 1 || done);
        return {key: s.key, title: s.title, rows};
      });
    },
    onScroll() {
      const el = this.scroller();
      // Where the list is: the first photo at the top and how far it sits
      // from it, so Back finds it however the rows are measured then.
      const top = el.getBoundingClientRect().top;
      const tile = this.topTile(el, top + 60); // below the sticky month heading
      const at = {y: el.scrollTop, id: tile ? +tile.dataset.id : 0, off: tile ? tile.getBoundingClientRect().top - top : 0};
      try { sessionStorage.setItem('scroll:' + location.hash, JSON.stringify(at)); } catch (e) {}
      if (this.view !== 'photos') return;
      // Highlight the year currently at the top of the viewport. Months
      // are measured by their whole section: a heading in a month not drawn
      // yet (content-visibility) has no position of its own.
      const elTop = el.getBoundingClientRect().top;
      let y = null;
      for (const s of el.querySelectorAll('.section')) {
        if (s.getBoundingClientRect().top - elTop > 120) break;
        y = +s.querySelector('.section-head span').textContent.slice(-4) || y;
      }
      this.currentYear = y || (this.years[0] && this.years[0].year);
      this.checkEarlier();
    },
    // checkEarlier loads the photos before the grid's start when the top is
    // near; also after other loading, which may have held it up.
    checkEarlier() {
      const el = this.$refs.photos;
      if (this.view === 'photos' && this.base > 0 && el && el.scrollTop < 600) this.loadEarlier();
    },
    moreLater() { return this.base + this.items.length < this.total; },
    // jumpToYear shows a year's photos. One not loaded yet starts the grid
    // at the year (its place in the list, from the timeline's counts):
    // loading and drawing every photo before it took a minute for a year
    // far back. Newer photos come back on scrolling up (loadEarlier).
    async jumpToYear(year) {
      const el = this.$refs.photos;
      const yearOf = im => im.taken && new Date(im.taken * 1000).getFullYear();
      if (!this.items.some(im => yearOf(im) === +year)) {
        const asc = this.sort === 'taken_asc';
        let off = 0;
        for (const y of this.years) if (asc ? y.year < year : y.year > year) off += y.n;
        const seq = ++this.reqSeq;
        this.loading = true;
        try {
          const res = await api('/api/images?' + this.query({offset: off, limit: PAGE}));
          if (seq !== this.reqSeq) return;
          this.base = off;
          this.items = res.items;
          this.total = res.total;
          this.sel.last = null;
          this.layout();
        } finally {
          if (seq === this.reqSeq) this.loading = false;
        }
        await this.$nextTick();
        el.scrollTop = 0;
      }
      this.$nextTick(() => {
        const sec = [...el.querySelectorAll('.section')].find(s => s.querySelector('.section-head span').textContent.endsWith(String(year)));
        if (!sec) return;
        // Measured by the month's section (see onScroll), and once more after
        // the months around it are drawn and have their real heights.
        const go = () => el.scrollTo({top: sec.getBoundingClientRect().top - el.getBoundingClientRect().top + el.scrollTop});
        go();
        setTimeout(go, 150);
      });
    },
    // loadEarlier puts back the page of photos before the grid's start
    // (after a jump to a year), keeping what is on screen in place.
    async loadEarlier() {
      if (this.loading || this.base <= 0 || this.lightbox.open) return;
      const seq = this.reqSeq;
      this.loading = true;
      try {
        const from = Math.max(0, this.base - PAGE);
        const res = await api('/api/images?' + this.query({offset: from, limit: this.base - from}));
        if (seq !== this.reqSeq) return;
        const el = this.$refs.photos, h0 = el.scrollHeight, y0 = el.scrollTop;
        this.base = from;
        this.items = res.items.concat(this.items);
        this.total = res.total;
        if (this.sel.last != null) this.sel.last += res.items.length;
        this.layout();
        await this.$nextTick();
        el.scrollTop = y0 + el.scrollHeight - h0;
        setTimeout(() => this.checkEarlier()); // still near the top: the page before
      } finally {
        this.loading = false;
      }
    },    async loadMoreNow() {
      const seq = this.reqSeq, at = this.base + this.items.length;
      const res = await api('/api/images?' + this.query({offset: at, limit: 1000}));
      // A new search or a jump to a year meanwhile: these don't follow on.
      if (seq !== this.reqSeq || at !== this.base + this.items.length) return;
      this.items = this.items.concat(res.items);
      this.total = res.total;
      this.layout();
    },

    // ---------- table ----------
    setSort(s) {
      if (s === 'taken') s = this.sort === 'taken_desc' ? 'taken_asc' : 'taken_desc';
      this.sort = s;
      this.reload();
    },
    sortClass(k) { return this.sort.startsWith(k) ? 'sorted' : ''; },

    // ---------- map ----------
    showMap() {
      if (typeof L === 'undefined') return;
      if (!this.map) {
        this.map = L.map(this.$refs.map, {zoomControl: false, worldCopyJump: true}).setView([30, 10], 2);
        L.control.zoom({position: 'bottomright'}).addTo(this.map);
        // The map is created while its view may still be hidden; re-measure
        // whenever the container changes size.
        if (!this._mapResize) {
          this._mapResize = new ResizeObserver(() => this.map && this.map.invalidateSize({pan: false}));
          this._mapResize.observe(this.$refs.map);
        }
        // OpenStreetMap's own tiles need no key (CARTO's now do); dark mode
        // darkens them with a CSS filter.
        this.tiles().addTo(this.map);
        this.loadGeo(true);
      } else {
        this.map.invalidateSize();
      }
    },
    async loadGeo(fit) {
      const pts = await api('/api/geo?' + this.query());
      this.geoCount = pts.length;
      if (this.mapLayer) this.map.removeLayer(this.mapLayer);
      const cluster = L.markerClusterGroup({
        showCoverageOnHover: false, maxClusterRadius: 70, spiderfyOnMaxZoom: true,
        iconCreateFunction: c => {
          const first = c.getAllChildMarkers()[0];
          return L.divIcon({
            html: `<div class="pin cluster" style="background-image:url(/thumb/${first.options.hash})"><span>${fmtCount(c.getChildCount())}</span></div>`,
            className: '', iconSize: [56, 56],
          });
        },
      });
      for (const p of pts) {
        const hash = p.h;
        const m = L.marker([p.a, p.o], {
          hash,
          icon: L.divIcon({html: `<div class="pin" style="background-image:url(/thumb/${hash})"></div>`, className: '', iconSize: [44, 44]}),
        });
        m.on('click', () => this.openImageById(p.i));
        cluster.addLayer(m);
      }
      this.mapLayer = cluster;
      this.map.addLayer(cluster);
      if (this.mapFocus) this.focusMap();
      else if (fit && pts.length) this.map.fitBounds(cluster.getBounds(), {padding: [40, 40], maxZoom: 12});
    },
    // focusMap centres the map on the photo it was opened from, marked
    // above the others whether or not the current filters include it.
    focusMap() {
      const f = this.mapFocus;
      if (!this.map || !f) return;
      if (this.mapFocusMarker) this.map.removeLayer(this.mapFocusMarker);
      this.mapFocusMarker = L.marker([f.lat, f.lon], {
        zIndexOffset: 1000,
        icon: L.divIcon({html: `<div class="pin focus" style="background-image:url(/thumb/${f.hash})"></div>`, className: '', iconSize: [60, 60]}),
      }).on('click', () => this.backFromMap()).addTo(this.map);
      this.map.invalidateSize({pan: false});
      this.map.setView([f.lat, f.lon], 15, {animate: false});
    },
    clearMapFocus() {
      if (this.mapFocusMarker && this.map) this.map.removeLayer(this.mapFocusMarker);
      this.mapFocus = this.mapFocusMarker = this.mapReturn = null;
    },
    backFromMap() {
      const r = this.mapReturn;
      this.clearMapFocus();
      if (!r) return;
      this.setView(r.view);
      this.openImageById(r.id);
    },
    showOnMap(im) {
      this.mapReturn = {view: this.view, id: im.id};
      this.mapFocus = {id: im.id, lat: im.lat, lon: im.lon, hash: im.hash};
      this.closeLightbox(true);
      this.setView('map');
      // A new map focuses once its points load; an existing one now.
      if (this.map) this.$nextTick(() => this.focusMap());
    },
    tiles() {
      const url = (this.settings && this.settings.saved.map_tiles) || 'https://tile.openstreetmap.org/{z}/{x}/{y}.png';
      return L.tileLayer(url, {
        maxZoom: 19,
        attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors',
      });
    },

    // ---------- places: photos the camera gave none ----------
    startSelect() { this.sel = {on: true, ids: {}, last: null}; },
    endSelect() { if (this.sel.on) this.sel = {on: false, ids: {}, last: null}; },
    selIDs() { return Object.keys(this.sel.ids).map(Number); },
    selCount() { return Object.keys(this.sel.ids).length; },
    selItems() { return this.items.filter(im => this.sel.ids[im.id]); },
    setSel(id, on) { if (on) this.sel.ids[id] = true; else delete this.sel.ids[id]; },
    // toggleSel picks or unpicks a photo; with Shift, every photo from the
    // last one clicked to this one.
    toggleSel(it, e) {
      const on = !this.sel.ids[it.im.id];
      if (e.shiftKey && this.sel.last != null) {
        for (let i = Math.min(this.sel.last, it.idx); i <= Math.max(this.sel.last, it.idx); i++) {
          if (this.items[i]) this.setSel(this.items[i].id, on);
        }
      } else {
        this.setSel(it.im.id, on);
      }
      this.sel.last = it.idx;
    },
    secIDs(sec) { return sec.rows.flatMap(r => r.items.map(it => it.im.id)); },
    sectionSelected(sec) { const ids = this.secIDs(sec); return ids.length > 0 && ids.every(id => this.sel.ids[id]); },
    async selectSection(sec) {
      const on = !this.sectionSelected(sec);
      // The last month shown may go on past what is loaded: load it all.
      while (on && this.moreLater() && monthKey(this.items[this.items.length - 1].taken) === sec.key) {
        await this.loadMoreNow();
      }
      const cur = this.sections.find(s => s.key === sec.key) || sec;
      for (const id of this.secIDs(cur)) this.setSel(id, on);
    },
    // placeChanged updates the photos shown after their place was set (lat)
    // or taken away (null); a place from the camera stays.
    placeChanged(ids, lat, lon) {
      const want = new Set(ids);
      const upd = im => {
        if (!im || !want.has(im.id) || (im.lat != null && !im.geo_src)) return;
        if (lat == null) { delete im.lat; delete im.lon; delete im.geo_src; } else { im.lat = lat; im.lon = lon; im.geo_src = 'user'; }
      };
      this.items.forEach(upd);
      (this.lightbox.list || []).forEach(upd);
      if (this.detail) {
        upd(this.detail);
        this.detail = {...this.detail};
      }
    },
    async openPlacePick(ids) {
      this.pick = {open: true, ids, q: '', results: [], lat: null, lon: null, label: '', busy: false};
      await this.$nextTick();
      if (typeof L === 'undefined') return;
      const m = L.map(this.$refs.pickMap, {worldCopyJump: true});
      this._pickMap = m;
      this.tiles().addTo(m);
      m.on('click', e => this.setPick(e.latlng.lat, e.latlng.lng, ''));
      // Start at the photo's own place, else where a place was set last,
      // else over the photos that have one.
      const d = this.detail;
      let last = null;
      try { last = JSON.parse(localStorage.getItem('lastPlace')); } catch (e) {}
      if (ids.length === 1 && d && d.id === ids[0] && d.lat != null) {
        this.setPick(d.lat, d.lon, '');
        m.setView([d.lat, d.lon], 13);
      } else if (last && last.lat != null) {
        m.setView([last.lat, last.lon], last.z || 11);
      } else {
        const pts = await api('/api/geo').catch(() => []);
        if (pts.length) m.fitBounds(pts.map(p => [p.a, p.o]), {padding: [20, 20], maxZoom: 10});
        else m.setView([30, 10], 2);
      }
      setTimeout(() => { m.invalidateSize(); this.$refs.pickSearch && this.$refs.pickSearch.focus(); }, 60);
    },
    closePlacePick() {
      this.pick.open = false;
      if (this._pickMap) this._pickMap.remove();
      this._pickMap = this._pickMarker = null;
    },
    setPick(lat, lon, label) {
      this.pick.lat = lat;
      this.pick.lon = lon;
      this.pick.label = label;
      const m = this._pickMap;
      if (!m) return;
      if (!this._pickMarker) {
        this._pickMarker = L.marker([lat, lon], {draggable: true, icon: L.divIcon({html: '<div class="pick-pin"></div>', className: '', iconSize: [22, 22]})}).addTo(m);
        this._pickMarker.on('dragend', e => { const p = e.target.getLatLng(); this.setPick(p.lat, p.lng, ''); });
      } else {
        this._pickMarker.setLatLng([lat, lon]);
      }
    },
    async searchPlaces() {
      const q = this.pick.q.trim();
      if (q.length < 2) { this.pick.results = []; return; }
      const res = await api('/api/places?q=' + encodeURIComponent(q)).catch(() => []);
      if (this.pick.q.trim() === q) this.pick.results = res;
    },
    pickResult(p) {
      this.setPick(p.lat, p.lon, [p.name, p.area || p.region, p.country].filter(Boolean).join(', '));
      if (this._pickMap) this._pickMap.setView([p.lat, p.lon], 12);
      this.pick.q = p.name;
      this.pick.results = [];
    },
    async savePlace() {
      const {ids, lat, lon} = this.pick;
      this.pick.busy = true;
      try {
        const r = await api('/api/geo/set', {method: 'POST', body: JSON.stringify({ids, lat, lon})});
        try { localStorage.setItem('lastPlace', JSON.stringify({lat, lon, z: this._pickMap ? this._pickMap.getZoom() : 11})); } catch (e) {}
        this.closePlacePick();
        this.placeChanged(ids, lat, lon);
        this.notify((r.set === 1 ? 'Place set for 1 photo' : `Place set for ${fmtCount(r.set)} photos`) +
          (r.skipped ? ` · ${fmtCount(r.skipped)} kept the camera’s` : ''), 5000);
        this.refreshStats().catch(() => {});
        if (this.map) this.loadGeo();
      } catch (e) {
        this.notify(`Couldn’t save: ${e.message}`, 6000);
      } finally {
        this.pick.busy = false;
      }
    },
    async clearPlaces(ids) {
      try {
        const r = await api('/api/geo/clear', {method: 'POST', body: JSON.stringify({ids})});
        this.placeChanged(ids, null, null);
        this.notify(r.cleared === 1 ? 'Place removed from 1 photo' : `Place removed from ${fmtCount(r.cleared)} photos`, 4000);
        this.refreshStats().catch(() => {});
        if (this.map) this.loadGeo();
      } catch (e) {
        this.notify(`Couldn’t save: ${e.message}`, 6000);
      }
    },
    openBorrow() {
      this.borrow.open = !this.borrow.open;
      this.borrow.sure = false;
      if (this.borrow.open) this.countBorrow();
    },
    async countBorrow() {
      this.borrow.count = null;
      const r = await api('/api/geo/borrow', {method: 'POST', body: JSON.stringify({hours: this.borrow.hours, apply: false})}).catch(() => ({count: 0}));
      this.borrow.count = r.count;
    },
    async applyBorrow() {
      this.borrow.busy = true;
      try {
        const r = await api('/api/geo/borrow', {method: 'POST', body: JSON.stringify({hours: this.borrow.hours, apply: true})});
        this.notify(r.count === 1 ? '1 photo got a place' : `${fmtCount(r.count)} photos got a place`, 5000);
        await this.refreshStats().catch(() => {});
        this.loadGeo();
        this.softRefresh();
        this.countBorrow();
      } catch (e) {
        this.notify(`Couldn’t save: ${e.message}`, 6000);
      } finally {
        this.borrow.busy = false;
      }
    },
    async clearBorrowed() {
      this.borrow.sure = false;
      try {
        const r = await api('/api/geo/clear', {method: 'POST', body: JSON.stringify({all: 'near'})});
        this.notify(r.cleared === 1 ? 'Place removed from 1 photo' : `Place removed from ${fmtCount(r.cleared)} photos`, 5000);
        await this.refreshStats().catch(() => {});
        this.loadGeo();
        this.softRefresh();
        this.countBorrow();
      } catch (e) {
        this.notify(`Couldn’t save: ${e.message}`, 6000);
      }
    },

    // ---------- lightbox ----------
    current() { return this.lightbox.open ? this.lightboxItems()[this.lightbox.idx] : null; },
    lightboxItems() { return this.lightbox.list || this.items; },
    openLightbox(idx) {
      this.lightbox.list = null;
      this.lightbox.back = null;
      this.lightbox.idx = idx;
      this.lightbox.open = true;
      return this.loadDetail();
    },
    // openImageById opens a photo; from a face (faceId), with the faces
    // shown and that one's popup open, and from a Review card (card), the
    // card is refreshed when the photo closes.
    async openImageById(id, faceId, card) {
      this.lightbox.card = card || 0;
      let idx = this.items.findIndex(im => im.id === id);
      if (idx >= 0) {
        await this.openLightbox(idx);
      } else {
        const d = await api('/api/images/' + id).catch(() => null);
        if (!d) return;
        this.lightbox.list = [d];
        this.lightbox.back = null;
        this.lightbox.idx = 0;
        this.lightbox.open = true;
        this.writeHash();
        await this.loadDetail(); // also loads the faces
      }
      if (!faceId) return;
      this.lightbox.faces = true;
      const f = ((this.detail && this.detail.faces) || []).find(f => f.id === faceId);
      if (f) this.openAssign(f);
    },
    // refreshCard updates a Review card after its faces were confirmed or
    // taken out one by one in the photo viewer.
    async refreshCard(gid) {
      const card = this.reviewItems.find(x => x.id === gid);
      if (!card) return;
      const g = await api(`/api/groups/${gid}`).catch(() => null);
      if (g && g.size > 0 && (!g.person || g.auto)) {
        Object.assign(card, {faces: g.faces.slice(0, Math.max(card.faces.length, 1)), size: g.size, person: g.person || 0, person_name: g.person_name || '', auto: !!g.auto, suggestions: g.suggestions || []});
        return;
      }
      // Gone, or named for good: it leaves the inbox.
      this.reviewItems = this.reviewItems.filter(x => x.id !== gid);
      this.reviewTotal = Math.max(0, this.reviewTotal - 1);
    },
    // alsoAnswer says whether a suggested group is the person just named.
    // The row goes at once and the answer is saved in the background, in
    // click order, so a list can be answered as fast as it is read; once
    // all answers are saved the next best matches are fetched (a yes
    // changes them).
    alsoAnswer(g, yes) {
      const pid = this.also.person.id, name = this.alsoName();
      this.also.answered.add(g.id);
      if (yes) touched.all = true; // the group's other photos changed too
      this.also.items = this.also.items.filter(x => x.id !== g.id);
      this.peek = null;
      if (yes && this.reviewItems.some(x => x.id === g.id)) {
        this.reviewItems = this.reviewItems.filter(x => x.id !== g.id);
        this.reviewTotal = Math.max(0, this.reviewTotal - 1);
        this.reviewFaces = Math.max(0, this.reviewFaces - (g.size || 0));
      }
      alsoPending++;
      alsoQueue = alsoQueue.then(async () => {
        try {
          await api(`/api/groups/${g.id}/${yes ? 'name' : 'reject'}`, {method: 'POST', body: JSON.stringify({person_id: pid, shown: yes ? shownIDs(g, 4) : []})}, {retry: true});
        } catch (e) {
          this.notify(`Couldn’t save the answer for ${name}: ${e.message}`, 6000);
        }
      });
      alsoQueue.then(() => this.alsoSaved(pid));
    },
    // alsoSaved runs after each saved answer; after the last one it
    // refreshes the suggestions and everything a yes changes.
    async alsoSaved(pid) {
      if (--alsoPending > 0) return;
      this.refreshFaces();
      this.loadPeople();
      this.peopleChanged();
      if (this.also.person?.id !== pid) return;
      const next = await api(`/api/people/${pid}/candidates?limit=12`).catch(() => null);
      if (this.also.person?.id !== pid || alsoPending > 0) return; // closed, or answered more meanwhile
      if (next) this.also.items = next.filter(x => !this.also.answered.has(x.id));
      if (!this.also.items.length) { this.notify(`No more likely matches for ${this.alsoName()}`); this.closeAlso(); }
    },
    // peekFace shows a hovered face crop large, above it (or below when
    // there's no room), kept inside the window.
    peekFace(e, f) {
      const r = e.target.getBoundingClientRect(), size = 236;
      const x = Math.min(Math.max(r.left + r.width / 2 - size / 2, 8), innerWidth - size - 8);
      const y = r.top - size - 30 > 8 ? r.top - size - 30 : r.bottom + 8;
      this.peek = {f, x, y};
    },
    closeAlso() { this.also = newAlso(); this.peek = null; },
    // ---------- face matcher ----------
    openMatcher() {
      this.matcher = {items: [], answered: new Set(), loading: false};
      this.loadMatches();
    },
    // setMatchFilter starts the list over for a new age range or order
    // (remembered in this browser).
    setMatchFilter() {
      try { localStorage.setItem('matchFilter', JSON.stringify(this.matchFilter)); } catch (e) {}
      this.matcher.items = [];
      this.loadMatches();
    },
    // loadMatches fetches the likeliest matches. The first cards keep their
    // place (and their suggestion), so ones being looked at don't move.
    async loadMatches() {
      const mt = this.matcher;
      if (this.view !== 'match') return;
      mt.loading = true;
      const mf = this.matchFilter, q = new URLSearchParams({limit: 40});
      if (+mf.minAge > 0) q.set('min_age', +mf.minAge);
      if (+mf.maxAge > 0) q.set('max_age', +mf.maxAge);
      if (mf.sort) q.set('sort', mf.sort);
      const res = await api('/api/matches?' + q).catch(() => null);
      mt.loading = false;
      if (!res || this.view !== 'match' || mt !== this.matcher) return;
      const fresh = res.filter(m => !mt.answered.has(m.face.id) && !mt.answered.has(m.face.id + ':' + m.person));
      const ids = new Set(fresh.map(m => m.face.id));
      const keep = mt.items.slice(0, 5).filter(m => ids.has(m.face.id));
      const kept = new Set(keep.map(m => m.face.id));
      mt.items = [...keep, ...fresh.filter(m => !kept.has(m.face.id))];
    },
    // matchAnswer: 'yes' (to pid, the suggestion or one of the others),
    // 'no' or 'skip' (decide later: to the back of the queue). The card
    // goes at once; answers are saved in click order, then the list refills.
    // matchAnswer: 'yes' (it's the person shown), 'no' (it isn't: the
    // face may come back as someone else) or 'skip' (decide later: to the
    // back of the queue). The card goes at once; answers are saved in
    // click order, then the list refills.
    matchAnswer(m, ans, pid = m.person) {
      const mt = this.matcher;
      this.slideCards(() => { mt.items = mt.items.filter(x => x !== m); });
      this.peek = null;
      const calls = [];
      if (ans === 'yes') {
        mt.answered.add(m.face.id);
        calls.push([`/api/faces/${m.face.id}/tag`, pid]);
      } else if (ans === 'no') {
        mt.answered.add(m.face.id + ':' + pid);
        calls.push([`/api/faces/${m.face.id}/reject`, pid]);
      } else {
        calls.push([`/api/faces/${m.face.id}/skip`, 0]);
      }
      if (ans !== 'skip') touched.ids.add(m.face.image);
      alsoPending++;
      alsoQueue = alsoQueue.then(async () => {
        try {
          for (const [url, person] of calls) await api(url, {method: 'POST', body: JSON.stringify({person_id: person})}, {retry: true});
        } catch (e) {
          this.notify(`Couldn’t save the answer: ${e.message}`, 6000);
        }
      });
      alsoQueue.then(async () => {
        if (--alsoPending > 0) return;
        this.peopleChanged();
        this.loadPeople();
        this.loadMatches();
      });
    },
    // slideCards runs change (which removes cards) and slides the cards
    // after the gap left into place, rather than letting them jump.
    slideCards(change) {
      const row = this.$refs.matchRow;
      const before = new Map([...(row ? row.children : [])].map(el => [el, el.getBoundingClientRect().left]));
      change();
      this.$nextTick(() => {
        for (const [el, x] of before) {
          if (!el.isConnected) continue;
          const dx = x - el.getBoundingClientRect().left;
          if (!dx) continue;
          el.animate([{transform: `translateX(${dx}px)`}, {transform: 'none'}], {duration: 220, easing: 'ease-out'});
        }
      });
    },
    // ---------- strangers at an event ----------
    // openEvent shows the occasion the open photo was taken at (photos with
    // no pause over two hours): all its photos, and first the unnamed faces
    // that look like someone named, before everyone unnamed is hidden.
    async openEvent() {
      const im = this.current();
      if (!im || this.eventHide) return;
      this.eventHide = {id: im.id, ev: null, busy: false, skip: {}, answers: {}};
      const ev = await api(`/api/images/${im.id}/event?details`).catch(e => { this.notify(`Couldn’t look: ${e.message}`, 5000); return null; });
      if (!ev || !this.eventHide || this.eventHide.id !== im.id) { this.eventHide = null; return; }
      this.eventHide.ev = ev;
      this.closeLightbox(true);
    },
    closeEvent() { this.eventHide = null; this.peek = null; },
    // setNsfw overrules the classifier for the open photo: false "not
    // NSFW", true "NSFW", null back to the classifier.
    async setNsfw(v) {
      const im = this.current();
      if (!im) return;
      try {
        await api('/api/images/' + im.id, {method: 'PATCH', body: JSON.stringify(v === null ? {reset_nsfw: true} : {nsfw: v})}, {retry: true});
        const d = await api('/api/images/' + im.id);
        if (this.detail && this.detail.id === im.id) Object.assign(this.detail, {nsfw: d.nsfw, nsfw_user: d.nsfw_user, nsfw_score: d.nsfw_score});
        Object.assign(im, {nsfw: d.nsfw});
        touched.ids.add(im.id);
        this.refreshStats();
      } catch (e) {
        this.notify(`Couldn’t save: ${e.message}`, 6000);
      }
    },
    // ---------- family ----------
    // sgNote explains a suggestion: how they're related to another one,
    // how old they looked then, and relatives in the photos.
    sgNote(p) {
      return [p.kin, p.looked ? 'looked ~' + Math.round(p.looked) + ' then' : '', p.why, p.off].filter(Boolean).join(' · ');
    },
    async setRelation(other, kind, on) {
      const p = this.openedPerson();
      if (!p) return;
      try {
        await api(`/api/people/${p.id}/relations`, {method: 'POST', body: JSON.stringify({person_id: other, kind, on})}, {retry: true});
        await this.loadPeople(); // and the open person's page, with its faces
      } catch (e) {
        this.notify(`Couldn’t save: ${e.message}`, 6000);
      }
    },
    addRelation() {
      const name = this.kinForm.name.trim().toLowerCase();
      const q = this.peopleData.people.find(x => x.name.toLowerCase() === name);
      if (!q) { this.notify(`No one named “${this.kinForm.name.trim()}” — name them first`, 5000); return; }
      if (q.id === this.openPersonId) return;
      this.setRelation(q.id, this.kinForm.kind, true);
      this.kinForm.name = '';
    },
    // companyLabel: "With Bob, Alice · 14 days, 2010–2019 · 3 places".
    companyLabel(k) {
      if (!k) return '';
      const years = k.from === k.to ? String(k.from) : `${k.from}–${k.to}`;
      return `With ${k.with.join(', ')} · ${k.days} ${k.days === 1 ? 'day' : 'days'}, ${years}` + (k.places > 1 ? ` · ${k.places} places` : '');
    },
    eventSpan(ev) {
      if (!ev.from) return 'one photo';
      const d = t => new Date(t * 1000);
      const day = t => d(t).toLocaleDateString(undefined, {day: 'numeric', month: 'long', year: 'numeric'});
      const time = t => d(t).toLocaleTimeString(undefined, {hour: '2-digit', minute: '2-digit'});
      return day(ev.from) === day(ev.to) ? `${day(ev.from)}, ${time(ev.from)}–${time(ev.to)}` : `${day(ev.from)} – ${day(ev.to)}`;
    },
    // eventKeep: the likely faces left alone: unanswered, or said yes to.
    eventKeep() {
      const eh = this.eventHide;
      return (eh.ev.likely || []).filter(m => eh.answers[m.face.id] !== 'no' && !eh.skip[m.face.image]).map(m => m.face.id);
    },
    eventCount() {
      const eh = this.eventHide;
      if (!eh || !eh.ev) return 0;
      let n = eh.ev.faces;
      for (const id in eh.skip) if (eh.skip[id]) n -= (eh.ev.unnamed || {})[id] || 0;
      return Math.max(0, n - this.eventKeep().length);
    },
    eventSkip(im) {
      const eh = this.eventHide;
      if (eh.skip[im.id]) delete eh.skip[im.id]; else eh.skip[im.id] = true;
    },
    eventAnswer(m, ans) {
      const a = this.eventHide.answers;
      if (a[m.face.id] === ans) delete a[m.face.id]; else a[m.face.id] = ans;
    },
    // openEventPhoto opens one of the event's photos; the arrows step
    // through the event, and closing it comes back here.
    openEventPhoto(i) {
      this.lightbox.list = this.eventHide.ev.images;
      this.lightbox.back = null;
      this.lightbox.idx = i;
      this.lightbox.open = true;
      this.loadDetail();
    },
    // hideEventStrangers saves the answers (yes: named; no: not them, and
    // hidden with the rest), then hides everyone unnamed, with an undo.
    async hideEventStrangers() {
      const eh = this.eventHide;
      if (!eh || eh.busy) return;
      eh.busy = true;
      try {
        for (const m of eh.ev.likely || []) {
          const ans = eh.answers[m.face.id];
          if (!ans || eh.skip[m.face.image]) continue;
          await api(`/api/faces/${m.face.id}/${ans === 'yes' ? 'tag' : 'reject'}`, {method: 'POST', body: JSON.stringify({person_id: m.person})}, {retry: true});
        }
        const skip = Object.keys(eh.skip).map(Number);
        const ev = await api(`/api/images/${eh.id}/event/strangers`, {method: 'POST', body: JSON.stringify({skip_images: skip, keep_faces: this.eventKeep()})}, {retry: true});
        this.closeEvent();
        touched.all = true;
        this.pruneResults();
        this.loadPeople();
        this.peopleChanged();
        this.notify(`Hid ${this.fmtCount(ev.faces)} ${ev.faces === 1 ? 'face' : 'faces'} as strangers, in ${this.fmtCount(ev.photos - skip.length)} photos`, 10000,
          {label: 'Undo', fn: async () => {
            await api('/api/groups/unhide', {method: 'POST', body: JSON.stringify({groups: ev.groups})}, {retry: true}).catch(e => this.notify(`Couldn’t undo: ${e.message}`, 6000));
            this.peopleChanged();
          }});
      } catch (e) {
        eh.busy = false;
        this.notify(`Couldn’t save: ${e.message}`, 6000);
      }
    },
    // ---------- find face ----------
    // findFace looks for the faces of a picture in the library. The
    // picture is only sent to be looked at; it is not kept.
    async findFace(file) {
      if (!file || !file.type.startsWith('image/')) { this.notify('That’s not a picture', 4000); return; }
      const fd = this.finder;
      if (this.view !== 'find') this.goNav({id: 'find'});
      if (fd.file !== file) {
        if (fd.url) URL.revokeObjectURL(fd.url);
        Object.assign(fd, {file, url: URL.createObjectURL(file), faces: [], pick: 0, sel: {}, error: ''});
      }
      fd.busy = true;
      try {
        const body = new FormData();
        body.append('file', file, file.name || 'pasted');
        const res = await api('/api/face-search', {method: 'POST', body});
        if (fd.file !== file) return;
        fd.faces = res.faces || [];
        fd.pick = Math.min(fd.pick, Math.max(fd.faces.length - 1, 0));
        if (!fd.faces.length) fd.error = 'No face found in that picture.';
      } catch (e) {
        if (fd.file === file) fd.error = e.message;
      } finally {
        if (fd.file === file) fd.busy = false;
      }
    },
    finderMatches() { const f = this.finder.faces[this.finder.pick]; return (f && f.matches) || []; },
    finderSelected() { return Object.keys(this.finder.sel).filter(k => this.finder.sel[k]).map(Number); },
    finderToggle(f) { this.finder.sel[f.id] = !this.finder.sel[f.id]; },
    finderSelectGroup(g) {
      const all = g.faces.every(f => this.finder.sel[f.id]);
      for (const f of g.faces) this.finder.sel[f.id] = !all;
    },
    // finderAct names, confirms or rejects a whole group, then looks again.
    async finderAct(g, action) {
      const fd = this.finder, name = (fd.names[g.group] || '').trim();
      if (fd.saving || (action === 'name' && !name)) return;
      fd.saving = true;
      try {
        if (action === 'name') await api(`/api/groups/${g.group}/name`, {method: 'POST', body: JSON.stringify({name})}, {retry: true});
        if (action === 'correct') await api(`/api/groups/${g.group}/name`, {method: 'POST', body: JSON.stringify({person_id: g.person})}, {retry: true});
        if (action === 'reject') await api(`/api/groups/${g.group}/reject`, {method: 'POST', body: JSON.stringify({person_id: g.person})}, {retry: true});
        this.notify(action === 'reject' ? `Not ${g.name} — noted` : `Saved: ${action === 'name' ? name : g.name}`);
        delete fd.names[g.group];
        this.afterFinder();
      } catch (e) {
        this.notify(`Couldn’t save: ${e.message}`, 6000);
      } finally {
        fd.saving = false;
      }
    },
    // finderTag names just the ticked faces, wherever they are grouped.
    async finderTag() {
      const fd = this.finder, name = fd.tagName.trim(), ids = this.finderSelected();
      if (fd.saving || !name || !ids.length) return;
      fd.saving = true;
      try {
        for (const id of ids) await api(`/api/faces/${id}/tag`, {method: 'POST', body: JSON.stringify({name})}, {retry: true});
        this.notify(`Saved: ${ids.length} ${ids.length === 1 ? 'face' : 'faces'} as ${name}`);
        fd.sel = {};
        fd.tagName = '';
        this.afterFinder();
      } catch (e) {
        this.notify(`Couldn’t save: ${e.message}`, 6000);
      } finally {
        fd.saving = false;
      }
    },
    afterFinder() {
      touched.all = true;
      this.loadPeople();
      this.peopleChanged();
      if (this.finder.file) this.findFace(this.finder.file); // names changed: look again
    },
    // A picture pasted or dropped anywhere (not into a text field) is
    // looked for.
    onPaste(e) {
      if (e.target.matches && e.target.matches('input, textarea')) return;
      const item = [...(e.clipboardData?.items || [])].find(i => i.type.startsWith('image/'));
      if (!item) return;
      e.preventDefault();
      this.findFace(item.getAsFile());
    },
    onDrop(e) {
      this.finder.drag = false;
      const file = [...(e.dataTransfer?.files || [])].find(f => f.type.startsWith('image/'));
      if (file) this.findFace(file);
    },
    // ---------- Discover ----------
    // loadDiscover fetches the queue (more: a page more of it), leaving out
    // what was answered or skipped here.
    // The questions answered or skipped are remembered for an hour across
    // reloads: the server may still serve a list built before the answer.
    saveDqDone() {
      const now = Date.now(), d = this.discover;
      d.doneAt = d.doneAt || {};
      for (const k of Object.keys(d.done)) d.doneAt[k] = d.doneAt[k] || now;
      for (const [k, t] of Object.entries(d.doneAt)) {
        if (now - t > 3600e3) { delete d.done[k]; delete d.doneAt[k]; }
      }
      try { localStorage.setItem('dqDone', JSON.stringify({done: d.done, at: d.doneAt})); } catch (e) {}
    },
    loadDqDone() {
      try {
        const v = JSON.parse(localStorage.getItem('dqDone') || 'null');
        if (v && v.done) { this.discover.done = v.done; this.discover.doneAt = v.at || {}; }
      } catch (e) {}
      this.saveDqDone(); // drops the stale
    },
    async loadDiscover(more) {
      const d = this.discover;
      if (!this._dqLoaded) {
        this._dqLoaded = true;
        this.loadDqDone();
        try {
          d.tab = localStorage.getItem('dqTab') || 'all';
          d.whoSort = localStorage.getItem('dqWhoSort') || '';
          d.ageMin = +localStorage.getItem('dqAgeMin') || 0;
          d.ageMax = +localStorage.getItem('dqAgeMax') || 90;
          d.minMatch = +localStorage.getItem('dqMinMatch') || 0;
        } catch (e) {}
      }
      d.loading = true;
      const want = more ? d.queue.length + d.skipped + 30 : Math.max(30, d.queue.length + d.skipped);
      try {
        const tab = this.dqTabs.find(t => t.id === d.tab);
        const r = await api('/api/discover/queue?limit=' + want + (tab && tab.kinds ? '&kind=' + tab.kinds.join(',') : '') +
          (d.ageMin > 0 ? '&age_min=' + d.ageMin : '') + (d.ageMax < 90 ? '&age_max=' + d.ageMax : '') + (d.minMatch ? '&min=' + d.minMatch / 100 : '') + (d.tab === 'who' && d.whoSort ? '&who_sort=' + d.whoSort : ''));
        d.total = r.total;
        d.kinds = r.kinds || {};
        const items = r.items.filter(it => d.done[it.key] !== this.dqAsked(it));
        // How the people just named are related comes first, while
        // they're in mind; the newest named first.
        const rank = it => {
          if (it.kind !== 'relation') return d.recent.length;
          const i = d.recent.findIndex(p => p === it.relation.a || p === it.relation.b);
          return i < 0 ? d.recent.length : i;
        };
        d.queue = items.map((it, i) => [rank(it), i, it]).sort((a, b) => a[0] - b[0] || a[1] - b[1]).map(x => x[2]);
        d.skipped = r.items.length - d.queue.length;
        d.since = 0;
      } catch (e) {
        this.notify(`Couldn’t load Discover: ${e.message}`, 6000);
      }
      d.loading = false;
    },
    // dqAsked is what a question asks, to tell a new one under the same key.
    dqAsked(it) {
      const fs = it.kind === 'more' ? it.more.faces : it.kind === 'who' ? it.who.faces : null;
      return fs ? fs.map(f => f.id).join(',') : it.key;
    },
    // dqDone takes a question off the list (answered or skipped), and
    // fetches the queue again once the server has learnt from the answer.
    dqDone(it, answered) {
      const d = this.discover;
      d.done[it.key] = this.dqAsked(it);
      this.saveDqDone();
      d.queue = d.queue.filter(x => x !== it);
      d.since++;
      // All shown answered: the next ones, if the server has more.
      if (!d.queue.length && !d.loading) this.loadDiscover();
      if (!answered) return;
      d.total = Math.max(0, d.total - 1);
      // The new questions load when the server is done learning from it
      // (onLearn); this is in case that word is missed.
      clearTimeout(this._dqT1);
      this._dqT1 = setTimeout(() => this.view === 'discover' && !this.learn.busy && this.loadDiscover(), 45000);
    },
    // Discover's tabs: everything, or one kind of question.
    dqTabs: [
      {id: 'all', label: 'All'},
      {id: 'who', label: 'Who is this?', kinds: ['who']},
      {id: 'more', label: 'More of…', kinds: ['more']},
      {id: 'rel', label: 'Related?', kinds: ['relation']},
      {id: 'face', label: 'Is this really them?', kinds: ['face', 'shot']},
      {id: 'pic', label: 'Pictures', kinds: ['picture']},
      {id: 'date', label: 'Dates', kinds: ['session', 'date']},
      {id: 'birth', label: 'Born?', kinds: ['birth']},
    ],
    dqTabCount(t) {
      const k = this.discover.kinds || {};
      return t.kinds ? t.kinds.reduce((n, x) => n + (k[x] || 0), 0) : Object.values(k).reduce((n, x) => n + x, 0);
    },
    // dqView sets how Discover is shown (any of whoSort: the order of
    // "Who is this?", ageMin and ageMax: the ages asked about (90: no
    // upper bound), minMatch: the least match % offered), kept for next
    // time, and loads it so.
    dqView(o) {
      const d = this.discover;
      Object.assign(d, o);
      try {
        localStorage.setItem('dqWhoSort', d.whoSort);
        localStorage.setItem('dqAgeMin', String(d.ageMin));
        localStorage.setItem('dqAgeMax', String(d.ageMax));
        localStorage.setItem('dqMinMatch', String(d.minMatch));
      } catch (e) {}
      d.queue = [];
      d.skipped = 0;
      this.loadDiscover();
    },
    whoSorts: [
      {id: '', label: 'Most worth knowing'},
      {id: 'family', label: 'Most days with someone you know'},
      {id: 'days', label: 'Seen on the most days'},
      {id: 'photos', label: 'In the most photos'},
      {id: 'recent', label: 'Seen most recently'},
    ],
    dqSetTab(t) {
      const d = this.discover;
      if (d.tab === t.id) return;
      d.tab = t.id;
      d.queue = [];
      d.skipped = 0;
      try { localStorage.setItem('dqTab', t.id); } catch (e) {}
      this.loadDiscover();
    },
    dqTicked(faces) { return faces.filter(f => !this.discover.off[f.id]); },
    // dqFirst ticks the first n faces (the best matches) and unticks the rest.
    dqFirst(faces, n) { faces.forEach((f, i) => { this.discover.off[f.id] = i >= n; }); },
    // dqFirstN is how many faces from the first are ticked in a row.
    dqFirstN(faces) {
      const i = faces.findIndex(f => this.discover.off[f.id]);
      return i < 0 ? faces.length : i;
    },
    dqTickAll(faces) {
      const on = this.dqTicked(faces).length === 0;
      for (const f of faces) this.discover.off[f.id] = !on;
    },
    // dqSend names the ticked faces as a person (by id or name), and says
    // the unticked ones aren't them.
    // The card goes at once and the answer is saved behind it, in click
    // order: the server may be busy learning, the user never waits for it.
    dqSend(it, faces, body, msg) {
      const d = this.discover;
      const yes = this.dqTicked(faces).map(f => f.id), no = faces.filter(f => d.off[f.id]).map(f => f.id);
      const at = d.queue.indexOf(it);
      this.dqDone(it, true);
      this.notify(msg(yes.length));
      return this.saveBehind(async () => {
        const r = await api('/api/discover/name', {method: 'POST', body: JSON.stringify({...body, yes, no})}, {retry: true});
        // Someone just named: how they're related is asked next.
        if (it.kind === 'who' && r && r.person_id) d.recent = [r.person_id, ...d.recent.filter(p => p !== r.person_id)].slice(0, 5);
        this.loadPeople();
        this.peopleChanged();
      }, () => {
        // Not saved: the question comes back.
        delete d.done[it.key];
        this.saveDqDone();
        if (!d.queue.includes(it)) d.queue.splice(Math.max(0, at), 0, it);
        d.total++;
      });
    },
    // saveBehind runs a save after the ones before it; on failure it says
    // so and undoes what was shown (undo).
    saveBehind(fn, undo) {
      this._saves = (this._saves || Promise.resolve()).then(fn).catch(e => {
        if (undo) undo();
        this.notify(`Couldn’t save: ${e.message}`, 6000);
      });
      return this._saves;
    },
    // faceSave answers about a face in the photo viewer: the card closes
    // and the face shows the answer at once (local: its new fields), saved
    // behind; the photo's faces are fetched again once saved.
    faceSave(f, local, call, msg) {
      this.closeAssign();
      touched.ids.add(f.image);
      const im = this.current();
      if (this.detail && this.detail.faces) {
        this.detail = {...this.detail, faces: local === null ? this.detail.faces.filter(x => x.id !== f.id)
          : this.detail.faces.map(x => x.id === f.id ? {...x, ...local} : x)};
      }
      if (msg) this.notify(msg);
      this._faceSaves = (this._faceSaves || 0) + 1;
      return this.saveBehind(async () => {
        const res = await call().finally(() => this._faceSaves--);
        // Fetched again only after the last answer: else it'd show the
        // answers still to be saved undone for a moment.
        if (this.current() === im && !this._faceSaves) await this.refreshFaces();
        this.loadPeople();
        this.peopleChanged();
        return res;
      }, () => this.current() === im && this.refreshFaces());
    },
    // nameWho names a "Who is this?": as typed, or as the person it looks
    // most like (person, an id).
    nameWho(it, person) {
      const name = person ? it.who.maybe_name : (this.discover.names[it.key] || '').trim();
      if (!name || this.discover.busy) return;
      const picture = !!this.discover.paint[it.key];
      return this.dqSend(it, it.who.faces, person ? {person, picture} : {name, picture},
        n => `Saved: ${n} face${n === 1 ? '' : 's'} of ${name}` + (picture ? ' (a painting or statue)' : ''));
    },
    acceptMore(it) {
      const m = it.more;
      return this.dqSend(it, m.faces, {person: m.person},
        n => n ? `Confirmed ${n} face${n === 1 ? '' : 's'} of ${m.name}` : `None are ${m.name}`);
    },
    // answerBirth enters someone's birth year (0: not known, not asked again).
    async answerBirth(it, year) {
      const b = it.birth;
      if (year && (year < 1850 || year > new Date().getFullYear())) return;
      this.dqDone(it, true);
      try {
        await api(`/api/people/${b.person}`, {method: 'PATCH', body: JSON.stringify(year ? {birth_year: year} : {birth_unknown: true})}, {retry: true});
        this.notify(year ? `${b.name}: born ${year}` : `Won’t ask when ${b.name} was born`);
        this.loadPeople();
        this.peopleChanged();
      } catch (e) {
        this.notify(`Couldn’t save: ${e.message}`, 6000);
      }
    },
    async hideWho(it) {
      this.dqDone(it, true);
      await api('/api/discover/hide', {method: 'POST', body: JSON.stringify({groups: it.who.groups})}, {retry: true}).catch(e => this.notify(`Couldn’t save: ${e.message}`, 6000));
      this.peopleChanged();
    },
    // ---------- faces that don't fit who they're filed under ----------
    // faceDoubtWhy says why: the wrong age for the photo, unlike the rest,
    // more like someone else, the other gender.
    faceDoubtWhy(d) {
      const why = [];
      if (d.age_off) {
        why.push(`looks ${Math.round(d.looks)} in a ${Math.round(d.year)} photo; ${d.name} was ${Math.round(d.year - d.born)} then`);
      }
      const pc = x => Math.round(x * 100) + '%';
      if (d.ahead) why.push(`${d.ahead_name} scores higher now: ${pc(d.ahead_score)}` + (d.ahead_then ? ` (${pc(d.ahead_then)} when you confirmed it)` : ''));
      else if (d.other) why.push(`more like ${d.other_name} (${pc(d.other_fit)})`);
      else if (d.fit < 0.3) why.push(`unlike ${d.name}’s other faces`);
      if (d.then) why.push(`was ${pc(d.then)} when you confirmed it`);
      if (d.cost_why && !d.age_off) why.push(d.cost_why);
      if (d.weak && !why.length) why.push(`too unlike to be filed by itself`);
      if (d.gender) why.push(`reads as a ${d.gender === 'male' ? 'woman' : 'man'}`);
      return why.join(' · ');
    },
    // A face looking much younger than its person then: likely a photo of
    // a photo (on a wall, in an album).
    faceDoubtPicture(d) { return d.age_off && d.year - d.born > d.looks; },
    async answerFaceDoubt(it, how) {
      const d = it.face;
      this.dqDone(it, true);
      const id = d.face.id;
      const [url, body, msg] = {
        yes: [`/api/faces/${id}/tag`, {person_id: d.person}, `Confirmed: ${d.name}`],
        other: [`/api/faces/${id}/tag`, {person_id: d.other}, `Moved to ${d.other_name}`],
        no: [`/api/faces/${id}/reject`, {person_id: d.person}, `Taken out of ${d.name}`],
        picture: [`/api/faces/${id}/picture`, {on: true, person_id: d.person}, `A picture of ${d.name}`],
      }[how];
      try {
        await api(url, {method: 'POST', body: JSON.stringify(body)}, {retry: true});
        this.notify(msg);
        this.peopleChanged();
      } catch (e) {
        this.notify(`Couldn’t save: ${e.message}`, 6000);
      }
    },
    // ---------- wrong dates ----------
    // dateWhy says why a photo's date is doubted: who's in it, how old they look.
    dateWhy(p) {
      if (p.unborn) return `${p.unborn} wasn’t born yet on its date`;
      return p.faces.map(f => `${f.name} looks ${Math.round(f.looks)}`).join(', ') +
        (p.faces.length === 1 ? `, born ~${Math.round(p.faces[0].born)}` : '');
    },
    // setYears dates photos to the middle of a year each, or keeps their
    // date (ok); the sources stay as they are: the date is kept here, the
    // file's own beside it. it: the Discover question it answers.
    async setYears(list, what, it) {
      const d = this.discover;
      if (d.dating) return;
      d.dating = what;
      let done = 0, failed = 0;
      const todo = [...list];
      const work = async () => {
        for (let job; (job = todo.shift());) {
          const body = job.ok ? {date_ok: true} : {taken: Date.UTC(job.year, 6, 1, 12) / 1000};
          try { await api('/api/images/' + job.id, {method: 'PATCH', body: JSON.stringify(body)}); done++; } catch (e) { failed++; }
        }
      };
      await Promise.all([work(), work(), work(), work()]);
      d.dating = '';
      this.notify(failed ? `Dated ${done}, ${failed} failed` : (list.length === 1 && list[0].ok ? 'Kept its date' : `Dated ${fmtCount(done)} photo${done === 1 ? '' : 's'}`), 4000);
      if (it) this.dqDone(it, true);
      this.peopleChanged();
    },
    // What B is to A, in words, with B's gender when known.
    relWord(kind, gender) {
      const w = {
        partner: {'': 'partner'}, // not married, necessarily
        child: {female: 'daughter', male: 'son', '': 'child'},
        parent: {female: 'mother', male: 'father', '': 'parent'},
        sibling: {female: 'sister', male: 'brother', '': 'sibling'},
        friend: {'': 'friend'},
        grandchild: {female: 'granddaughter', male: 'grandson', '': 'grandchild'},
        grandparent: {female: 'grandmother', male: 'grandfather', '': 'grandparent'},
      }[kind];
      return w ? (w[gender || ''] || w['']) : kind;
    },
    relQuestion(g) {
      if (g.kind === 'partner') {
        // Partners come one after another: "Were Dan and Erin
        // partners, before Fay?"
        const other = g.then_of === g.a_name ? g.b_name : g.a_name;
        if (g.then) return `Was ${g.then_of} in a partnership with ${other} ${g.then === 'before' ? 'earlier, before' : 'later, after'} ${g.then_with}?`;
        const was = g.last_year < new Date().getFullYear() - 2;
        return `${was ? 'Were' : 'Are'} ${g.a_name} and ${g.b_name} partners?`;
      }
      return `Is ${g.b_name} ${g.a_name}’s ${this.relWord(g.kind, g.b_gender)}?`;
    },
    relNote(g) {
      const yrs = g.first_year === g.last_year ? g.first_year : `${g.first_year}–${g.last_year}`;
      return `Together in ${this.fmtCount(g.photos)} photos on ${this.fmtCount(g.days)} days, ${yrs}` +
        (g.beside ? ` · side by side in ${this.fmtCount(g.beside)}` : '') +
        (g.duo ? ` · just the two of them in ${this.fmtCount(g.duo)}` : '') +
        (g.a_birth && g.b_birth ? ` · born ~${Math.round(g.a_birth)} and ~${Math.round(g.b_birth)}` : '');
    },
    // Why a guess is asked though it doesn't square with the family
    // entered, or what entered it puts right.
    relWarn(g) {
      if (g.entered) {
        return `${g.b_name} is entered as ${g.a_name}’s ${this.relWord(g.entered, g.b_gender)}, ` +
          `but looks born ~${Math.round(g.b_birth)}, and ${g.a_name} ~${Math.round(g.a_birth)}`;
      }
      return g.clash ? `Though ${g.clash}` : '';
    },
    // familyTrees is the family entered, laid out as trees (see
    // layoutFamily), for the Family view; laid out again when people change.
    familyTrees() {
      const ppl = this.peopleData.people;
      if (this._treesFor !== ppl) {
        this._treesFor = ppl;
        this._trees = layoutFamily(ppl || []);
      }
      return this._trees;
    },
    treeSVG(t) {
      return t.lines.map(l => `<path class="tl-${l.kind}" d="${l.d}"/>`).join('');
    },
    // treeRel is what n is to the person hovered ("aunt"), '' if nothing.
    treeRel(n) {
      if (!this.treeHover || this.treeHover === n.p.id) return '';
      const h = (this.peopleData.people || []).find(p => p.id === this.treeHover);
      const r = h && (h.family || []).find(r => r.id === n.p.id);
      return r ? r.word || r.kind : '';
    },
    treeNote(n) {
      if (this.treeHover && this.treeHover !== n.p.id) return this.treeRel(n);
      const b = n.p.birth_year || (n.p.est_birth ? '~' + Math.round(n.p.est_birth) : '');
      return b ? 'b. ' + b : '';
    },
    // The guess as the first answer, a choice rather than "Yes":
    // "Partners", "Son", "Sister".
    relAnswer(g) {
      const w = g.kind === 'partner' ? 'partners' : this.relWord(g.kind, g.b_gender);
      return w.charAt(0).toUpperCase() + w.slice(1);
    },
    // The other answers offered, besides the guess: only the ties entered;
    // grandparents, cousins and the like follow from them.
    relOthers(g) {
      return ['friend', 'partner', 'child', 'parent', 'sibling'].filter(k => k !== g.kind);
    },
    // answerShot: both faces are pid, or (0) two people after all.
    answerShot(it, pid) {
      const sp = it.shot;
      this.dqDone(it, true);
      const who = pid === sp.a_person ? sp.a_name : pid === sp.b_person ? sp.b_name : '';
      this.notify(who ? `Both ${who}` : 'Noted: two people');
      this.saveBehind(async () => {
        await api('/api/discover/shot', {method: 'POST', body: JSON.stringify({a: sp.a.id, b: sp.b.id, person: pid})}, {retry: true});
        this.peopleChanged();
      });
    },
    async answerRelation(it, kind) {
      const g = it.relation;
      this.dqDone(it, true);
      try {
        await api('/api/discover/relations', {method: 'POST', body: JSON.stringify({a: g.a, b: g.b, kind, was: g.entered || ''})}, {retry: true});
        this.notify(kind === 'skip' ? (g.entered ? 'Kept as entered' : 'Won’t ask again')
          : `Saved: ${g.b_name} is ${g.a_name}’s ${this.relWord(kind, g.b_gender)}`, 6000);
        this.peopleChanged();
      } catch (e) {
        this.notify(`Couldn’t save: ${e.message}`, 6000);
      }
    },
    // ---------- pictures on the wall ----------
    pictureNote(set) {
      const y = t => new Date(t * 1000).getFullYear();
      return `${set.photos} photos, ${y(set.from)}–${y(set.to)}, always looking about ${Math.round(set.age)}`;
    },
    async loadPictures(pid) {
      this.pictures[pid] = await api(`/api/people/${pid}/pictures`).catch(() => []);
    },
    // openPictures shows a picture's faces, all ticked: untick any that
    // are the person for real, then say whether it's a picture.
    // it: the Discover question it answers, if asked there.
    openPictures(pid, set, it) {
      const tick = {}, inTime = new Set(set.in_time || []);
      for (const f of set.faces) tick[f.id] = !inTime.has(f.id); // their age fits then: likely the real photos
      for (const w of set.with || []) for (const f of w.faces) tick[f.id] = true;
      this.picView = {pid, set, it, tick, busy: false, with: (set.with || []).map(w => ({person: w.person || 0, faces: w.faces}))};
    },
    picTicked(faces) { return faces.filter(f => this.picView.tick[f.id]).map(f => f.id); },
    // picAnswer closes at once; the answer is saved behind it.
    picAnswer(yes) {
      const v = this.picView;
      if (!v) return;
      const send = (pid, faces, picture) => faces.length &&
        api(`/api/people/${pid}/pictures`, {method: 'POST', body: JSON.stringify({faces, picture})}, {retry: true});
      const sets = yes ? [[v.pid, this.picTicked(v.set.faces), true], ...v.with.map(w => [w.person, this.picTicked(w.faces), true]),
        // Unticked: them for real; not to be asked again.
        [v.pid, v.set.faces.filter(f => !v.tick[f.id]).map(f => f.id), false]]
        : [[v.pid, v.set.faces.map(f => f.id), false]];
      this.picView = null;
      if (v.it) this.dqDone(v.it, true);
      this.notify(yes ? 'Marked as a picture on the wall' : 'Noted: not a picture');
      this.saveBehind(async () => {
        for (const [pid, faces, picture] of sets) await send(pid, faces, picture);
        touched.all = true;
        this.peopleChanged();
        this.loadPeople();
        this.loadPictures(v.pid);
      });
    },
    // ---------- Lane Match ----------
    personCover(pid) { const p = this.peopleData.people.find(p => p.id === pid); return p ? p.cover : ''; },
    async openLanes() {
      const L = this.lanes;
      try { L.autoOn = localStorage.getItem('laneAuto') === '1'; } catch (e) {}
      if (!L.people.length) {
        try { L.people = JSON.parse(localStorage.getItem('lanePeople') || '[]'); } catch (e) {}
        if (!L.people.length) L.people = await api('/api/lanes/people?n=6').catch(() => []);
      }
      L.people.forEach(pid => { this.loadLane(pid); this.loadPictures(pid); });
    },
    saveLanes() {
      try {
        localStorage.setItem('lanePeople', JSON.stringify(this.lanes.people));
        localStorage.setItem('laneAuto', this.lanes.autoOn ? '1' : '');
      } catch (e) {}
    },
    // loadLane fetches a person's likeliest faces; those already shown keep
    // their place, new ones come in after them.
    async loadLane(pid) {
      const L = this.lanes;
      if (!(pid in this.pictures)) this.loadPictures(pid);
      const fresh = await api(`/api/people/${pid}/face-candidates?limit=40`).catch(() => null);
      if (!fresh || !L.people.includes(pid)) return;
      const ok = fresh.filter(f => !L.gone[f.id]);
      const ids = new Set(ok.map(f => f.id));
      const kept = (L.faces[pid] || []).filter(f => ids.has(f.id) && !L.gone[f.id]);
      const have = new Set(kept.map(f => f.id));
      L.faces[pid] = [...kept, ...ok.filter(f => !have.has(f.id))];
    },
    // laneAnswer: is face f person pid? It fades out of every row; the
    // answer is saved in the background; a few seconds after the last
    // answer, the rows refresh.
    laneAnswer(pid, f, yes) {
      const L = this.lanes;
      L.gone[f.id] = true;
      this.peek = null;
      setTimeout(() => {
        for (const p of L.people) if (L.faces[p]) L.faces[p] = L.faces[p].filter(x => x.id !== f.id);
      }, 260);
      touched.ids.add(f.image);
      alsoPending++;
      alsoQueue = alsoQueue.then(async () => {
        try {
          await api(`/api/faces/${f.id}/${yes ? 'tag' : 'reject'}`, {method: 'POST', body: JSON.stringify({person_id: pid})}, {retry: true});
        } catch (e) {
          this.notify(`Couldn’t save the answer: ${e.message}`, 6000);
        }
      });
      alsoQueue.then(() => { alsoPending--; });
      clearTimeout(L.timer);
      L.timer = setTimeout(() => this.refreshLanes(), 3000);
    },
    // refreshLanes, after a pause in answering: what is now near certain
    // is filed by itself, then every row reloads with what was learnt.
    async refreshLanes() {
      const L = this.lanes;
      await alsoQueue;
      if (this.view !== 'lanes') return;
      for (const pid of L.autoOn ? L.people : []) {
        const res = await api(`/api/lanes/${pid}/auto`, {method: 'POST'}, {retry: true}).catch(() => null);
        if (res && res.accepted.length) {
          L.auto[pid] = [...(L.auto[pid] || []), ...res.accepted];
          res.accepted.forEach(a => { L.gone[a.face] = true; });
        }
      }
      this.peopleChanged();
      this.loadPeople();
      L.people.forEach(pid => this.loadLane(pid));
    },
    async undoAuto(pid) {
      const L = this.lanes, list = L.auto[pid] || [];
      L.auto[pid] = [];
      for (const a of list) {
        await api(`/api/faces/${a.face}/unaccept`, {method: 'POST', body: JSON.stringify({from: a.from})}, {retry: true}).catch(() => {});
        delete L.gone[a.face];
      }
      this.peopleChanged();
      this.loadLane(pid);
    },
    addLane() {
      const name = this.lanes.add.trim().toLowerCase();
      const p = this.peopleData.people.find(x => x.name.toLowerCase() === name);
      if (!p) { this.notify(`No one named “${this.lanes.add.trim()}”`, 4000); return; }
      this.lanes.add = '';
      if (!this.lanes.people.includes(p.id)) {
        this.lanes.people = [p.id, ...this.lanes.people];
        this.saveLanes();
        this.loadLane(p.id);
      }
    },
    closeLane(pid) {
      this.lanes.people = this.lanes.people.filter(x => x !== pid);
      delete this.lanes.faces[pid];
      this.saveLanes();
    },
    // goPerson opens a person's page from the photo viewer.
    goPerson(id) {
      this.closeLightbox(true);
      this.openPerson({id});
    },
    alsoName() { return this.also.person ? this.also.person.name || 'them' : ''; },
    // closeLightbox closes the photo; Back too, when opening it added the
    // history entry. leaving: another page follows (nothing to go back to).
    closeLightbox(leaving) {
      const changed = touched.ids.size > 0 || touched.all;
      const back = !leaving && !this.restoring && this.lightbox.open && history.state && history.state.photo;
      this.closeAlso();
      this.lightbox.open = false;
      this.lightbox.list = null;
      this.lightbox.back = null;
      this.around = {id: 0, by: '', items: [], more: [false, false]};
      this.detail = null;
      if (back) history.back(); // to the page it was opened from; nothing else to restore
      else this.writeHash();
      this.pruneResults();
      if (this.lightbox.card) this.refreshCard(this.lightbox.card);
      this.lightbox.card = 0;
      // Faces may have been named in the photo: the open person's page follows.
      if (changed && this.view === 'people' && this.openPersonId) this.loadPerson();
      if (changed && this.view === 'match') alsoQueue.then(() => this.loadMatches());
    },
    // pruneResults drops photos that no longer match a people search
    // ($unknown, a name, ...) after their faces were named in the viewer,
    // in place, keeping the scroll position.
    async pruneResults() {
      if (!touched.ids.size && !touched.all) return;
      await alsoQueue; // answers still being saved
      const all = touched.all, ids = new Set(touched.ids);
      touched.ids.clear();
      touched.all = false;
      if (!this.person && !this.searchPeople.length && !this.searchSpecial.length) return;
      const check = this.items.map(im => im.id).filter(id => all || ids.has(id));
      const keep = new Set();
      for (let i = 0; i < check.length; i += 400) {
        const part = check.slice(i, i + 400);
        const res = await api('/api/images?' + this.query({offset: 0, limit: part.length, ids: part.join(',')})).catch(() => null);
        if (!res) return;
        res.items.forEach(im => keep.add(im.id));
      }
      const gone = new Set(check.filter(id => !keep.has(id)));
      if (gone.size) this.removeItems(gone);
    },
    // removeItems takes photos out of the loaded results, keeping the
    // first photo still on screen where it was.
    async removeItems(gone) {
      const el = this.scroller();
      const sel = this.view === 'table' ? 'tr[data-id]' : '.tile[data-id]';
      const top = el ? el.getBoundingClientRect().top : 0;
      let anchor = null, off = 0;
      for (const t of el ? el.querySelectorAll(sel) : []) {
        const r = t.getBoundingClientRect();
        if (r.bottom > top && !gone.has(+t.dataset.id)) { anchor = t.dataset.id; off = r.top - top; break; }
      }
      const open = this.lightbox.open && !this.lightbox.list ? this.current() : null;
      this.items = this.items.filter(im => !gone.has(im.id));
      this.total = Math.max(0, this.total - gone.size);
      if (open) this.lightbox.idx = Math.max(0, this.items.findIndex(im => im.id === open.id));
      this.layout();
      // Alpine redraws in a microtask queued by the change above; this one
      // runs after it and before the browser paints, so nothing jumps.
      await Promise.resolve();
      const t = anchor && el.querySelector(`${sel}[data-id="${anchor}"]`);
      if (t) el.scrollTop += t.getBoundingClientRect().top - top - off;
    },
    async step(d) {
      const n = this.lightbox.idx + d;
      if (n < 0) return;
      if (!this.lightbox.list && n >= this.items.length && this.moreLater()) await this.loadMoreNow();
      if (n >= this.lightboxItems().length) return;
      this.lightbox.idx = n;
      this.loadDetail();
      // Warm the cache for the next image.
      const next = this.lightboxItems()[n + d];
      if (next) new Image().src = this.previewURL(next);
    },
    // ---------- delete (move to the deleted-photos folder) ----------
    async askDelete() {
      if (!this.current()) return;
      const d = this.deleteAsk;
      let t = '';
      try { t = localStorage.getItem('trashDir') || ''; } catch (e) {}
      if (!t) t = (await api('/api/trash').catch(() => ({}))).default || '';
      d.trash = t;
      d.open = true;
      this.$nextTick(() => this.$refs.deleteGo && this.$refs.deleteGo.focus());
    },
    async confirmDelete() {
      const d = this.deleteAsk, im = this.current();
      if (!im || d.busy || !d.trash.trim()) return;
      d.busy = true;
      try {
        const res = await api('/api/images/delete', {method: 'POST', body: JSON.stringify({ids: [im.id], trash: d.trash})});
        try { localStorage.setItem('trashDir', res.trash); } catch (e) {}
        if (res.deleted) {
          d.open = false;
          this.dropFromViewer(im.id);
          this.notify(`Moved to ${res.trash}`, 6000);
        } else {
          this.notify(`Couldn’t delete: ${(res.errors || []).join('; ')}`, 9000);
        }
      } catch (e) {
        this.notify(`Couldn’t delete: ${e.message}`, 8000);
      } finally {
        d.busy = false;
      }
    },
    // dropFromViewer takes a deleted photo out of what is shown and opens
    // the next one.
    dropFromViewer(id) {
      const lists = [this.items, this.lightbox.list].filter(Boolean);
      for (const l of new Set(lists)) {
        const i = l.findIndex(x => x.id === id);
        if (i >= 0) l.splice(i, 1);
      }
      if (typeof this.total === 'number' && this.total > 0) this.total--;
      const n = this.lightboxItems().length;
      if (!n) { this.closeLightbox(); return; }
      this.lightbox.idx = Math.min(this.lightbox.idx, n - 1);
      this.loadDetail();
      if (typeof this.layout === 'function') this.layout();
    },
    async loadDetail() {
      this.closeAssign();
      this.resetZoom();
      this.dateEdit = null;
      const im = this.current();
      this.writeHash();
      if (!im) return;
      this.detail = {...im, files: []};
      this.loadAround();
      const [d, faces] = await Promise.all([
        api('/api/images/' + im.id).catch(() => null),
        api('/api/images/' + im.id + '/faces').catch(() => []),
      ]);
      if (d && this.current() && this.current().id === d.id) this.detail = {...d, faces};
    },
    // ---------- around: the photos taken just before and after ----------
    toggleStrip() {
      this.lightbox.strip = !this.lightbox.strip;
      try { localStorage.setItem('showStrip', this.lightbox.strip ? '1' : ''); } catch (e) {}
      if (this.lightbox.strip) this.loadAround();
      else if (this.lightbox.back) this.backToResults();
    },
    // loadAround fetches the photos around the open one, unless the strip
    // already holds it with enough on both sides.
    async loadAround() {
      const im = this.current();
      if (!im || !this.lightbox.strip) return;
      const a = this.around, i = a.items.findIndex(x => x.id === im.id);
      if (i >= 0 && (i >= 12 || !a.more[0]) && (a.items.length - 1 - i >= 12 || !a.more[1])) {
        a.id = im.id;
        this.centerStrip(true);
        return;
      }
      const n = 40, id = im.id;
      const res = await api(`/api/images/${id}/around?n=${n}`).catch(() => null);
      if (!res || !this.current() || this.current().id !== id) return;
      const at = res.items.findIndex(x => x.id === id);
      this.around = {id, by: res.by, items: res.items, more: [at === n, res.items.length - 1 - at === n]};
      // Walking the strip: it is the list the arrows step through.
      if (this.lightbox.back) {
        this.lightbox.list = res.items;
        this.lightbox.idx = at;
      }
      this.centerStrip(i >= 0);
    },
    centerStrip(smooth) {
      this.$nextTick(() => {
        const el = this.$refs.strip, t = el && el.querySelector('.cur');
        if (t) el.scrollTo({left: t.offsetLeft - (el.clientWidth - t.offsetWidth) / 2, behavior: smooth ? 'smooth' : 'instant'});
      });
    },
    // goAround opens a photo from the strip; the arrows then walk through
    // time (or the folder) until "Back to results".
    goAround(im) {
      if (!this.lightbox.back) this.lightbox.back = {list: this.lightbox.list, idx: this.lightbox.idx, id: this.current().id};
      this.lightbox.list = this.around.items;
      this.lightbox.idx = this.around.items.indexOf(im);
      this.loadDetail();
    },
    backToResults() {
      const b = this.lightbox.back;
      if (!b) return;
      this.lightbox.back = null;
      this.lightbox.list = b.list;
      const i = (b.list || this.items).findIndex(x => x.id === b.id);
      this.lightbox.idx = i >= 0 ? i : Math.min(b.idx, Math.max(this.lightboxItems().length - 1, 0));
      this.loadDetail();
    },
    // aroundGap labels a jump in time between two photos in the strip.
    aroundGap(i) {
      if (this.around.by !== 'time' || i === 0) return '';
      const a = this.around.items[i - 1].taken, b = this.around.items[i].taken;
      const h = (b - a) / 3600;
      if (h < 3) return '';
      if (h < 48) return Math.round(h) + ' h';
      const d = h / 24;
      if (d < 60) return Math.round(d) + ' days';
      if (d < 730) return Math.round(d / 30.4) + ' months';
      return Math.round(d / 365.25) + ' years';
    },
    async saveEdit(patch) {
      const im = this.current();
      if (!im) return;
      await api('/api/images/' + im.id, {method: 'PATCH', body: JSON.stringify(patch)});
      Object.assign(im, patch);
      if (this.detail && this.detail.id === im.id) Object.assign(this.detail, patch);
    },
    // rotateCurrent turns the open photo (its camera recorded the wrong
    // orientation), by 90 (right), -90 (left) or 180. The view updates at
    // once, and it can be turned again right away; thumbnail and faces
    // follow once it has been analysed again upright, a few seconds later.
    async rotateCurrent(by) {
      const im = this.current();
      if (!im || this.rotating) return;
      this.resetZoom();
      this.rotating = true; // just while the turn is saved
      let res;
      // The faces from before the first of several quick turns.
      const old = turning.get(im.id) || new Set(((this.detail && this.detail.faces) || []).map(f => f.id));
      turning.set(im.id, old);
      try {
        res = await api(`/api/images/${im.id}/rotate`, {method: 'POST', body: JSON.stringify({by})}, {retry: true});
      } catch (e) {
        this.notify(`Couldn’t turn it: ${e.message}`, 5000);
        return;
      } finally {
        this.rotating = false;
      }
      // Every copy of it on the page (results, the strip…) turns too; a
      // quarter turn swaps its shape, so the grid lays out again.
      const turn = x => {
        if (x.id !== im.id || x.rot === res.rot) return;
        if (((res.rot - (x.rot || 0)) % 180 + 180) % 180 === 90) [x.w, x.h] = [x.h, x.w];
        x.rot = res.rot;
      };
      [im, ...this.items, ...(this.lightbox.list || []), ...this.around.items].forEach(turn);
      this.layout();
      if (this.detail && this.detail.id === im.id) this.detail = {...this.detail, rot: res.rot, faces: []};
      this.notify('Turned — finding faces again…');
      try {
        // Faces and thumbnail come back once the import stage has redone it.
        for (const wait of [3000, 5000, 8000, 15000]) {
          await new Promise(r => setTimeout(r, wait));
          if (!this.current() || this.current().id !== im.id || im.rot !== res.rot) break; // moved on, or turned again
          await this.refreshFaces();
          const fs = (this.detail && this.detail.faces) || [];
          if (fs.length && !fs.some(f => old.has(f.id))) { turning.delete(im.id); break; } // analysed again
          if (fs.some(f => old.has(f.id))) this.detail = {...this.detail, faces: []}; // not yet: hide the old boxes
        }
        this.peopleChanged();
      } catch (e) {
        // The faces will turn up on their own; the turn itself was saved.
      }
    },
    // ---------- faces in the viewer ----------
    toggleFaces() {
      this.lightbox.faces = !this.lightbox.faces;
      if (!this.lightbox.faces) this.closeAssign();
      try { localStorage.setItem('showFaces', this.lightbox.faces ? '1' : ''); } catch (e) {}
    },
    // Face boxes are stored as 0..1 of the upright photo, so they scale
    // with the displayed image.
    boxStyle(f) {
      const [x1, y1, x2, y2] = f.box;
      return `left:${x1 * 100}%;top:${y1 * 100}%;width:${(x2 - x1) * 100}%;height:${(y2 - y1) * 100}%`;
    },
    assignStyle() {
      const f = this.assign.face;
      // An object, not a string: a string replaces the whole style
      // attribute, and with it x-show's display:none.
      if (!f) return {};
      // Below the face, or above it when the face is near the bottom.
      const below = f.box[3] < 0.6;
      const x = Math.min(Math.max(f.box[0] * 100, 0), 70) + '%';
      // Zoomed, it is scaled back to normal size from the corner at the face.
      return below ? {left: x, top: `calc(${f.box[3] * 100}% + 30px / var(--z))`, bottom: 'auto', transformOrigin: '0 0'}
        : {left: x, top: 'auto', bottom: `calc(${(1 - f.box[1]) * 100}% + 8px / var(--z))`, transformOrigin: '0 100%'};
    },
    async openAssign(f) {
      this.assign = {face: f, group: null, name: '', busy: false};
      if (!this.peopleData.people.length) this.loadPeople();
      if (f.group) {
        // Suggestions for this face itself (its group may be mixed); the
        // server leaves out anyone already named in the photo.
        const [g, sugg] = await Promise.all([
          api(`/api/groups/${f.group}`).catch(() => null),
          api(`/api/faces/${f.id}/suggestions`).catch(() => null),
        ]);
        if (g && sugg) g.suggestions = sugg;
        if (this.assign.face === f) this.assign.group = g;
      }
      this.$nextTick(() => this.$refs.assignInput && this.$refs.assignInput.focus());
    },
    closeAssign() { this.assign = {face: null, group: null, name: '', busy: false}; },
    // openGroup shows a face's whole group as the first card in Review.
    openGroup(gid) {
      this.closeAssign();
      this.closeLightbox(true);
      this.reviewGroup = gid;
      this.reviewFocus = null;
      this.setView('review');
      this.$nextTick(() => this.$refs.review && this.$refs.review.scrollTo({top: 0}));
    },
    closeGroup() { this.reviewGroup = 0; this.reviewFocus = null; this.writeHash(); },
    // reviewCards is the inbox, with an opened group first.
    reviewCards() {
      const f = this.reviewFocus;
      return f ? [f, ...this.reviewItems.filter(i => i.id !== f.id)] : this.reviewItems;
    },
    // notAFace drops a detection that isn't a person (a cat, a pattern).
    // markPicture says a face is a picture of its person (or of pid, whom
    // it is then tagged as), or takes that back.
    markPicture(on, pid) {
      const f = this.assign.face;
      if (!f) return;
      const local = pid ? {picture: on, person: pid, person_name: this.personName(pid), auto: false} : {picture: on};
      this.faceSave(f, local, () => api(`/api/faces/${f.id}/picture`, {method: 'POST', body: JSON.stringify({on, person_id: pid || 0})}, {retry: true}),
        on ? `Marked as a picture of ${pid ? this.personName(pid) : f.person_name}` : 'No longer marked as a picture');
    },
    // faceGuess is who an unconfirmed face was taken for: filed there
    // automatically, or someone who is another face in the photo too.
    faceGuess(f) {
      if (!f) return null;
      if (f.person && f.auto) return {id: f.person, name: f.person_name};
      if (!f.person && f.twice) return {id: f.twice, name: f.twice_name};
      return null;
    },
    // okStyle puts a face's confirm tick on its box's top right corner. An
    // object, not a string: a string replaces the whole style attribute,
    // and with it x-show's display:none (the tick then shows on every face).
    okStyle(f) {
      const [, y1, x2] = f.box;
      return {left: `${x2 * 100}%`, top: `${y1 * 100}%`};
    },
    // assignCands are the people the open face may be, likeliest first: who
    // it was taken for, then the suggestions.
    assignCands() {
      const f = this.assign.face;
      if (!f) return [];
      const sugg = ((this.assign.group && this.assign.group.suggestions) || []).map(p => ({...p, note: this.sgNote(p)}));
      const g = this.faceGuess(f);
      if (!g) return sugg.filter(p => p.id !== f.person);
      const s = sugg.find(p => p.id === g.id);
      const p = this.peopleData.people.find(p => p.id === g.id) || {};
      const top = {...(s || {id: g.id, name: g.name, cover: p.cover}), note: [f.auto ? 'filed automatically' : 'also in this photo', s && s.note].filter(Boolean).join(' · ')};
      return [top, ...sugg.filter(p => p.id !== g.id)];
    },
    // pickFace says a face (the open one, or f from its tick) is pid.
    async pickFace(pid, f) {
      if (f && this.assign.face !== f) this.assign = {face: f, group: null, name: '', busy: false};
      const face = this.assign.face, g = this.faceGuess(face);
      const name = g && g.id === pid ? g.name : this.personName(pid);
      return this.faceSave(face, {person: pid, person_name: name, auto: false, twice: 0},
        () => api(`/api/faces/${face.id}/tag`, {method: 'POST', body: JSON.stringify({person_id: pid})}, {retry: true}),
        g && g.id === pid ? `Confirmed: ${name}` : `Saved: ${name}`);
    },
    faceLabel(f) {
      if (!f.person && f.twice) return f.twice_name + '?'; // filed under someone who is another face here too
      if (!f.person) return 'Who is this?';
      return f.person_name + (f.auto ? '?' : '') + (f.picture ? ' (picture)' : '');
    },
    // localInput is a date as a datetime-local input value, in local time
    // as dates are shown.
    localInput(t) {
      const d = t ? new Date(t * 1000) : new Date();
      const p = n => String(n).padStart(2, '0');
      return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())}T${p(d.getHours())}:${p(d.getMinutes())}`;
    },
    // saveTaken sets the photo's date from the form, or (reset) goes back
    // to the date in the file.
    async saveTaken(reset) {
      const im = this.current();
      if (!im) return;
      const patch = reset ? {reset_taken: true} : {taken: Math.floor(new Date(this.dateEdit).getTime() / 1000)};
      if (!reset && !Number.isFinite(patch.taken)) return;
      try {
        await api('/api/images/' + im.id, {method: 'PATCH', body: JSON.stringify(patch)}, {retry: true});
        const d = await api('/api/images/' + im.id);
        im.taken = d.taken;
        if (this.detail && this.detail.id === im.id) Object.assign(this.detail, {taken: d.taken, camera_taken: d.camera_taken ?? null});
        this.dateEdit = null;
        this.notify(reset ? 'Back to the file’s date' : 'Date saved — the file itself is unchanged');
      } catch (e) {
        this.notify(`Couldn’t save: ${e.message}`, 6000);
      }
    },
    // ---------- lightbox zoom: wheel zooms at the pointer, drag pans ----------
    resetZoom() { this.zoom = {s: 1, x: 0, y: 0, drag: null, moved: false}; },
    zoomStyle() {
      const z = this.zoom;
      return z.s === 1 ? '' : `transform: translate(${z.x}px, ${z.y}px) scale(${z.s}); --z: ${z.s}`;
    },
    // zoomBox is the photo's untransformed place and size on screen.
    zoomBox() {
      const fr = this.$refs.lbFrame;
      if (!fr) return null;
      const r = fr.getBoundingClientRect(), z = this.zoom;
      return {ox: r.left - z.x, oy: r.top - z.y, w: r.width / z.s, h: r.height / z.s};
    },
    // zoomTo sets the scale, keeping the photo point under (cx, cy) put.
    zoomTo(s, cx, cy) {
      const b = this.zoomBox(), z = this.zoom;
      if (!b) return;
      s = Math.min(8, Math.max(1, s));
      const px = cx - b.ox, py = cy - b.oy;
      z.x = px - (px - z.x) * s / z.s;
      z.y = py - (py - z.y) * s / z.s;
      z.s = s;
      this.clampZoom(b);
    },
    // clampZoom keeps a zoomed photo covering the stage (no empty margins
    // to pan into), and centred along a side that still fits.
    clampZoom(b) {
      const z = this.zoom, st = this.$refs.lbStage.getBoundingClientRect();
      const axis = (t, o, size, lo, hi) => size * z.s <= hi - lo
        ? -(z.s - 1) * size / 2
        : Math.min(lo - o, Math.max(hi - o - size * z.s, t));
      z.x = axis(z.x, b.ox, b.w, st.left, st.right);
      z.y = axis(z.y, b.oy, b.h, st.top, st.bottom);
      if (z.s === 1) z.x = z.y = 0;
    },
    wheelZoom(e) {
      if (e.target.closest('.assign-pop, .lb-nav')) return;
      this.closeAssign();
      const k = Math.exp(-e.deltaY * (e.deltaMode === 1 ? 0.05 : 0.0025));
      this.zoomTo(this.zoom.s * k, e.clientX, e.clientY);
    },
    dblZoom(e) {
      if (e.target.closest('.assign-pop, .lb-nav, .face-box')) return;
      this.zoomTo(this.zoom.s > 1 ? 1 : 2.5, e.clientX, e.clientY);
    },
    panStart(e) {
      if (this.zoom.s === 1 || e.button !== 0 || e.target.closest('.assign-pop, .lb-nav')) return;
      this.zoom.drag = {px: e.clientX, py: e.clientY, x: this.zoom.x, y: this.zoom.y, b: this.zoomBox()};
      this.zoom.moved = false;
    },
    panMove(e) {
      const d = this.zoom.drag;
      if (!d) return;
      const dx = e.clientX - d.px, dy = e.clientY - d.py;
      if (!this.zoom.moved && Math.hypot(dx, dy) < 4) return; // still a click
      if (!this.zoom.moved) {
        this.zoom.moved = true;
        this.$refs.lbStage.setPointerCapture(e.pointerId);
        this.closeAssign();
      }
      this.zoom.x = d.x + dx;
      this.zoom.y = d.y + dy;
      this.clampZoom(d.b);
    },
    panEnd() { this.zoom.drag = null; },
    // hideStranger hides the face's whole group: someone the user doesn't
    // know, in this photo and the group's others. Undo shows them again.
    hideStranger() {
      const f = this.assign.face;
      if (!f || !f.group) return;
      const gid = f.group, n = this.assign.group?.size || 1;
      this.faceSave(f, {bg: true}, () => api(`/api/groups/${gid}/hide`, {method: 'POST'}, {retry: true}));
      this.notify(n > 1 ? `Hidden as a stranger, in all ${this.fmtCount(n)} photos` : 'Hidden as a stranger', 8000,
        {label: 'Undo', fn: () => this.saveBehind(async () => {
          await api(`/api/groups/${gid}/hide?unhide=1`, {method: 'POST'}, {retry: true});
          await this.refreshFaces();
          this.peopleChanged();
        })});
    },
    notAFace() {
      const f = this.assign.face;
      if (!f) return;
      this.faceSave(f, null, () => api(`/api/faces/${f.id}/notface`, {method: 'POST'}, {retry: true}),
        'Removed — it won’t be detected as a person again');
    },
    assignFace(personId) {
      const f = this.assign.face, name = this.assign.name.trim();
      if (!f || (!personId && !name)) return;
      if (personId) return this.pickFace(personId);
      // A new name, or one typed: shown at once, its person known once saved.
      const p = this.peopleData.people.find(p => p.name.toLowerCase() === name.toLowerCase());
      this.faceSave(f, {person: p ? p.id : -1, person_name: p ? p.name : name, auto: false, twice: 0},
        () => api(`/api/faces/${f.id}/tag`, {method: 'POST', body: JSON.stringify({name})}, {retry: true}), `Saved: ${name}`);
    },
    // twiceAct settles a face taken for someone who is another face in the
    // same photo too: 'yes' (this one is them; the other face is then the
    // one in doubt) or 'no' (this one isn't them).
    twiceAct(kind) {
      const f = this.assign.face;
      if (!f || !f.twice) return;
      if (kind === 'yes') return this.pickFace(f.twice);
      this.faceSave(f, {twice: 0}, () => api(`/api/faces/${f.id}/remove`, {method: 'POST'}, {retry: true}));
    },
    unassignFace() {
      const f = this.assign.face;
      if (!f || !f.person) return;
      // Just this face is not them.
      this.faceSave(f, {person: 0, person_name: '', auto: false},
        () => api(`/api/faces/${f.id}/remove`, {method: 'POST'}, {retry: true}), `Not ${f.person_name}`);
    },
    async refreshFaces() {
      const im = this.current();
      if (!im || !this.detail) return;
      touched.ids.add(im.id);
      const faces = await api('/api/images/' + im.id + '/faces').catch(() => null);
      if (faces && this.detail && this.detail.id === im.id) this.detail = {...this.detail, faces};
    },
    toggleFav(im) { if (im) { this.saveEdit({fav: !im.fav}); this.peopleChanged(); } },

    onKey(e) {
      if (e.target.matches('input, textarea, select')) {
        if (e.key === 'Escape') {
          e.target.blur();
          if (this.assign.face) this.closeAssign();
        }
        return;
      }
      if (this.pick.open) {
        if (e.key === 'Escape') this.closePlacePick();
        return;
      }
      if (this.sel.on && !this.lightbox.open && e.key === 'Escape') {
        this.endSelect();
        return;
      }
      // Match tab: Y, N, S answer the first card.
      if (this.view === 'match' && !this.lightbox.open && this.matcher.items[0]) {
        const ans = {y: 'yes', n: 'no', s: 'skip'}[e.key];
        if (!ans) return;
        this.matchAnswer(this.matcher.items[0], ans);
        e.preventDefault();
        return;
      }
      if (this.picView && !this.lightbox.open && e.key === 'Escape') {
        this.picView = null;
        return;
      }
      if (this.eventHide && !this.lightbox.open && e.key === 'Escape') {
        this.closeEvent();
        return;
      }
      if (this.also.person && !this.lightbox.open && e.key === 'Escape') {
        this.closeAlso();
        return;
      }
      if (this.groupView.g && !this.lightbox.open && e.key === 'Escape') {
        this.closeGroupView();
        return;
      }
      if (this.view === 'map' && this.mapReturn && !this.lightbox.open && e.key === 'Escape') {
        this.backFromMap();
        return;
      }
      if (this.lightbox.open) {
        if (e.key === 'Escape') this.assign.face ? this.closeAssign() : this.also.person ? this.closeAlso() : this.closeLightbox();
        else if (e.key === 'ArrowRight') this.step(1);
        else if (e.key === 'ArrowLeft') this.step(-1);
        else if (e.key === 'f') this.toggleFaces();
        else if (e.key === 'r') this.rotateCurrent(90);
        else if (e.key === 'R') this.rotateCurrent(-90);
        else if (e.key === 'h') this.toggleFav(this.current());
        else if (e.key === 'i') this.lightbox.info = !this.lightbox.info;
        else if (e.key === 'Delete') this.askDelete();
        else if (e.key === 't') this.toggleStrip();
        else if (e.key >= '0' && e.key <= '5') this.saveEdit({rating: +e.key});
        else return;
        e.preventDefault();
      } else if (this.view === 'people' && !this.openPersonId && this.$refs.peopleFilter &&
                 (e.key === '/' || e.key.length === 1 && !e.ctrlKey && !e.metaKey && !e.altKey && e.key !== ' ')) {
        // Typing on the People list filters it: the key lands in the box.
        if (e.key === '/') e.preventDefault();
        this.$refs.peopleFilter.focus();
      } else if (e.key === '/') {
        e.preventDefault();
        if (!this.isPhotoView()) this.goNav({id: 'photos'});
        this.$nextTick(() => this.$refs.search.focus());
      }
    },

    // ---------- search autocomplete (people) ----------
    // updateSuggest offers people whose name starts with the word (or two)
    // being typed at the cursor.
    updateSuggest() {
      const el = this.$refs.search, v = el.value, end = el.selectionStart ?? v.length;
      const before = v.slice(0, end);
      const people = [...this.specialTokens(), ...this.peopleData.people];
      const found = [];
      // Try the last two words first, so "Anna Ma" completes "Anna Maria".
      for (const n of [2, 1]) {
        const m = before.match(new RegExp('(?:^|\\s)((?:\\S+\\s+){' + (n - 1) + '}\\S+)$'));
        if (!m) continue;
        const tok = m[1].toLowerCase(), from = end - m[1].length;
        for (const p of people) {
          const name = p.name.toLowerCase();
          if (name === tok || found.some(f => f.p.id === p.id) || this.tokens.some(t => t.id === p.id)) continue;
          const words = p.special ? [name.slice(1)] : name.split(/\s+/);
          if (name.startsWith(tok) || (n === 1 && (!p.special || tok.length >= 3) && words.some(w => w.startsWith(tok)))) found.push({p, from});
        }
        if (found.length) break;
      }
      found.sort((a, b) => b.p.photos - a.p.photos);
      this.suggest.items = found.slice(0, 6).map(f => f.p);
      this.suggest.from = found.length ? found[0].from : 0;
      this.suggest.to = end;
      this.suggest.i = 0;
    },
    suggestKey(e) {
      const s = this.suggest, el = this.$refs.search;
      // Backspace at the start of the text removes the last person chip whole.
      if (e.key === 'Backspace' && this.tokens.length && el.selectionStart === 0 && el.selectionEnd === 0) {
        e.preventDefault();
        this.removeToken(this.tokens[this.tokens.length - 1]);
        return;
      }
      if (!s.items.length) return;
      if (e.key === 'ArrowDown') s.i = (s.i + 1) % s.items.length;
      else if (e.key === 'ArrowUp') s.i = (s.i + s.items.length - 1) % s.items.length;
      else if (e.key === 'Enter' || e.key === 'Tab') this.pickSuggest(s.items[s.i]);
      else if (e.key === 'Escape') s.items = [];
      else return;
      e.preventDefault();
    },
    // pickSuggest turns the typed fragment into a person chip.
    pickSuggest(p) {
      const el = this.$refs.search, s = this.suggest;
      const v = (el.value.slice(0, s.from) + el.value.slice(s.to)).replace(/\s{2,}/g, ' ').replace(/^\s+/, '');
      el.value = v;
      this.q = v;
      if (!this.tokens.some(t => t.id === p.id)) this.tokens.push({id: p.id, name: p.name, special: !!p.special});
      s.items = [];
      const caret = Math.min(s.from, v.length);
      this.$nextTick(() => { el.focus(); el.setSelectionRange(caret, caret); });
      this.reload();
    },
    removeToken(t) {
      this.tokens = this.tokens.filter(x => x.id !== t.id);
      this.$refs.search.focus();
      this.reload();
    },
    tokenIDs() { return this.tokens.map(t => t.id).join(','); },
    // faceTokens are the face filters: $known (confirmed or not), $confirmed,
    // $unconfirmed and $unknown, and one(…)/many(…) of each.
    faceTokens() {
      const n = this.peopleData.face_photos || {}, out = [];
      for (const c of ['', 'one', 'many']) for (const k of ['known', 'unknown', 'confirmed', 'unconfirmed']) {
        const id = c ? c + '-' + k : k;
        out.push({id, name: c ? c + '($' + k + ')' : '$' + k, special: true, hint: this.specialLabel(id), photos: n[id] || 0});
      }
      return out;
    },
    // specialLabel says what a search filter finds ("with one person not named yet").
    specialLabel(id) {
      const [c, k] = id.includes('-') ? id.split('-') : ['', id];
      const what = {known: 'named', unknown: 'not named yet', confirmed: 'you’ve confirmed', unconfirmed: 'filed but not confirmed'}[k];
      if (what) return 'with ' + ({'': 'someone', one: 'exactly one person', many: 'two or more people'}[c]) + ' ' + what;
      return {alone: 'one person only', nsfw: 'not safe for work', blurry: 'blurry', picture: 'a picture of someone'}[id] || id;
    },
    // specialTokens are the search filters offered next to people.
    specialTokens() {
      return [
        ...this.faceTokens(),
        {id: 'alone', name: '$alone', special: true, hint: 'only one person in the photo', photos: this.peopleData.alone_photos || 0},
        {id: 'nsfw', name: '$nsfw', special: true, hint: 'photos marked not safe for work', photos: (this.jobs && this.jobs.nsfw_marked) || 0},
        {id: 'blurry', name: '$blurry', special: true, hint: 'photos that look blurry', photos: (this.jobs && this.jobs.blurry) || 0},
        {id: 'picture', name: '$picture', special: true, hint: 'a picture of someone in it (on a wall, in a frame); with a name: pictures of them', photos: this.peopleData.picture_photos || 0},
      ];
    },

    // ---------- settings ----------
    async loadSettings() {
      const st = await api('/api/settings').catch(() => null);
      if (!st) return;
      this.settings = st;
      this.resetForm();
    },
    resetForm() {
      // Unset (zero) numbers show as empty, so the field shows the value in use.
      const f = {...(this.settings ? this.settings.saved : {})};
      for (const k of ['embed_workers', 'face_gpu_mem_mb', 'workers', 'conns', 'rescan_minutes', 'review_min']) if (!f[k]) f[k] = '';
      this.form = f;
      this.testResult = null;
    },
    async saveSettings() {
      if (this.savingSettings) return;
      this.savingSettings = true;
      try {
        // Empty number fields mean "default".
        const s = {...this.form};
        for (const k of Object.keys(s)) if (s[k] === '' || s[k] === null || Number.isNaN(s[k])) delete s[k];
        this.settings = await api('/api/settings', {method: 'PUT', body: JSON.stringify(s)}, {retry: true});
        this.resetForm();
        this.notify(this.settings.restart_needed ? 'Saved — some changes need a restart' : 'Saved');
        this.reload(); // showing NSFW photos, or not, changes what's listed
        this.peopleChanged(); // the Review count and the issues badge
        if (this.map) { this.map.remove(); this.map = this.mapLayer = this.mapFocusMarker = null; } // new tiles next time
      } catch (e) {
        this.notify(`Couldn’t save: ${e.message}`, 6000);
      } finally {
        this.savingSettings = false;
      }
    },
    async testEmbed() {
      const url = (this.form.embed_url || this.settings.effective.embed_url || '').trim();
      if (!url || url === 'off') { this.testResult = {ok: false, error: 'No server set'}; return; }
      this.testing = true;
      this.testResult = null;
      try {
        this.testResult = await api('/api/settings/test-embed', {method: 'POST', body: JSON.stringify({url})});
      } catch (e) {
        this.testResult = {ok: false, error: e.message};
      } finally {
        this.testing = false;
      }
    },
    testText() {
      const r = this.testResult;
      if (!r) return '';
      if (r.ok) return `✓ Connected — ${r.model || 'model'} with image support`;
      if (r.model && !r.vision) return `✗ ${r.model} has no image support — start llama-server with --mmproj`;
      return `✗ ${r.error || 'No answer'}`;
    },
    async restartServer() {
      this.restarting = true;
      try {
        await api('/api/restart', {method: 'POST'});
        this.notify('Restarting…', 4000);
        // The page reconnects by itself; then reload the settings.
        for (let i = 0; i < 40; i++) {
          await new Promise(r => setTimeout(r, 1500));
          const st = await fetch('/api/settings').then(r => r.ok ? r.json() : null).catch(() => null);
          if (st && !st.restart_needed) { this.settings = st; this.resetForm(); this.refreshStats(); break; }
        }
      } finally {
        this.restarting = false;
      }
    },

    // ---------- people (directory) ----------
    async loadPeople() {
      this.peopleData = await api('/api/people');
      if (this.openPersonId) this.loadPerson();
    },
    faceURL(key) { return '/face/' + key; },
    // levelURL is a face crop with its light evened out, for comparing.
    levelURL(key) { return key ? '/face/' + key + (key.includes('?') ? '&' : '?') + 'level' : ''; },
    openPerson(p) {
      this.openPersonId = p.id;
      this.groupSel = {};
      this.moveTo = '';
      this.personGroups = [];
      this.personFaces = [];
      this.personFacesShow = 240;
      this.faceCands = newFaceCands(p.id);
      this.doubts = {pid: p.id, items: [], show: 30};
      if (this.view !== 'people') this.setView('people');
      this.writeHash();
      this.loadPerson();
      this.$nextTick(() => this.$refs.people && this.$refs.people.scrollTo({top: 0}));
    },
    async loadPerson() {
      const id = this.openPersonId;
      const faces = await api(`/api/people/${id}/faces`).catch(() => []);
      if (id === this.openPersonId) this.personFaces = faces;
      this.loadFaceCands();
      this.loadDoubts();
      this.loadPictures(id);
    },
    // loadDoubts fetches the open person's faces, the least likely them
    // first: "Is this really …?".
    async loadDoubts() {
      const pid = this.openPersonId;
      const items = await api(`/api/people/${pid}/doubts?limit=120`).catch(() => null);
      if (items && this.doubts.pid === pid) this.doubts.items = items;
    },
    // doubtAnswer: yes, it's them (looked at alone: not doubted again), or
    // no, it isn't (taken out, and never filed there again).
    doubtAnswer(d, yes) {
      const pid = this.doubts.pid, name = d.name;
      this.doubts.items = this.doubts.items.filter(x => x !== d);
      this.peek = null;
      touched.ids.add(d.face.image);
      this.notify(yes ? `Confirmed: ${name}` : `Not ${name}`);
      this.saveBehind(async () => {
        await api(`/api/faces/${d.face.id}/${yes ? 'tag' : 'reject'}`, {method: 'POST', body: JSON.stringify({person_id: pid})}, {retry: true});
        this.peopleChanged();
      });
    },
    // loadFaceCands fetches the faces that may be the open person, one by
    // one: filed automatically and not confirmed, or alike in unnamed groups.
    async loadFaceCands() {
      const pid = this.openPersonId;
      if (!pid) return;
      if (this.faceCands.pid !== pid) this.faceCands = newFaceCands(pid);
      this.faceCands.loading = true;
      const items = await api(`/api/people/${pid}/face-candidates?limit=120`).catch(() => null);
      if (this.faceCands.pid !== pid) return;
      this.faceCands.loading = false;
      if (items) this.faceCands.items = items.filter(f => !this.faceCands.answered.has(f.id));
    },
    // faceAnswer says whether one face is the open person. Like the "Is
    // this also" panel, the face goes at once and answers are saved in the
    // background in click order; then the list and groups are refreshed.
    faceAnswer(f, yes) {
      const fc = this.faceCands, pid = fc.pid, name = this.openedPerson()?.name || 'them';
      fc.answered.add(f.id);
      fc.items = fc.items.filter(x => x.id !== f.id);
      this.peek = null;
      touched.ids.add(f.image);
      alsoPending++;
      alsoQueue = alsoQueue.then(async () => {
        try {
          await api(`/api/faces/${f.id}/${yes ? 'tag' : 'reject'}`, {method: 'POST', body: JSON.stringify({person_id: pid})}, {retry: true});
        } catch (e) {
          this.notify(`Couldn’t save the answer for ${name}: ${e.message}`, 6000);
        }
      });
      alsoQueue.then(() => this.faceAnswered(pid));
    },
    async faceAnswered(pid) {
      if (--alsoPending > 0) return;
      this.peopleChanged();
      this.loadPeople();
      if (this.openPersonId === pid) this.loadPerson(); // groups, and the next faces
    },
    closePerson() { this.openPersonId = 0; this.writeHash(); this.loadPeople(); },
    // ---------- copy a person's photos out ----------
    async openExport() {
      const ef = this.exportForm;
      let base = '';
      try { base = localStorage.getItem('exportBase') || ''; ef.byPlace = localStorage.getItem('exportByPlace') || ''; } catch (e) {}
      const name = this.openedPerson()?.name || '';
      // Suggest the last folder's parent with this person's name.
      if (base) ef.dir = base.replace(/[\\/]+$/, '') + (base.includes('\\') ? '\\' : '/') + name;
      ef.open = true;
      ef.status = await api('/api/export').catch(() => null);
      if (ef.status && ef.status.running) this.pollExport();
    },
    async startExport() {
      const ef = this.exportForm, pid = this.openPersonId;
      if (ef.busy || !ef.dir.trim()) return;
      ef.busy = true;
      try {
        ef.status = await api(`/api/people/${pid}/export`, {method: 'POST', body: JSON.stringify({dir: ef.dir, confirmed_only: ef.confirmedOnly, by_place: ef.byPlace})});
        try { localStorage.setItem('exportByPlace', ef.byPlace); } catch (e) {}
        try {
          const d = ef.status.dir, cut = Math.max(d.lastIndexOf('\\'), d.lastIndexOf('/'));
          localStorage.setItem('exportBase', cut > 0 ? d.slice(0, cut) : d);
        } catch (e) {}
        this.pollExport();
      } catch (e) {
        this.notify(`Couldn’t copy: ${e.message}`, 7000);
      } finally {
        ef.busy = false;
      }
    },
    async pollExport() {
      const ef = this.exportForm;
      clearTimeout(this._exportTimer);
      ef.status = await api('/api/export').catch(() => ef.status);
      if (ef.status && ef.status.running) {
        this._exportTimer = setTimeout(() => this.pollExport(), 1000);
      } else if (ef.status && ef.status.finished_at && !ef.status.tidy) {
        const st = ef.status;
        this.notify(`Copied ${fmtCount(st.copied)} photos of ${st.person}` + (st.skipped ? `, ${fmtCount(st.skipped)} were already there` : '') + (st.failed ? `, ${fmtCount(st.failed)} failed` : ''), 8000);
      }
    },
    cancelExport() { api('/api/export/cancel', {method: 'POST'}).catch(() => {}); },
    // ---------- the whole library into one tidy folder ----------
    lang() { return (window.i18n && window.i18n.lang()) || 'en'; },
    async loadTidy() {
      const t = this.tidy;
      try { t.dir = t.dir || localStorage.getItem('tidyDir') || ''; t.from = t.from || localStorage.getItem('fetchFrom') || ''; } catch (e) {}
      const [pre, st] = await Promise.all([
        api('/api/tidy', {method: 'POST', body: JSON.stringify({dry: true, lang: this.lang()})}).catch(() => null),
        api('/api/export').catch(() => null),
      ]);
      t.preview = pre;
      t.status = st;
      // A library of one tidy folder: that's where new photos go.
      if (!t.dir && this.folders.length === 1) t.dir = this.folders[0].location;
      if (st && st.tidy && st.running) this.pollTidy();
    },
    // tidyExample shows where a photo goes, with the folder typed.
    // tidyBase is the example folder shown before one is typed.
    tidyBase() {
      const win = navigator.platform.startsWith('Win'), name = this.lang() === 'da' ? 'Billeder ryddet' : 'Tidy photos';
      return win ? 'E:\\' + name : '/home/you/' + name;
    },
    tidyExample() {
      const p = this.tidy.preview, top = p && p.sources[0];
      const sep = navigator.platform.startsWith('Win') ? '\\' : '/';
      const base = (this.tidy.dir.trim() || this.tidyBase()).replace(/[\\/]+$/, '');
      const month = new Date(2011, 4, 1).toLocaleString(this.lang() === 'da' ? 'da-DK' : 'en-GB', {month: 'long'});
      return [base, '2011', '05 ' + month[0].toUpperCase() + month.slice(1), top ? top.name : 'Canon', 'IMG_0037.JPG'].join(sep);
    },
    async startTidy() {
      const t = this.tidy;
      if (t.busy || !t.dir.trim()) return;
      t.busy = true;
      try {
        t.status = await api('/api/tidy', {method: 'POST', body: JSON.stringify({dir: t.dir, lang: this.lang()})});
        try { localStorage.setItem('tidyDir', t.status.dir); } catch (e) {}
        this.pollTidy();
      } catch (e) {
        this.notify(`Couldn’t copy: ${e.message}`, 8000);
      } finally {
        t.busy = false;
      }
    },
    async startFetch() {
      const t = this.tidy;
      if (t.busy || !t.from.trim() || !t.dir.trim()) return;
      t.busy = true;
      try {
        t.status = await api('/api/tidy/fetch', {method: 'POST', body: JSON.stringify({from: t.from, dir: t.dir, lang: this.lang()})});
        try { localStorage.setItem('fetchFrom', t.status.from); localStorage.setItem('tidyDir', t.status.dir); } catch (e) {}
        this.pollTidy();
      } catch (e) {
        this.notify(`Couldn’t fetch: ${e.message}`, 8000);
      } finally {
        t.busy = false;
      }
    },
    async pollTidy() {
      const t = this.tidy;
      clearTimeout(this._tidyTimer);
      t.status = await api('/api/export').catch(() => t.status);
      if (t.status && t.status.running) {
        this._tidyTimer = setTimeout(() => this.pollTidy(), 1000);
      } else if (t.status && t.status.fetch && t.status.finished_at) {
        const st = t.status;
        this.notify((st.copied === 1 ? 'Fetched 1 new photo' : `Fetched ${fmtCount(st.copied)} new photos`) + (st.failed ? `, ${fmtCount(st.failed)} failed` : ''), 8000);
        this.refreshStats().catch(() => {});
      } else if (t.status && t.status.tidy && t.status.finished_at) {
        const st = t.status;
        this.notify(`Copied ${fmtCount(st.copied)} photos` + (st.skipped ? `, ${fmtCount(st.skipped)} were already there` : '') + (st.failed ? `, ${fmtCount(st.failed)} failed` : ''), 8000);
      }
    },
    openedPerson() { return this.peopleData.people.find(p => p.id === this.openPersonId) || null; },
    showPersonPhotos(p) {
      this.person = p.id;
      this.personName = p.name;
      this.q = '';
      this.tokens = [];
      this.fav = false;
      this.setView('photos');
      this.reload();
    },
    clearPerson() { this.person = 0; this.reload(); },
    personLabel() { const p = this.peopleData.people.find(p => p.id === this.person); return p ? p.name : this.personName || '…'; },
    // confirmGroup turns an automatically filed group into a confirmed one.
    async confirmGroup(g) {
      const ok = await this.guard(g, () => api(`/api/groups/${g.id}/name`, {method: 'POST', body: JSON.stringify({person_id: this.openPersonId, shown: shownIDs(g)})}, {retry: true}));
      if (ok) g.auto = false;
      this.peopleChanged();
    },
    async removeGroup(g) {
      const pid = this.openPersonId;
      await this.guard(g, () => api(`/api/groups/${g.id}/reject`, {method: 'POST', body: JSON.stringify({person_id: pid})}, {retry: true}));
      this.personGroups = this.personGroups.filter(x => x.id !== g.id);
      this.loadPeople();
      this.peopleChanged();
    },
    selectedGroups() { return Object.keys(this.groupSel).map(Number); },
    // moveGroups gives the selected groups to another person, new or
    // existing by name: how one person who is really two is split.
    async moveGroups() {
      const name = this.moveTo.trim(), from = this.openedPerson();
      const ids = this.selectedGroups();
      if (!name || !ids.length || this.moving || !from) return;
      if (name.toLowerCase() === from.name.toLowerCase()) {
        this.notify(`They're already ${from.name}. Give the other person a different name, e.g. “${from.name} H.”`, 6000);
        return;
      }
      this.moving = true;
      try {
        for (const id of ids) {
          await api(`/api/groups/${id}/name`, {method: 'POST', body: JSON.stringify({name})}, {retry: true});
        }
        this.notify(`Moved ${ids.length} ${ids.length === 1 ? 'group' : 'groups'} to ${name}`);
        this.groupSel = {};
        this.moveTo = '';
        await this.loadPeople();
      } catch (e) {
        this.notify(`Couldn’t move: ${e.message}`, 6000);
      } finally {
        this.moving = false;
        this.peopleChanged();
      }
    },
    async renamePerson(el) {
      const name = el.value.trim(), p = this.openedPerson();
      if (!p || !name || name === p.name) { if (p) el.value = p.name; return; }
      try {
        await this.updatePerson({name});
        this.notify(`Renamed to ${name}`);
      } catch (e) {
        el.value = p.name;
        this.notify(`Couldn’t rename: ${e.message}`, 5000);
      }
    },
    async openGroupView(g) {
      this.groupView = {g, faces: [], loading: true};
      const faces = await api(`/api/groups/${g.id}/faces`).catch(() => []);
      if (this.groupView.g === g) this.groupView = {g, faces, loading: false};
    },
    closeGroupView() { this.groupView = {g: null, faces: [], loading: false}; this.peek = null; },
    isCover(f) { const p = this.openedPerson(); return !!p && p.cover === f.crop; },
    async setCover(f) {
      try {
        await this.updatePerson({cover_face: f.id});
        this.notify('Picture updated');
      } catch (e) {
        this.notify(`Couldn’t set picture: ${e.message}`, 5000);
      }
    },
    async takeOutFace(f) {
      const g = this.groupView.g;
      try {
        await api(`/api/faces/${f.id}/remove`, {method: 'POST'}, {retry: true});
        this.groupView.faces = this.groupView.faces.filter(x => x.id !== f.id);
        this.personFaces = this.personFaces.filter(x => x.id !== f.id);
        if (g) { g.size--; g.faces = g.faces.filter(x => x.id !== f.id); }
        this.peek = null;
        const pid = this.openPersonId, name = (this.openedPerson() || {}).name;
        this.notify(`Taken out of ${name}`, 8000, {label: 'Undo', fn: () => this.undoTakeOut(f, pid)});
        this.loadPeople();
      } catch (e) {
        this.notify(`Couldn’t remove: ${e.message}`, 5000);
      }
      this.peopleChanged();
    },
    // notPicture says a face marked as a picture of the open person is
    // them for real after all.
    async notPicture(f) {
      try {
        await api(`/api/faces/${f.id}/picture`, {method: 'POST', body: JSON.stringify({on: false})}, {retry: true});
        f.picture = false;
        this.notify('Not a picture: them, for real');
        this.peopleChanged();
      } catch (e) {
        this.notify(`Couldn’t save: ${e.message}`, 5000);
      }
    },
    // searchPictures shows the photos with a picture of someone.
    searchPictures(p) {
      this.tokens = [{id: p.id, name: p.name}, this.specialTokens().find(t => t.id === 'picture')];
      this.q = '';
      this.setView('photos');
      this.reload();
    },
    // undoTakeOut puts a face that was taken out back as the person's (as
    // a face tagged as them).
    async undoTakeOut(f, pid) {
      try {
        await api(`/api/faces/${f.id}/tag`, {method: 'POST', body: JSON.stringify({person_id: pid})}, {retry: true});
        if (this.groupView.g && !this.groupView.faces.some(x => x.id === f.id)) this.groupView.faces.push(f);
        if (this.openPersonId === pid) this.loadPerson();
        this.notify('Put back');
        this.loadPeople();
      } catch (e) {
        this.notify(`Couldn’t put it back: ${e.message}`, 5000);
      }
      this.peopleChanged();
    },
    async updatePerson(patch) {
      await api('/api/people/' + this.openPersonId, {method: 'PATCH', body: JSON.stringify(patch)}, {retry: true});
      this.peopleData = await api('/api/people');
    },
    async mergePerson() {
      const into = +this.mergeInto;
      const from = this.openedPerson();
      const target = this.peopleData.people.find(p => p.id === into);
      if (!from || !target) return;
      if (!confirm(`Merge “${from.name}” into “${target.name}”? All of ${from.name}'s groups will be ${target.name}'s.`)) return;
      await api(`/api/people/${from.id}/merge`, {method: 'POST', body: JSON.stringify({into})}, {retry: true});
      this.mergeInto = '';
      this.notify(`Merged into ${target.name}`);
      this.peopleData = await api('/api/people');
      this.openPerson({id: into});
      this.peopleChanged();
    },

    // ---------- review (inbox) ----------
    // ---------- file all sure matches at once ----------
    bulkMin() { return Math.min(100, Math.max(50, Math.round(+this.bulk.min || 75))) / 100; },
    async loadBulk() {
      const b = this.bulk, min = this.bulkMin();
      try { localStorage.setItem('bulkMin', String(Math.round(min * 100))); } catch (e) {}
      b.loading = true;
      const res = await api(`/api/review/confident?min=${min}`).catch(() => null);
      if (this.bulkMin() === min) b.preview = res;
      b.loading = false;
    },
    onBulkMin() { clearTimeout(this._bulkTimer); this._bulkTimer = setTimeout(() => this.loadBulk(), 300); },
    async bulkAccept() {
      const b = this.bulk;
      if (b.busy || !b.preview || !b.preview.groups) return;
      b.busy = true;
      try {
        const res = await api('/api/review/confident', {method: 'POST', body: JSON.stringify({min: this.bulkMin()})});
        const f = res.filed;
        this.notify(`Filed ${fmtCount(f.groups)} ${f.groups === 1 ? 'group' : 'groups'} (${fmtCount(f.faces)} faces): ` + f.people.slice(0, 5).map(p => p.name).join(', ') + (f.people.length > 5 ? '…' : ''), 8000);
        touched.all = true;
        this.loadPeople();
        this.peopleChanged();
        await this.loadReview(true);
      } catch (e) {
        this.notify(`Couldn’t file them: ${e.message}`, 7000);
      } finally {
        b.busy = false;
        this.loadBulk();
      }
    },
    // sureSuggestion is a card's suggestion at least bulk.min alike, when
    // only one is: what GO would file it as.
    sureSuggestion(it) {
      if (!it || it.auto || !it.suggestions) return null;
      const min = this.bulkMin(), sure = it.suggestions.filter(p => p.score >= min && !p.off);
      return sure.length === 1 && it.suggestions.filter(p => p.score >= min).length === 1 ? sure[0] : null;
    },
    async loadReview(reset) {
      if (this.reviewLoading) return;
      this.reviewLoading = true;
      if (reset) this.onBulkMin(); // the counts change as groups are named
      try {
        const offset = reset ? 0 : this.reviewItems.length;
        if (reset && this.reviewGroup) {
          const gid = this.reviewGroup;
          const g = await api(`/api/groups/${gid}`).catch(() => null);
          if (gid === this.reviewGroup) this.reviewFocus = g;
        }
        const res = await api(`/api/review?offset=${offset}&limit=40&order=${this.reviewOrder}`);
        this.reviewItems = reset ? res.items : this.reviewItems.concat(res.items.filter(i => !this.reviewItems.some(x => x.id === i.id)));
        this.reviewTotal = res.total;
        this.reviewFaces = res.faces || 0;
      } finally {
        this.reviewLoading = false;
      }
    },
    // bornYear is the known birth year, else the estimate (0: unknown).
    bornYear(p) { return p.birth_year || (p.est_birth ? Math.round(p.est_birth) : 0); },
    sortedPeople() {
      // The quick filter: every word typed starts a word of the name, or is
      // inside it; case and accents don't matter.
      const fold = t => t.normalize('NFD').replace(/[\u0300-\u036f]/g, '').replace(/ø/g, 'o').replace(/æ/g, 'ae').replace(/å/g, 'a').toLowerCase();
      const words = fold(this.peopleFilter).split(/\s+/).filter(Boolean);
      const ps = this.peopleData.people.filter(p => { const n = fold(p.name); return words.every(w => n.includes(w)); });
      const byName = (a, b) => a.name.localeCompare(b.name);
      const born = p => this.bornYear(p) || null;
      const byBorn = dir => (a, b) => {
        const x = born(a), y = born(b);
        if (x === null || y === null) return (x === null) - (y === null) || byName(a, b); // unknown last
        return dir * (x - y) || byName(a, b);
      };
      switch (this.peopleOrder) {
        case 'most': return ps.sort((a, b) => b.photos - a.photos || byName(a, b));
        case 'fewest': return ps.sort((a, b) => a.photos - b.photos || byName(a, b));
        case 'oldest': return ps.sort(byBorn(1));
        case 'youngest': return ps.sort(byBorn(-1));
        default: return ps.sort(byName);
      }
    },
    savePeopleOrder() { try { localStorage.setItem('peopleOrder', this.peopleOrder); } catch (e) {} },
    personName(id) { const p = this.peopleData.people.find(p => p.id === id); return p ? p.name : '…'; },
    setReviewOrder() {
      try { localStorage.setItem('reviewOrder', this.reviewOrder); } catch (e) {}
      this.loadReview(true);
      this.$refs.review && this.$refs.review.scrollTo({top: 0});
    },
    onReviewScroll() {
      const el = this.$refs.review;
      if (el.scrollTop + el.clientHeight > el.scrollHeight - 800 && this.reviewItems.length < this.reviewTotal) this.loadReview(false);
    },
    // reviewAct handles one decision on an inbox card: a suggested person,
    // a typed name, skip or hide. The card leaves the inbox on success.
    async reviewAct(it, action, personId) {
      const name = (this.names[it.id] || '').trim();
      if (action === 'name' && !name) return;
      let named = null; // the person, and their next likeliest groups
      let gone = null;  // "not a person": the faces, for an undo
      const ok = await this.guard(it, () => {
        switch (action) {
          case 'person': return api(`/api/groups/${it.id}/name`, {method: 'POST', body: JSON.stringify({person_id: personId, shown: shownIDs(it)})}, {retry: true}).then(r => { named = r; });
          case 'name': return api(`/api/groups/${it.id}/name`, {method: 'POST', body: JSON.stringify({name, shown: shownIDs(it)})}, {retry: true}).then(r => { named = r; });
          case 'skip': return api(`/api/groups/${it.id}/skip`, {method: 'POST'}, {retry: true});
          case 'hide': return api(`/api/groups/${it.id}/hide`, {method: 'POST'}, {retry: true});
          case 'notface': return api(`/api/groups/${it.id}/notface`, {method: 'POST'}, {retry: true}).then(r => { gone = r.faces; });
          case 'reject': return api(`/api/groups/${it.id}/reject`, {method: 'POST', body: JSON.stringify({person_id: it.person})}, {retry: true});
        }
      });
      if (!ok) return;
      if (action === 'reject') {
        // Not who it was filed as: it stays here as an unknown group.
        this.notify(`Not ${this.personName(it.person)} — noted`);
        it.auto = false;
        it.person = 0;
        return;
      }
      delete this.names[it.id];
      this.saveDrafts();
      this.reviewItems = this.reviewItems.filter(x => x.id !== it.id);
      if (this.reviewFocus && this.reviewFocus.id === it.id) this.closeGroup();
      this.reviewTotal = Math.max(0, this.reviewTotal - 1);
      this.reviewFaces = Math.max(0, this.reviewFaces - (it.size || 0));
      if (this.jobs) this.jobs.review = this.reviewTotal; // the sidebar badge, right away
      this.peopleChanged();
      if (action === 'person' || action === 'name') {
        this.notify(action === 'name' ? `Saved: ${name}` : 'Saved');
        // New names and groups change suggestions for the rest.
        this.peopleData = await api('/api/people');
        this.loadReview(true);
        // Then ask about their likeliest other groups, as in the viewer.
        if (named && named.candidates && named.candidates.length) {
          this.also = newAlso({id: named.person_id, name: action === 'name' ? name : this.personName(named.person_id)}, named.candidates);
        }
      }
      if (action === 'hide' || action === 'notface') {
        const n = this.fmtCount(it.size || 1);
        const undo = action === 'hide'
          ? () => api(`/api/groups/${it.id}/hide?unhide=1`, {method: 'POST'}, {retry: true})
          : () => api(`/api/groups/${it.id}/restore`, {method: 'POST', body: JSON.stringify({faces: gone || []})}, {retry: true});
        this.notify(action === 'hide' ? `Hidden as a stranger, in all ${n} photos` : `Not a person: ${n} ${it.size === 1 ? 'face' : 'faces'} taken out`, 8000,
          {label: 'Undo', fn: async () => {
            await undo().catch(e => this.notify(`Couldn’t undo: ${e.message}`, 6000));
            this.peopleChanged();
            this.loadReview(true);
          }});
      }
      if (this.reviewItems.length < 12 && this.reviewTotal > this.reviewItems.length) this.loadReview(false);
    },
    // guard runs a request for a card, marks it busy and reports failure
    // without losing anything typed.
    async guard(it, fn) {
      if (this.busyGroup[it.id]) return false;
      this.busyGroup[it.id] = true;
      try {
        await fn();
        return true;
      } catch (e) {
        this.notify(`Couldn’t save: ${e.message}. Nothing was lost — try again.`, 6000);
        return false;
      } finally {
        delete this.busyGroup[it.id];
      }
    },
    birthLabel(y) { return y ? 'born ~' + Math.round(y) : ''; },
    // Always a real boolean: Alpine keeps a disabled attribute when a
    // binding yields undefined, which left Save permanently disabled.
    isBusy(g) { return !!this.busyGroup[g.id]; },
    // Names typed but not yet saved survive reloads.
    saveDrafts() { try { sessionStorage.setItem('nameDrafts', JSON.stringify(this.names)); } catch (e) {} },
    loadDrafts() { try { Object.assign(this.names, JSON.parse(sessionStorage.getItem('nameDrafts') || '{}')); } catch (e) {} },

    // ---------- folders ----------
    async loadFolders() { this.folders = await api('/api/folders'); },
    // addLocation builds the folder location from the add form.
    addLocation() {
      const a = this.add;
      if (a.kind === 'file') return a.path.trim();
      const user = a.user.trim() ? encodeURIComponent(a.user.trim()) + '@' : '';
      const path = a.path.trim().replace(/^\/*/, '/').split('/').map(encodeURIComponent).join('/');
      return `smb://${user}${a.host.trim()}${path === '/' ? '' : path}`;
    },
    async addFolder() {
      this.folderError = '';
      this.add.busy = true;
      try {
        await api('/api/folders', {method: 'POST', body: JSON.stringify({location: this.addLocation(), password: this.add.password})});
        Object.assign(this.add, {path: '', password: ''});
        this.loadFolders();
        this.notify('Folder added — scanning');
      } catch (e) {
        this.folderError = e.message;
      } finally {
        this.add.busy = false;
      }
    },
    async removeFolder(f) {
      if (!confirm(`Stop tracking ${f.location}?\n\nFiles are not touched, and ratings/notes are kept if the photos show up again.`)) return;
      await api('/api/folders/' + f.id, {method: 'DELETE'});
      this.loadFolders();
      this.refreshStats();
      this.reload();
    },
    async rescan() { await api('/api/scan', {method: 'POST'}); this.notify('Rescan started'); },
    openBrowser() {
      this.browser.open = true;
      const loc = this.addLocation();
      this.browse(this.add.kind === 'file' ? (loc || '/') : loc);
    },
    async browse(location) {
      Object.assign(this.browser, {loading: true, error: '', dirs: []});
      try {
        const r = await api('/api/browse', {method: 'POST', body: JSON.stringify({location, password: this.add.password})});
        Object.assign(this.browser, r);
      } catch (e) {
        this.browser.location = location;
        this.browser.error = e.message;
      } finally {
        this.browser.loading = false;
      }
    },
    childLocation(loc, name) {
      if (loc.startsWith('/')) return (loc.endsWith('/') ? loc : loc + '/') + name;
      return loc.replace(/\/$/, '') + '/' + encodeURIComponent(name);
    },
    // SMB servers list shares at the top level; a share root is the minimum.
    canChoose() {
      const l = this.browser.location;
      if (!l || this.browser.error) return false;
      if (l.startsWith('/')) return true;
      try { return new URL(l).pathname.replace(/\/$/, '') !== ''; } catch (e) { return false; }
    },
    chooseBrowsed() {
      const l = this.browser.location;
      if (l.startsWith('/')) this.add.path = l;
      else this.add.path = decodeURIComponent(new URL(l).pathname);
      this.browser.open = false;
    },

    // ---------- live connection ----------
    // One SSE stream. A missing heartbeat or an error flips the UI to offline
    // and reconnects with backoff. On every (re)connect the server says hello
    // with its asset hashes: CSS changes are hot-swapped in place, anything
    // else reloads the page (state survives via the URL), and a new Go build
    // with unchanged assets just refreshes data.
    connect() {
      clearTimeout(this.conn.timer);
      if (this.conn.es) this.conn.es.close();
      const es = new EventSource('/api/events');
      this.conn.es = es;
      const seen = () => { this.conn.lastMsg = Date.now(); this.conn.retry = 500; };
      es.addEventListener('hello', e => { seen(); this.onHello(JSON.parse(e.data)); });
      es.addEventListener('scan', e => { seen(); this.onScan(JSON.parse(e.data)); });
      es.addEventListener('learn', e => { seen(); this.onLearn(JSON.parse(e.data)); });
      es.addEventListener('ping', seen);
      es.onerror = () => {
        if (this.conn.online) console.info('[live] connection lost');
        this.conn.online = false;
        es.close();
        this.conn.timer = setTimeout(() => this.connect(), this.conn.retry);
        this.conn.retry = Math.min(5000, this.conn.retry * 1.6);
      };
      if (!this.statsTimer) {
        // Indexing and face progress aren't pushed; poll gently while they run.
        this.statsTimer = setInterval(() => {
          if (!this.conn.online || !this.busy() && !this.facesLeft()) return;
          this.refreshStats();
          // Show newly imported photos as they arrive on the first page.
          if (this.jobs && this.jobs.pending > 0 && this.items.length < PAGE && !this.q) this.softRefresh();
        }, 2500);
      }
      if (!this.watchdog) {
        this.watchdog = setInterval(() => {
          if (this.conn.online && Date.now() - this.conn.lastMsg > 25000) {
            console.info('[live] heartbeat missed, reconnecting');
            this.conn.online = false;
            this.connect();
          }
        }, 5000);
        // Phones and laptops suspend timers; check right away when we wake.
        document.addEventListener('visibilitychange', () => {
          if (!document.hidden && Date.now() - this.conn.lastMsg > 15000) this.connect();
        });
      }
    },
    onHello(h) {
      const wasConnected = this.conn.everConnected;
      this.conn.online = true;
      this.conn.everConnected = true;
      this.conn.dev = h.dev;
      if (h.asset_hash !== window.ASSET_HASH) {
        // Never reload under someone's fingers: wait until they leave the
        // field they're typing in.
        const a = document.activeElement;
        if (a && a.matches('input, textarea') && a.value) {
          if (!this.reloadPending) {
            this.reloadPending = true;
            this.notify('Update ready — it will apply when you finish typing', 4000);
            a.addEventListener('blur', () => setTimeout(() => location.reload(), 300), {once: true});
          }
          return;
        }
        console.info('[live] UI changed, reloading');
        this.saveDrafts();
        this.writeHash();
        location.reload();
        return;
      }
      if (h.css_hash !== window.CSS_HASH) {
        hotSwapCSS(h.css_hash);
        window.CSS_HASH = h.css_hash;
        this.notify('Styles updated');
      }
      if (wasConnected && this.conn.buildID && h.build_id !== this.conn.buildID) {
        this.notify('Server updated');
        this.refreshStats();
        this.softRefresh();
        if (this.view === 'folders') this.loadFolders();
      } else if (wasConnected) {
        this.refreshStats();
      }
      this.conn.buildID = h.build_id;
    },
    onScan(s) {
      const finished = this.scan.running && !s.running;
      this.scan = s;
      if (s.running || finished) this.refreshStats();
      if (finished) this.loadFolders();
    },
    // notify shows a message; action ({label, fn}) adds a button, e.g. Undo.
    notify(text, ms = 2200, action = null) {
      clearTimeout(this.toast.timer);
      this.toast.text = text;
      this.toast.action = action;
      this.toast.show = true;
      this.toast.timer = setTimeout(() => { this.toast.show = false; }, ms);
    },

    runToastAction() {
      const a = this.toast.action;
      this.toast.show = false;
      this.toast.action = null;
      if (a) a.fn();
    },

    // ---------- formatting ----------
    // The rotation is part of the URLs: a corrected photo gets new images
    // even where the browser has cached the old ones forever.
    thumbURL(im) { return im.thumb ? '/thumb/' + im.hash + (im.rot ? '?r=' + im.rot : '') : ''; },
    previewURL(im) { return '/preview/' + im.id + '?h=' + im.hash + (im.rot ? '&r=' + im.rot : ''); },
    fmtCount: n => (n || 0).toLocaleString(),
    fmtBytes(n) {
      if (!n) return '0 B';
      const u = ['B', 'KB', 'MB', 'GB', 'TB'];
      const i = Math.min(u.length - 1, Math.floor(Math.log(n) / Math.log(1024)));
      return (n / 1024 ** i).toFixed(i > 1 ? 1 : 0) + ' ' + u[i];
    },
    // onLearn shows the background learning as it goes: the face counts
    // climb as faces are filed; when it's done, the new questions load.
    onLearn(l) {
      this.learn = l;
      if (this.stats.jobs && l.faces) this.stats.jobs.faces = l.faces;
      if (!l.busy) {
        this.stats.discover = l.questions;
        if (this.view !== 'discover') this.discover.since = 0; // learnt: the answers are out of its count
        if (this.view === 'discover') this.loadDiscover();
        if (l.filed || l.unfiled) this.loadPeople();
      }
    },
    learnLine() {
      const l = this.learn;
      if (!l.busy) return '';
      if (l.phase === 'questions') return `Filed ${fmtCount(l.filed)} faces by themselves · finding the next questions…`;
      return `Filing faces that are now near certain · pass ${l.pass} · +${fmtCount(l.filed)}` + (l.unfiled ? ` −${fmtCount(l.unfiled)}` : '');
    },
    // listWords: "A", "A and B", "A, B and C".
    listWords(xs) { return xs.length < 2 ? xs.join('') : xs.slice(0, -1).join(', ') + ' and ' + xs[xs.length - 1]; },
    // facesLine: the faces identified (and how many by the user), and not.
    facesLine() {
      const f = this.stats.jobs && this.stats.jobs.faces;
      if (!f) return '';
      return `${fmtCount(f.named)} faces identified (${fmtCount(f.confirmed)} by you) · ${fmtCount(f.unnamed)} not`;
    },
    fmtYear: t => t ? String(new Date(t * 1000).getFullYear()) : '',
    fmtDate: t => t ? new Date(t * 1000).toLocaleString(undefined, {dateStyle: 'medium', timeStyle: 'short'}) : '—',
    fmtDateShort: t => t ? new Date(t * 1000).toLocaleDateString(undefined, {day: 'numeric', month: 'short', year: 'numeric'}) : '',
    fmtDateLong: t => t ? new Date(t * 1000).toLocaleString(undefined, {weekday: 'long', day: 'numeric', month: 'long', year: 'numeric', hour: '2-digit', minute: '2-digit'}) : 'Unknown date',
    fmtAgo(t) {
      const s = Date.now() / 1000 - t;
      if (s < 60) return 'just now';
      if (s < 3600) return Math.floor(s / 60) + ' min ago';
      if (s < 86400) return Math.floor(s / 3600) + ' h ago';
      return Math.floor(s / 86400) + ' days ago';
    },
    exposureLine(im) {
      return [im.focal ? Math.round(im.focal) + ' mm' : '', im.fnumber ? 'ƒ/' + im.fnumber.toFixed(1) : '',
        im.exposure ? im.exposure + ' s' : '', im.iso ? 'ISO ' + im.iso : ''].filter(Boolean).join(' · ');
    },
    baseName: p => (p || '').split('/').pop(),
    dirName: p => (p || '').split('/').slice(0, -1).join('/'),
  }));
});

// api calls the server. With {retry: true}, a request that fails because
// the server is restarting is retried for up to ~20 s instead of being lost.
async function api(url, opts = {}, {retry = false} = {}) {
  for (let attempt = 0; ; attempt++) {
    let r;
    try {
      // A form (an upload) sets its own type, with the multipart boundary.
      r = await fetch(url, {headers: opts.body instanceof FormData ? {} : {'Content-Type': 'application/json'}, ...opts});
    } catch (e) {
      if (retry && attempt < 20) { await new Promise(res => setTimeout(res, 1000)); continue; }
      throw new Error('server unreachable');
    }
    if ((r.status === 502 || r.status === 503) && retry && attempt < 20) {
      await new Promise(res => setTimeout(res, 1000));
      continue;
    }
    const body = await r.json().catch(() => ({}));
    if (!r.ok) throw new Error(body.error || r.statusText);
    return body;
  }
}

// shownIDs lists the faces of a group card the user could see when
// deciding (the panel shows only the first few).
// "Is this also …?" answers are saved one at a time, in click order.
let alsoQueue = Promise.resolve(), alsoPending = 0;
// Photos whose faces were named since the viewer opened (all: a whole
// group was), checked against the search when it closes.
const touched = {ids: new Set(), all: false};
// Photos turned and not analysed again yet: their faces from before.
const turning = new Map();
function newFaceCands(pid = 0) { return {pid, items: [], answered: new Set(), show: 30, loading: false}; }
function newAlso(person = null, items = []) { return {person, items: items || [], answered: new Set()}; }
function shownIDs(g, n) { return (g.faces || []).slice(0, n || undefined).map(f => f.id); }

function hotSwapCSS(hash) {
  const old = document.querySelector('link[data-hot="app.css"]');
  if (!old) return;
  const link = old.cloneNode();
  link.href = '/static/app.css?v=' + hash;
  // Swap only once the new sheet has loaded, so nothing flashes unstyled.
  link.onload = () => old.remove();
  old.after(link);
}

function sessionStorageGet(k) { try { return sessionStorage.getItem(k); } catch (e) { return null; } }
// readScroll reads a saved list position ({y, id, off}; an old plain number is y).
function readScroll(k) {
  const v = sessionStorageGet(k);
  if (!v) return null;
  try { const at = JSON.parse(v); return typeof at === 'number' ? {y: at, id: 0, off: 0} : at; } catch (e) { return null; }
}
function aspect(im) { return im.w > 0 && im.h > 0 ? Math.min(3, Math.max(0.33, im.w / im.h)) : 1.5; }
function monthKey(t) { if (!t) return 'undated'; const d = new Date(t * 1000); return d.getFullYear() + '-' + d.getMonth(); }
function monthTitle(t) { if (!t) return 'Undated'; const d = new Date(t * 1000); return MONTHS[d.getMonth()] + ' ' + d.getFullYear(); }
function yearsFrom(tl) {
  const m = new Map();
  for (const {m: month, n} of tl) {
    const y = +month.slice(0, 4);
    m.set(y, (m.get(y) || 0) + n);
  }
  return [...m].map(([year, n]) => ({year, n}));
}
function fmtCount(n) { return (n || 0).toLocaleString(); }

// ---------- family tree ----------
// TREE sizes, in pixels: a person's box, the gaps beside partners and
// between families, a generation's height.
const TREE = {w: 92, partner: 10, gap: 34, row: 168, top: 24, avatar: 64};

// layoutFamily lays out the family entered (parents, partners, siblings;
// everything else follows from those) as trees, one per family, largest
// first: a generation to a row, partners side by side, children under
// their parents. Returns [{nodes: [{p, x, y}], lines: [{d, kind}], w, h}].
function layoutFamily(people) {
  const byId = new Map(people.map(p => [p.id, p]));
  const parents = new Map(), partners = new Map(), sibs = new Map();
  const link = (m, a, b) => { if (!m.has(a)) m.set(a, new Set()); m.get(a).add(b); };
  for (const p of people) {
    if (p.picture) continue;
    for (const r of p.family || []) {
      if (!byId.has(r.id)) continue;
      // Parents worked out too (siblings share them); partners and
      // siblings as entered.
      if (r.kind === 'parent') link(parents, p.id, r.id);
      else if (r.kind === 'partner' && !r.inferred) { link(partners, p.id, r.id); link(partners, r.id, p.id); }
      else if (r.kind === 'sibling' && !r.inferred) { link(sibs, p.id, r.id); link(sibs, r.id, p.id); }
    }
  }
  const ids = new Set([...parents.keys(), ...partners.keys(), ...sibs.keys()]);
  for (const ps of parents.values()) for (const q of ps) ids.add(q);
  // Families: everyone joined by any of those.
  const root = new Map([...ids].map(id => [id, id]));
  const find = a => { while (root.get(a) !== a) { root.set(a, root.get(root.get(a))); a = root.get(a); } return a; };
  const union = (a, b) => root.set(find(a), find(b));
  for (const m of [parents, partners, sibs]) for (const [a, qs] of m) for (const q of qs) union(a, q);
  // Generations: children below their parents; partners and siblings level.
  const gen = new Map([...ids].map(id => [id, 0]));
  for (let it = 0; it < 60; it++) {
    let changed = false;
    const raise = (id, g) => { if (gen.get(id) < g) { gen.set(id, g); changed = true; } };
    for (const [c, ps] of parents) for (const q of ps) { raise(c, gen.get(q) + 1); raise(q, gen.get(c) - 1); }
    for (const m of [partners, sibs]) for (const [a, qs] of m) for (const q of qs) raise(a, gen.get(q));
    if (!changed) break;
  }
  const families = new Map();
  for (const id of ids) {
    const r = find(id);
    if (!families.has(r)) families.set(r, []);
    families.get(r).push(id);
  }
  const born = id => { const p = byId.get(id); return p.birth_year || p.est_birth || 9999; };
  const out = [];
  for (const members of families.values()) {
    if (members.length < 2) continue;
    const g0 = Math.min(...members.map(id => gen.get(id)));
    const rows = [];
    for (const id of members) (rows[gen.get(id) - g0] ||= []).push(id);
    // A row's units: partners together, the one with most partners in
    // the middle (earlier and later partners either side).
    const units = rows.map(row => {
      const seen = new Set(), us = [];
      for (const id of [...row].sort((a, b) => born(a) - born(b))) {
        if (seen.has(id)) continue;
        const u = [], todo = [id];
        seen.add(id);
        while (todo.length) {
          const x = todo.pop();
          u.push(x);
          for (const q of partners.get(x) || []) if (!seen.has(q) && row.includes(q)) { seen.add(q); todo.push(q); }
        }
        if (u.length > 2) {
          u.sort((a, b) => (partners.get(b)?.size || 0) - (partners.get(a)?.size || 0));
          const [hub, ...rest] = u;
          u.splice(0, u.length, ...rest.slice(0, rest.length >> 1), hub, ...rest.slice(rest.length >> 1));
        } else if (u.length === 2 && byId.get(u[0]).gender === 'female' && byId.get(u[1]).gender === 'male') {
          u.reverse();
        }
        us.push(u);
      }
      return us;
    });
    const width = u => u.length * TREE.w + (u.length - 1) * TREE.partner;
    const x = new Map();
    // place lays a row's units out in order, without overlap, as near
    // their wishes as can be: units pushing each other apart settle
    // around the mean of their wishes (pool adjacent violators). Units
    // with no wish follow the one before.
    const place = (us, wish) => {
      const blocks = [];
      let prevWant = 0;
      us.forEach((u, i) => {
        const w = width(u), c = wish(u);
        const want = c == null ? prevWant : c - w / 2, weight = c == null ? 1e-3 : 1;
        prevWant = want + w + TREE.gap;
        let b = {units: [u], w, sum: want * weight, weight};
        b.left = b.sum / b.weight;
        while (blocks.length) {
          const last = blocks.at(-1);
          if (last.left + last.w + TREE.gap <= b.left) break;
          blocks.pop();
          // Merged: b's wishes count from the block's left edge.
          const shift = last.w + TREE.gap;
          b = {units: [...last.units, ...b.units], w: last.w + TREE.gap + b.w,
            sum: last.sum + b.sum - shift * b.weight, weight: last.weight + b.weight};
          b.left = b.sum / b.weight;
        }
        blocks.push(b);
      });
      for (const b of blocks) {
        let left = b.left;
        for (const u of b.units) {
          u.forEach((id, i) => x.set(id, left + i * (TREE.w + TREE.partner) + TREE.w / 2));
          left += width(u) + TREE.gap;
        }
      }
    };
    const mean = xs => xs.length ? xs.reduce((a, b) => a + b, 0) / xs.length : null;
    const upWish = u => mean(u.flatMap(id => [...(parents.get(id) || [])].filter(q => x.has(q)).map(q => x.get(q))));
    const kids = new Map();
    for (const [c, ps] of parents) for (const q of ps) { if (!kids.has(q)) kids.set(q, []); kids.get(q).push(c); }
    const downWish = u => mean(u.flatMap(id => (kids.get(id) || []).filter(c => x.has(c)).map(c => x.get(c))));
    const order = (us, wish) => {
      const key = new Map(us.map((u, i) => [u, wish(u) ?? (i ? x.get(us[i - 1].at(-1)) ?? -1 : -1) + 0.5]));
      us.sort((a, b) => key.get(a) - key.get(b));
    };
    // Sweeps down and up, ordering each row by where its parents or
    // children are, so families keep together and lines cross little.
    // Those with no parents here (married in) go by their children.
    const down = u => upWish(u) ?? downWish(u), up = u => downWish(u) ?? upWish(u);
    place(units[0] || [], () => null);
    for (let sweep = 0; sweep < 4; sweep++) {
      for (let g = 1; g < units.length; g++) { order(units[g], down); place(units[g], down); }
      for (let g = units.length - 2; g >= 0; g--) { order(units[g], up); place(units[g], up); }
    }
    for (let g = 1; g < units.length; g++) place(units[g], down);
    const minX = Math.min(...members.map(id => x.get(id) - TREE.w / 2));
    const nodes = members.map(id => ({p: byId.get(id), x: x.get(id) - minX, y: TREE.top + (gen.get(id) - g0) * TREE.row}));
    const at = new Map(nodes.map(n => [n.p.id, n]));
    const lines = [];
    const mid = TREE.avatar / 2;
    // Partners: a line between them.
    for (const n of nodes) for (const q of partners.get(n.p.id) || []) {
      const m = at.get(q);
      if (m && n.p.id < q && m.y === n.y) lines.push({kind: 'partner', d: `M${Math.min(n.x, m.x) + mid} ${n.y + mid}H${Math.max(n.x, m.x) - mid}`});
    }
    // Children: from between their parents down to a bar, and on to each.
    const broods = new Map();
    for (const n of nodes) {
      const ps = [...(parents.get(n.p.id) || [])].filter(q => at.has(q)).sort((a, b) => a - b);
      if (!ps.length) continue;
      const k = ps.join(',');
      if (!broods.has(k)) broods.set(k, {ps, kids: []});
      broods.get(k).kids.push(n);
    }
    for (const {ps, kids: cs} of broods.values()) {
      const px = mean(ps.map(q => at.get(q).x)), py = Math.max(...ps.map(q => at.get(q).y));
      const top = Math.min(...cs.map(c => c.y)), bar = top - 22;
      const from = ps.length === 2 && at.get(ps[0]).y === at.get(ps[1]).y && Math.abs(at.get(ps[0]).x - at.get(ps[1]).x) <= TREE.w + TREE.partner
        ? py + mid : py + TREE.avatar + 40; // a couple: from the line between them; one: below the name
      let d = `M${px} ${from}V${bar}`;
      const xs = cs.map(c => c.x);
      d += `M${Math.min(px, ...xs)} ${bar}H${Math.max(px, ...xs)}`;
      for (const c of cs) d += `M${c.x} ${bar}V${c.y}`;
      lines.push({kind: 'child', d});
    }
    // Siblings with no parents entered: a bracket over them.
    for (const n of nodes) for (const q of sibs.get(n.p.id) || []) {
      const m = at.get(q);
      if (m && n.p.id < q && m.y === n.y && !parents.get(n.p.id)?.size && !parents.get(q)?.size) {
        lines.push({kind: 'sibling', d: `M${n.x} ${n.y}V${n.y - 12}H${m.x}V${m.y}`});
      }
    }
    const w = Math.max(...nodes.map(n => n.x)) + TREE.w / 2;
    const h = Math.max(...nodes.map(n => n.y)) + TREE.row - 40;
    out.push({nodes, lines, w, h, size: members.length});
  }
  return out.sort((a, b) => b.size - a.size);
}
