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
  gear: '<svg viewBox="0 0 24 24"><circle cx="12" cy="12" r="3.2"/><path d="M12 2.8v2.4M12 18.8v2.4M2.8 12h2.4M18.8 12h2.4M5.5 5.5l1.7 1.7M16.8 16.8l1.7 1.7M5.5 18.5l1.7-1.7M16.8 7.2l1.7-1.7"/><circle cx="12" cy="12" r="6.6"/></svg>',
  edit: '<svg viewBox="0 0 24 24"><path d="M4 20h4L19 9l-4-4L4 16v4z"/><path d="M14 6l4 4"/></svg>',
  grid: '<svg viewBox="0 0 24 24"><rect x="4" y="4" width="7" height="7" rx="1.5"/><rect x="13" y="4" width="7" height="7" rx="1.5"/><rect x="4" y="13" width="7" height="7" rx="1.5"/><rect x="13" y="13" width="7" height="7" rx="1.5"/></svg>',
  close: '<svg viewBox="0 0 24 24"><path d="M6 6l12 12M18 6L6 18"/></svg>',
  left: '<svg viewBox="0 0 24 24"><path d="M15 5l-7 7 7 7"/></svg>',
  right: '<svg viewBox="0 0 24 24"><path d="M9 5l7 7-7 7"/></svg>',
  up: '<svg viewBox="0 0 24 24"><path d="M12 19V5M5 12l7-7 7 7"/></svg>',
  info: '<svg viewBox="0 0 24 24"><circle cx="12" cy="12" r="9"/><path d="M12 11v6M12 7.5v.5"/></svg>',
  download: '<svg viewBox="0 0 24 24"><path d="M12 4v11M7 10l5 5 5-5M5 20h14"/></svg>',
  camera: '<svg viewBox="0 0 24 24"><path d="M4 8h3l2-3h6l2 3h3v11H4z"/><circle cx="12" cy="13" r="3.5"/></svg>',
  image: '<svg viewBox="0 0 24 24"><rect x="3" y="3" width="18" height="18" rx="3"/><path d="M21 15l-5-5L5 21"/></svg>',
  server: '<svg viewBox="0 0 24 24"><rect x="3" y="4" width="18" height="7" rx="2"/><rect x="3" y="13" width="18" height="7" rx="2"/><path d="M7 7.5h.01M7 16.5h.01"/></svg>',
  people: '<svg viewBox="0 0 24 24"><circle cx="9" cy="8" r="3.5"/><path d="M2.5 20a6.5 6.5 0 0113 0"/><circle cx="17" cy="9" r="2.5"/><path d="M15.5 14.2A5 5 0 0121.5 19"/></svg>',
  check: '<svg viewBox="0 0 24 24"><path d="M5 12.5l4.5 4.5L19 7"/></svg>',
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
      }
      return 0;
    },
    nav: [
      {id: 'photos', label: 'Photos', icon: 'photos'},
      {id: 'favorites', label: 'Favorites', icon: 'heart', view: 'photos', fav: true},
      {id: 'people', label: 'People', icon: 'people'},
      {id: 'review', label: 'Review', icon: 'check', badge: true},
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
    semantic: false,
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
    reviewItems: [],
    reviewGroup: 0,     // a group opened from a photo, shown first in Review
    reviewFocus: null,
    reviewTotal: 0,
    reviewFaces: 0,     // faces in the review groups
    reviewOrder: 'size',
    peopleOrder: 'name',
    reviewLoading: false,
    names: {},          // group id -> typed name (drafts persist across reloads)
    busyGroup: {},      // group id -> request in flight
    mergeInto: '',
    progress: [], // recent {t, left} samples for the ETA
    folders: [],
    add: {kind: 'file', path: '', host: '', user: '', password: '', busy: false},
    folderError: '',
    browser: {open: false, location: '', parent: '', dirs: [], loading: false, error: ''},
    lightbox: {open: false, idx: 0, info: true, faces: false},
    assign: {face: null, group: null, name: '', busy: false},
    also: {person: null, items: [], busy: 0},
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
    peek: null,         // face crop shown large on hover: {f, x, y}   // "Is this also <name>?" after naming a face
    detail: null,
    geoCount: 0,
    map: null,
    mapLayer: null,
    mapFocus: null,     // the photo "Show on map" came from: {id, lat, lon, hash}
    mapFocusMarker: null,
    mapReturn: null,    // where "Back to photo" goes: {view, id}
    toast: {show: false, text: '', timer: null, action: null},
    conn: {online: false, everConnected: false, dev: false, buildID: null, es: null, lastMsg: 0, retry: 500, timer: null},

    // ---------- lifecycle ----------
    init() {
      this.readHash();
      try { this.rowHeight = +localStorage.getItem('rowHeight') || this.rowHeight; } catch (e) {}
      try { this.lightbox.faces = localStorage.getItem('showFaces') === '1'; } catch (e) {}
      try { this.reviewOrder = localStorage.getItem('reviewOrder') || 'size'; } catch (e) {}
      try { this.peopleOrder = localStorage.getItem('peopleOrder') || 'name'; } catch (e) {}
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
        const st = this.restoreScroll;
        if (st) this.$nextTick(() => { this.scroller().scrollTop = st; });
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
      this.fav = p.get('fav') === '1';
      this.person = +p.get('person') || 0;
      this.openPersonId = +p.get('p') || 0;
      this.reviewGroup = +p.get('g') || 0;
      this.pendingImage = +p.get('img') || null;
      this.restoreScroll = +sessionStorageGet('scroll:' + location.hash) || 0;
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
      if (h !== location.hash.slice(1)) history.replaceState(null, '', h ? '#' + h : location.pathname);
    },
    onHashChange() {
      const before = [this.view, this.q, this.tokenIDs(), this.sort, this.fav, this.person].join('|');
      const view = this.view;
      this.readHash();
      if ([this.view, this.q, this.tokenIDs(), this.sort, this.fav, this.person].join('|') !== before) this.reload();
      if (this.view !== view) this.enterView();
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
      this.view = v;
      this.writeHash();
      this.enterView();
    },
    // enterView loads what the current view shows. It runs however the
    // view was reached: sidebar, URL, reload or live reload.
    enterView() {
      const v = this.view;
      if (v === 'map') this.$nextTick(() => this.showMap());
      if (v === 'folders') this.loadFolders();
      if (v === 'settings') this.loadSettings();
      if (v === 'people') this.loadPeople();
      if (v === 'review') { this.loadPeople(); this.loadReview(true); }
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
      this.writeHash();
      const seq = ++this.reqSeq;
      this.loading = true;
      try {
        const [res, tl] = await Promise.all([
          api('/api/images?' + this.query({offset: 0, limit: PAGE})),
          api('/api/timeline?' + this.query()),
        ]);
        if (seq !== this.reqSeq) return;
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
      const res = await api('/api/images?' + this.query({offset: 0, limit: Math.min(n, 2000)}));
      if (seq !== this.reqSeq) return;
      this.items = res.items;
      this.total = res.total;
      this.layout();
    },
    async loadMore() {
      if (this.loading || this.items.length >= this.total || !this.ready) return;
      if (this.view !== 'photos' && this.view !== 'table') return;
      const seq = this.reqSeq;
      this.loading = true;
      try {
        const res = await api('/api/images?' + this.query({offset: this.items.length, limit: PAGE}));
        if (seq !== this.reqSeq) return;
        this.items = this.items.concat(res.items);
        this.total = res.total;
        this.layout();
      } finally {
        this.loading = false;
      }
    },
    // peopleChanged re-reads the counts (the Review badge) after any
    // decision about people; auto-filing can change them too.
    peopleChanged() {
      clearTimeout(this._statsTimer);
      this._statsTimer = setTimeout(() => this.refreshStats().catch(() => {}), 300);
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
      if (wasBusy && !this.busy()) {
        this.loadFolders();
        this.softRefresh();
      }
    },
    busy() { const j = this.jobs; return !!(this.scan.running || (j && (j.pending > 0 || j.embedded < j.embeddable))); },
    // One number for all per-photo processing: files waiting to be read
    // plus photos whose search embedding is still missing (backfill).
    remaining() {
      const j = this.jobs;
      if (!j) return 0;
      return (j.pending || 0) + Math.max(0, (j.embeddable || 0) - (j.embedded || 0));
    },
    processedPct() {
      const done = this.stats.library.images || 0, left = this.remaining();
      return done + left ? Math.min(100, 100 * done / (done + left)) : 0;
    },
    processingActive() { const j = this.jobs; return !!(j && ((j.import && j.import.running) || (j.embed && j.embed.running))); },
    processingNote() {
      const j = this.jobs;
      if (!j) return '';
      if (j.import && j.import.paused) return 'Paused';
      if (j.import && !j.import.online) return 'Source unreachable — will resume';
      if (j.pending === 0 && j.embed && !j.embed.online) return 'Search server offline — will resume';
      return '';
    },
    // ETA from the rate over the samples kept in refreshStats.
    etaLabel() {
      const p = this.progress;
      if (p.length < 3) return 'estimating…';
      const a = p[0], b = p[p.length - 1];
      const rate = (a.left - b.left) / ((b.t - a.t) / 1000); // photos per second
      if (!(rate > 0)) return this.processingActive() ? 'estimating…' : '';
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
      const byMonth = this.sort.startsWith('taken') && !this.semantic;
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
      const done = this.items.length >= this.total;
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
      try { sessionStorage.setItem('scroll:' + location.hash, el.scrollTop); } catch (e) {}
      if (this.view !== 'photos') return;
      // Highlight the year currently at the top of the viewport.
      const heads = el.querySelectorAll('.section-head');
      let y = null;
      for (const h of heads) {
        if (h.offsetTop - el.scrollTop > 120) break;
        y = h.textContent.slice(-4);
      }
      this.currentYear = y || (this.years[0] && this.years[0].year);
    },
    async jumpToYear(year) {
      // Load until the year's first photo is present, then scroll to it.
      const asc = this.sort === 'taken_asc';
      const reached = () => this.items.some(im => im.taken && new Date(im.taken * 1000).getFullYear() === +year);
      const past = () => {
        const last = this.items[this.items.length - 1];
        if (!last || !last.taken) return false;
        const ly = new Date(last.taken * 1000).getFullYear();
        return asc ? ly > year : ly < year;
      };
      while (!reached() && !past() && this.items.length < this.total) {
        await this.loadMoreNow();
      }
      this.$nextTick(() => {
        const head = [...this.$refs.photos.querySelectorAll('.section-head')].find(h => h.textContent.endsWith(String(year)));
        if (head) this.$refs.photos.scrollTo({top: head.offsetTop - 12, behavior: 'smooth'});
      });
    },
    async loadMoreNow() {
      const res = await api('/api/images?' + this.query({offset: this.items.length, limit: 1000}));
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
        const tiles = (this.settings && this.settings.saved.map_tiles) || 'https://tile.openstreetmap.org/{z}/{x}/{y}.png';
        L.tileLayer(tiles, {
          maxZoom: 19,
          attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors',
        }).addTo(this.map);
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
      this.closeLightbox();
      this.setView('map');
      // A new map focuses once its points load; an existing one now.
      if (this.map) this.$nextTick(() => this.focusMap());
    },

    // ---------- lightbox ----------
    current() { return this.lightbox.open ? this.lightboxItems()[this.lightbox.idx] : null; },
    lightboxItems() { return this.lightbox.list || this.items; },
    openLightbox(idx) {
      this.lightbox.list = null;
      this.lightbox.idx = idx;
      this.lightbox.open = true;
      this.loadDetail();
    },
    async openImageById(id) {
      let idx = this.items.findIndex(im => im.id === id);
      if (idx >= 0) return this.openLightbox(idx);
      const d = await api('/api/images/' + id).catch(() => null);
      if (!d) return;
      this.lightbox.list = [d];
      this.lightbox.idx = 0;
      this.lightbox.open = true;
      this.writeHash();
      this.loadDetail(); // also loads the faces
    },
    // alsoAnswer says whether a suggested group is the person just named,
    // then fetches the next best matches (a yes changes them).
    async alsoAnswer(g, yes) {
      const pid = this.also.person.id;
      if (this.also.busy) return;
      this.also.busy = g.id;
      try {
        await api(`/api/groups/${g.id}/${yes ? 'name' : 'reject'}`, {method: 'POST', body: JSON.stringify({person_id: pid, shown: yes ? shownIDs(g, 4) : []})}, {retry: true});
        const next = await api(`/api/people/${pid}/candidates?limit=5`).catch(() => null);
        if (this.also.person && this.also.person.id === pid) {
          this.also.items = next || this.also.items.filter(x => x.id !== g.id);
          if (!this.also.items.length) { this.closeAlso(); this.notify(`No more likely matches for ${this.alsoName()}`); }
        }
        if (yes) {
          this.refreshFaces();
          this.loadPeople();
          if (this.reviewItems.some(x => x.id === g.id)) {
            this.reviewItems = this.reviewItems.filter(x => x.id !== g.id);
            this.reviewTotal = Math.max(0, this.reviewTotal - 1);
            this.reviewFaces = Math.max(0, this.reviewFaces - (g.size || 0));
          }
        }
      } catch (e) {
        this.notify(`Couldn’t save: ${e.message}`, 6000);
      } finally {
        this.also.busy = 0;
      }
      this.peopleChanged();
    },
    // peekFace shows a hovered face crop large, above it (or below when
    // there's no room), kept inside the window.
    peekFace(e, f) {
      const r = e.target.getBoundingClientRect(), size = 236;
      const x = Math.min(Math.max(r.left + r.width / 2 - size / 2, 8), innerWidth - size - 8);
      const y = r.top - size - 30 > 8 ? r.top - size - 30 : r.bottom + 8;
      this.peek = {f, x, y};
    },
    closeAlso() { this.also = {person: null, items: [], busy: 0}; this.peek = null; },
    alsoName() { return this.also.person ? this.also.person.name || 'them' : ''; },
    closeLightbox() {
      this.closeAlso();
      this.lightbox.open = false;
      this.lightbox.list = null;
      this.detail = null;
      this.writeHash();
    },
    async step(d) {
      const n = this.lightbox.idx + d;
      if (n < 0) return;
      if (!this.lightbox.list && n >= this.items.length && this.items.length < this.total) await this.loadMoreNow();
      if (n >= this.lightboxItems().length) return;
      this.lightbox.idx = n;
      this.loadDetail();
      // Warm the cache for the next image.
      const next = this.lightboxItems()[n + d];
      if (next) new Image().src = this.previewURL(next);
    },
    async loadDetail() {
      this.closeAssign();
      const im = this.current();
      this.writeHash();
      if (!im) return;
      this.detail = {...im, files: []};
      const [d, faces] = await Promise.all([
        api('/api/images/' + im.id).catch(() => null),
        api('/api/images/' + im.id + '/faces').catch(() => []),
      ]);
      if (d && this.current() && this.current().id === d.id) this.detail = {...d, faces};
    },
    async saveEdit(patch) {
      const im = this.current();
      if (!im) return;
      await api('/api/images/' + im.id, {method: 'PATCH', body: JSON.stringify(patch)});
      Object.assign(im, patch);
      if (this.detail && this.detail.id === im.id) Object.assign(this.detail, patch);
    },
    // rotateCurrent turns the open photo (its camera recorded the wrong
    // orientation). The view updates at once; thumbnail and faces follow
    // once it has been analysed again upright, a few seconds later.
    async rotateCurrent(by) {
      const im = this.current();
      if (!im || this.rotating) return;
      this.rotating = true;
      try {
        const old = new Set(((this.detail && this.detail.faces) || []).map(f => f.id));
        const res = await api(`/api/images/${im.id}/rotate`, {method: 'POST', body: JSON.stringify({by})}, {retry: true});
        im.rot = res.rot;
        if (this.detail && this.detail.id === im.id) this.detail = {...this.detail, rot: res.rot, faces: []};
        this.notify('Turned — finding faces again…');
        // Faces and thumbnail come back once the import stage has redone it.
        for (const wait of [3000, 5000, 8000, 15000]) {
          await new Promise(r => setTimeout(r, wait));
          if (!this.current() || this.current().id !== im.id) break;
          await this.refreshFaces();
          const fs = (this.detail && this.detail.faces) || [];
          if (fs.length && !fs.some(f => old.has(f.id))) break; // analysed again
          if (fs.some(f => old.has(f.id))) this.detail = {...this.detail, faces: []}; // not yet: hide the old boxes
        }
        this.peopleChanged();
      } catch (e) {
        this.notify(`Couldn’t turn it: ${e.message}`, 5000);
      } finally {
        this.rotating = false;
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
      if (!f) return '';
      // Below the face, or above it when the face is near the bottom.
      const below = f.box[3] < 0.6;
      const x = Math.min(Math.max(f.box[0] * 100, 0), 70);
      return below ? `left:${x}%;top:calc(${f.box[3] * 100}% + 30px)` : `left:${x}%;bottom:calc(${(1 - f.box[1]) * 100}% + 8px)`;
    },
    async openAssign(f) {
      this.assign = {face: f, group: null, name: '', busy: false};
      if (!this.peopleData.people.length) this.loadPeople();
      if (f.group) {
        const g = await api(`/api/groups/${f.group}`).catch(() => null);
        // A person appears once in a photo: don't suggest anyone already
        // tagged on another face here.
        const here = new Set(((this.detail && this.detail.faces) || []).filter(o => o.id !== f.id && o.person).map(o => o.person));
        if (g) g.suggestions = g.suggestions.filter(p => !here.has(p.id));
        if (this.assign.face === f) this.assign.group = g;
      }
      this.$nextTick(() => this.$refs.assignInput && this.$refs.assignInput.focus());
    },
    closeAssign() { this.assign = {face: null, group: null, name: '', busy: false}; },
    // openGroup shows a face's whole group as the first card in Review.
    openGroup(gid) {
      this.closeAssign();
      this.closeLightbox();
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
    async notAFace() {
      const f = this.assign.face;
      if (!f || this.assign.busy) return;
      this.assign.busy = true;
      try {
        await api(`/api/faces/${f.id}/notface`, {method: 'POST'}, {retry: true});
        this.closeAssign();
        await this.refreshFaces();
        this.notify('Removed — it won’t be detected as a person again');
      } catch (e) {
        this.assign.busy = false;
        this.notify(`Couldn’t save: ${e.message}`, 6000);
      }
      this.peopleChanged();
    },
    async assignFace(personId) {
      const f = this.assign.face, name = this.assign.name.trim();
      if (!f || (!personId && !name) || this.assign.busy) return;
      this.assign.busy = true;
      try {
        // Tags just this face; its group is then offered as "Is this also …?".
        const res = await api(`/api/faces/${f.id}/tag`, {method: 'POST', body: JSON.stringify(personId ? {person_id: personId} : {name})}, {retry: true});
        const who = personId ? (this.peopleData.people.find(p => p.id === personId) || {}).name : name;
        this.notify('Saved');
        this.closeAssign();
        // Their other groups (other ages, other lighting) are likely
        // unnamed yet: ask about the best matches right away.
        this.also = {person: {id: res.person_id, name: who}, items: (res.candidates || []).slice(0, 5), busy: 0};
        await this.refreshFaces();
        this.loadPeople();
      } catch (e) {
        this.assign.busy = false;
        this.notify(`Couldn’t save: ${e.message}. Nothing was lost — try again.`, 6000);
      }
      this.peopleChanged();
    },
    async unassignFace() {
      const f = this.assign.face;
      if (!f || !f.person || this.assign.busy) return;
      this.assign.busy = true;
      try {
        // Just this face is not them.
        await api(`/api/faces/${f.id}/remove`, {method: 'POST'}, {retry: true});
        this.closeAssign();
        await this.refreshFaces();
        this.loadPeople();
      } catch (e) {
        this.assign.busy = false;
        this.notify(`Couldn’t save: ${e.message}`, 6000);
      }
      this.peopleChanged();
    },
    async refreshFaces() {
      const im = this.current();
      if (!im || !this.detail) return;
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
        else if (e.key === 'h') this.toggleFav(this.current());
        else if (e.key === 'i') this.lightbox.info = !this.lightbox.info;
        else if (e.key >= '0' && e.key <= '5') this.saveEdit({rating: +e.key});
        else return;
        e.preventDefault();
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
    // specialTokens are the face filters offered next to people: photos
    // with someone not named yet, or with anyone named.
    specialTokens() {
      return [
        {id: 'unknown', name: '$unknown', special: true, hint: 'someone not named yet', photos: this.peopleData.unknown_photos || 0},
        {id: 'known', name: '$known', special: true, hint: 'anyone you’ve named', photos: this.peopleData.known_photos || 0},
        {id: 'alone', name: '$alone', special: true, hint: 'only one person in the photo', photos: this.peopleData.alone_photos || 0},
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
    openPerson(p) {
      this.openPersonId = p.id;
      this.groupSel = {};
      this.moveTo = '';
      this.personGroups = [];
      if (this.view !== 'people') this.setView('people');
      this.writeHash();
      this.loadPerson();
      this.$nextTick(() => this.$refs.people && this.$refs.people.scrollTo({top: 0}));
    },
    async loadPerson() {
      const id = this.openPersonId;
      const groups = await api(`/api/people/${id}/groups`);
      if (id === this.openPersonId) this.personGroups = groups;
    },
    closePerson() { this.openPersonId = 0; this.writeHash(); this.loadPeople(); },
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
    // undoTakeOut puts a face that was taken out back as the person's (as
    // a face tagged as them).
    async undoTakeOut(f, pid) {
      try {
        await api(`/api/faces/${f.id}/tag`, {method: 'POST', body: JSON.stringify({person_id: pid})}, {retry: true});
        if (this.groupView.g && !this.groupView.faces.some(x => x.id === f.id)) this.groupView.faces.push(f);
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
    async loadReview(reset) {
      if (this.reviewLoading) return;
      this.reviewLoading = true;
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
      const ps = [...this.peopleData.people];
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
      const ok = await this.guard(it, () => {
        switch (action) {
          case 'person': return api(`/api/groups/${it.id}/name`, {method: 'POST', body: JSON.stringify({person_id: personId, shown: shownIDs(it)})}, {retry: true}).then(r => { named = r; });
          case 'name': return api(`/api/groups/${it.id}/name`, {method: 'POST', body: JSON.stringify({name, shown: shownIDs(it)})}, {retry: true}).then(r => { named = r; });
          case 'skip': return api(`/api/groups/${it.id}/skip`, {method: 'POST'}, {retry: true});
          case 'hide': return api(`/api/groups/${it.id}/hide`, {method: 'POST'}, {retry: true});
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
          this.also = {person: {id: named.person_id, name: action === 'name' ? name : this.personName(named.person_id)}, items: named.candidates.slice(0, 5), busy: 0};
        }
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
      es.addEventListener('ping', seen);
      es.onerror = () => {
        if (this.conn.online) console.info('[live] connection lost');
        this.conn.online = false;
        es.close();
        this.conn.timer = setTimeout(() => this.connect(), this.conn.retry);
        this.conn.retry = Math.min(5000, this.conn.retry * 1.6);
      };
      if (!this.statsTimer) {
        // Indexing progress isn't pushed; poll gently while it runs.
        this.statsTimer = setInterval(() => {
          if (!this.conn.online || !this.busy()) return;
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
      r = await fetch(url, {headers: {'Content-Type': 'application/json'}, ...opts});
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
