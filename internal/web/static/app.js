// FileQuery tarayıcı arayüzü
'use strict';

const $ = (s) => document.querySelector(s);
const $$ = (s) => [...document.querySelectorAll(s)];

const el = {
  surum: $('#surum'),
  sorgu: $('#sorgu'),
  vurgu: $('#vurgu'),
  calistir: $('#calistir'),
  durdur: $('#durdur'),
  ornekler: $('#ornekler'),
  olusturucu: $('#olusturucu'),
  onizlemeBaslik: $('#onizleme-baslik'),
  sekmeNot: $('#sekme-not'),
  ornekListesi: $('#ornek-listesi'),
  sistemNotu: $('#sistem-notu'),
  hata: $('#hata'),
  hataKod: $('#hata-kod'),
  hataMesaj: $('#hata-mesaj'),
  durum: $('#durum'),
  uyari: $('#uyari'),
  disaAktar: $('#disa-aktar'),
  tablo: $('#tablo'),
  tabloKap: $('#tablo-kap'),
  baslik: $('#baslik'),
  govde: $('#govde'),
  bos: $('#bos'),
  cekmece: $('#cekmece'),
  cekmeceBaslik: $('#cekmece-baslik'),
  alanListesi: $('#alan-listesi'),
  fonksiyonListesi: $('#fonksiyon-listesi'),
  gecmisListesi: $('#gecmis-listesi'),
  gecmisBos: $('#gecmis-bos'),
  bildirim: $('#bildirim'),
};

const KAYIT = {
  sorgu: 'fq.sorgu', gecmis: 'fq.gecmis', tema: 'fq.tema',
  sistem: 'fq.sistem', sekme: 'fq.sekme', olusturucu: 'fq.olusturucu',
};

const durum = {
  sema: null,
  alanlar: new Set(),
  fonksiyonlar: new Set(),
  sutunlar: [],
  satirlar: [],
  siralama: null, // { sutun, artan }
  iptal: null,
  sonSorgu: '',
  ilerleme: { scanned: 0, rows: 0 }, // son ilerleme mesajı
  sonuc: null, // 'tam' | 'kesik' | 'durduruldu'
  sanal: { satirY: 0, bas: -1, son: -1, istek: 0, genislikler: [] },
  sekme: 'sql',
  olusturucu: null, // kurulduğunda { yenile }
  uretilenSorgu: null, // oluşturucunun editöre son yazdığı sorgu
};

// ---------- Yardımcılar ----------

function oku(anahtar, varsayilan) {
  try {
    const v = localStorage.getItem(anahtar);
    return v === null ? varsayilan : JSON.parse(v);
  } catch {
    return varsayilan;
  }
}

function yaz(anahtar, deger) {
  try {
    localStorage.setItem(anahtar, JSON.stringify(deger));
  } catch { /* depolama yoksa önemli değil */ }
}

const sayi = new Intl.NumberFormat('tr-TR');
const ondalik = new Intl.NumberFormat('tr-TR', { maximumFractionDigits: 2 });
const tarihBicim = new Intl.DateTimeFormat('tr-TR', { dateStyle: 'short', timeStyle: 'short' });

function boyut(b) {
  if (b < 1024) return `${sayi.format(b)} B`;
  const birimler = ['KB', 'MB', 'GB', 'TB', 'PB'];
  let f = b;
  let i = -1;
  while (f >= 1024 && i < birimler.length - 1) {
    f /= 1024;
    i++;
  }
  return `${f.toLocaleString('tr-TR', { minimumFractionDigits: 1, maximumFractionDigits: 1 })} ${birimler[i]}`;
}

function sure(saniye) {
  let s = Math.abs(saniye);
  const g = Math.floor(s / 86400); s -= g * 86400;
  const sa = Math.floor(s / 3600); s -= sa * 3600;
  const dk = Math.floor(s / 60);
  const parcalar = [];
  if (g) parcalar.push(`${g}g`);
  if (sa) parcalar.push(`${sa}sa`);
  if (dk || !parcalar.length) parcalar.push(`${dk}dk`);
  return (saniye < 0 ? '-' : '') + parcalar.join(' ');
}

function hucreMetni(v, boyutMu) {
  if (v === null || v === undefined) return '';
  if (typeof v === 'boolean') return v ? 'evet' : 'hayır';
  if (typeof v === 'number') return boyutMu ? boyut(v) : (Number.isInteger(v) ? sayi.format(v) : ondalik.format(v));
  if (typeof v === 'object' && 't' in v) return tarihBicim.format(new Date(v.t));
  if (typeof v === 'object' && 'd' in v) return sure(v.d);
  return String(v);
}

function siralamaDegeri(v) {
  if (v === null || v === undefined) return null;
  if (typeof v === 'object' && 't' in v) return Date.parse(v.t);
  if (typeof v === 'object' && 'd' in v) return v.d;
  if (typeof v === 'boolean') return v ? 1 : 0;
  return v;
}

const karsilastir = new Intl.Collator('tr', { sensitivity: 'base', numeric: true }).compare;

function bildir(metin) {
  el.bildirim.textContent = metin;
  el.bildirim.hidden = false;
  clearTimeout(bildir.zaman);
  bildir.zaman = setTimeout(() => { el.bildirim.hidden = true; }, 3500);
}

// ---------- Sözdizimi renklendirme ----------

const ANAHTARLAR = new Set(['SELECT', 'FROM', 'WHERE', 'AND', 'OR', 'NOT', 'LIKE', 'MATCHES', 'IN', 'BETWEEN',
  'ORDER', 'BY', 'ASC', 'DESC', 'LIMIT', 'GROUP', 'AS', 'DEPTH', 'TRUE', 'FALSE', 'NULL', 'IS']);

function kacis(s) {
  return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
}

// Sunucudaki lexer'ın sadeleştirilmiş bir kopyası; sadece renklendirme için.
function renklendir(metin, hataKonumu) {
  const desen = /(--[^\n]*)|('(?:[^']|'')*'?|"(?:[^"]|"")*"?)|(\d+(?:\.\d+)?(?:\s+[KkMmGgTt]?[Bb](?![\p{L}\p{N}_])|[\p{L}]*))|(@[\p{L}\p{N}_]*)|([\p{L}_][\p{L}\p{N}_]*)|(\s+)|([\s\S])/gu;
  let html = '';
  let m;
  while ((m = desen.exec(metin)) !== null) {
    const [parca, yorum, metinSabit, sayiSabit, hazirListe, kelime] = m;
    const baslangic = m.index;
    let sinif = '';
    if (yorum) sinif = 't-yorum';
    else if (metinSabit) sinif = 't-metin';
    else if (sayiSabit) sinif = 't-sayi';
    else if (hazirListe) sinif = 't-liste';
    else if (kelime) {
      const buyuk = kelime.toUpperCase();
      const sonraki = metin.slice(baslangic + kelime.length).trimStart()[0];
      if (sonraki === '(' && durum.fonksiyonlar.has(buyuk)) sinif = 't-fonk';
      else if (ANAHTARLAR.has(buyuk)) sinif = (buyuk === 'GROUP' || buyuk === 'DEPTH') && durum.alanlar.has(kelime.toLowerCase()) && !/^\s*(BY|\d)/i.test(metin.slice(baslangic + kelime.length)) ? 't-alan' : 't-anahtar';
      else if (durum.alanlar.has(kelime.toLowerCase())) sinif = 't-alan';
    }

    let icerik = kacis(parca);
    if (hataKonumu !== null && hataKonumu >= baslangic && hataKonumu < baslangic + parca.length && !/^\s+$/.test(parca)) {
      sinif = (sinif + ' t-hata').trim();
    }
    html += sinif ? `<span class="${sinif}">${icerik}</span>` : icerik;
  }
  // Son satır boşsa yüksekliğin eşleşmesi için
  return html + '\n';
}

// Hata konumu karakter (rune) olarak gelir; JS dizeleri UTF-16 olduğu için çevir.
function runeKonumu(metin, runeIndex) {
  let i = 0;
  let r = 0;
  for (const ch of metin) {
    if (r === runeIndex) return i;
    i += ch.length;
    r++;
  }
  return i;
}

let hataKonumu = null;

function vurguyuGuncelle() {
  el.vurgu.innerHTML = renklendir(el.sorgu.value, hataKonumu);
  el.vurgu.parentElement.scrollTop = el.sorgu.scrollTop;
}

// ---------- Şema, örnekler, alanlar ----------

// ---------- İşletim sistemi ve örnekler ----------

const SISTEM_ADLARI = { windows: 'Windows', linux: 'Linux', darwin: 'macOS' };

// Sunucu kullanıcının bilgisayarında çalıştığı için işletim sistemini o bildirir.
// Bildirmezse (eski sürüm) tarayıcının kimliğinden tahmin edilir.
function algilananSistem() {
  if (durum.sema?.os && durum.sema.os in SISTEM_ADLARI) return durum.sema.os;
  const ua = `${navigator.userAgentData?.platform || ''} ${navigator.userAgent}`;
  if (/win/i.test(ua)) return 'windows';
  if (/mac/i.test(ua)) return 'darwin';
  return 'linux';
}

function seciliSistem() {
  const s = oku(KAYIT.sistem, null);
  return s in SISTEM_ADLARI ? s : algilananSistem();
}

function ornekListesi() {
  return window.FQOrnekler.uret(seciliSistem(), algilananSistem(), durum.sema?.places || []);
}

function ornegiYukle(sorgu) {
  sekmeAc('sql');
  sorguyuAyarla(sorgu);
  el.sorgu.focus();
}

async function semaYukle() {
  try {
    const yanit = await fetch('/api/schema');
    if (!yanit.ok) throw new Error((await yanit.json()).hata);
    durum.sema = await yanit.json();
  } catch (hata) {
    hataGoster(null, `Sunucuya ulaşılamadı: ${hata.message}. fq penceresinin açık olduğundan emin olun.`);
    return;
  }
  el.surum.textContent = `v${durum.sema.version}`;
  durum.sema.fields.forEach((f) => durum.alanlar.add(f.name));
  durum.sema.functions.forEach((f) => durum.fonksiyonlar.add(f));

  el.alanListesi.replaceChildren(...durum.sema.fields.map((f) => {
    const li = document.createElement('li');
    const b = document.createElement('button');
    b.type = 'button';
    b.innerHTML = '<span><span class="alan-ad"></span><span class="alan-tur"></span></span><span class="alan-aciklama"></span>';
    b.querySelector('.alan-ad').textContent = f.name;
    b.querySelector('.alan-tur').textContent = f.kind;
    b.querySelector('.alan-aciklama').textContent = f.desc;
    b.addEventListener('click', () => ekle(f.name));
    li.append(b);
    return li;
  }));

  el.fonksiyonListesi.replaceChildren(...durum.sema.functions.map((f) => {
    const li = document.createElement('li');
    const b = document.createElement('button');
    b.type = 'button';
    b.textContent = `${f}()`;
    b.addEventListener('click', () => ekle(f === 'COUNT' ? 'COUNT(*)' : `${f}()`, f === 'COUNT' ? 0 : -1));
    li.append(b);
    return li;
  }));

  $('#liste-listesi').replaceChildren(...(durum.sema.sets || []).map((k) => {
    const li = document.createElement('li');
    const b = document.createElement('button');
    b.type = 'button';
    b.innerHTML = '<span><span class="liste-ad"></span><span class="alan-tur"></span></span><span class="alan-aciklama"></span><span class="liste-uzantilar"></span>';
    b.querySelector('.liste-ad').textContent = `@${k.name}`;
    b.querySelector('.alan-tur').textContent = `${k.exts.length} uzantı`;
    b.querySelector('.alan-aciklama').textContent = k.desc;
    b.querySelector('.liste-uzantilar').textContent = k.exts.join(', ');
    b.addEventListener('click', () => ekle(`@${k.name}`));
    li.append(b);
    return li;
  }));

  ornekleriOlustur();
  if (!el.sorgu.value.trim()) sorguyuAyarla(ornekListesi()[0].sorgu);
  if (durum.sekme === 'olusturucu') olusturucuyuKur();
  vurguyuGuncelle();
}

// ---------- Sekmeler ve sorgu oluşturucu ----------

function sekmeAc(ad) {
  durum.sekme = ad;
  yaz(KAYIT.sekme, ad);
  $$('[data-sekme]').forEach((b) => b.setAttribute('aria-selected', String(b.dataset.sekme === ad)));
  const olusturucu = ad === 'olusturucu';
  el.olusturucu.hidden = !olusturucu;
  el.onizlemeBaslik.hidden = !olusturucu;
  el.sekmeNot.textContent = '';
  if (olusturucu) {
    if (!durum.olusturucu && durum.sema) olusturucuyuKur();
    else durum.olusturucu?.yenile();
  }
}

function olusturucuyuKur() {
  const yerler = durum.sema?.places || [];
  const varsayilan = (yerler.find((y) => y.key === 'downloads') || yerler[0] || { path: '~' }).path;
  durum.olusturucu = window.FQOlusturucu.kur(el.olusturucu, {
    yerler,
    durum: oku(KAYIT.olusturucu, null),
    varsayilanKaynak: varsayilan,
    kumeler: durum.sema?.sets || [],
    kaydet: (d) => yaz(KAYIT.olusturucu, d),
    sqlDegisti: (sql) => {
      durum.uretilenSorgu = sql;
      el.sekmeNot.textContent = '';
      sorguyuAyarla(sql);
    },
    dogrula: sorguyuDenetle,
  });
}

// Sorguyu çalıştırmadan denetler (söz dizimi ve klasörlerin varlığı).
async function sorguyuDenetle(sorgu) {
  try {
    const yanit = await fetch('/api/check', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ query: sorgu }),
    });
    const sonuc = await yanit.json();
    if (sonuc.ok) return { ok: true };
    return { ok: false, mesaj: sonuc.error?.message || sonuc.hata || 'Sorgu denetlenemedi' };
  } catch (hata) {
    return { ok: false, mesaj: `Sunucuya ulaşılamadı: ${hata.message}` };
  }
}

function ornekleriOlustur() {
  const liste = ornekListesi();
  const hizli = liste.slice(0, 4).map((o) => {
    const b = document.createElement('button');
    b.type = 'button';
    b.className = 'ornek';
    b.textContent = o.ad;
    b.title = `${o.aciklama}\n\n${o.sorgu}`;
    b.addEventListener('click', () => ornegiYukle(o.sorgu));
    return b;
  });
  const tumu = document.createElement('button');
  tumu.type = 'button';
  tumu.className = 'ornek ornek-tumu';
  tumu.textContent = `Tüm örnekler (${liste.length})`;
  tumu.addEventListener('click', () => cekmeceAc('ornekler'));
  el.ornekler.replaceChildren(...hizli, tumu);
  if (!el.cekmece.hidden && el.cekmece.dataset.panel === 'ornekler') ornekPaneliCiz();
}

function ornekPaneliCiz() {
  const secili = seciliSistem();
  const algilanan = algilananSistem();
  $$('[data-sistem]').forEach((b) => {
    b.setAttribute('aria-checked', String(b.dataset.sistem === secili));
    b.textContent = SISTEM_ADLARI[b.dataset.sistem] + (b.dataset.sistem === algilanan ? ' ✓' : '');
    b.title = b.dataset.sistem === algilanan ? 'Bu bilgisayar' : '';
  });
  el.sistemNotu.textContent = secili === algilanan
    ? `Bu bilgisayar ${SISTEM_ADLARI[algilanan]} olarak algılandı; örnekler kendi klasör yollarınızı kullanıyor.`
    : `Bu bilgisayar ${SISTEM_ADLARI[algilanan]}. ${SISTEM_ADLARI[secili]} örnekleri genel klasör yollarıyla gösteriliyor.`;

  const gruplar = new Map();
  for (const o of ornekListesi()) {
    if (!gruplar.has(o.grup)) gruplar.set(o.grup, []);
    gruplar.get(o.grup).push(o);
  }
  const parcalar = [];
  for (const [grup, ornekler] of gruplar) {
    const baslik = document.createElement('h3');
    baslik.textContent = grup;
    const ul = document.createElement('ul');
    ul.className = 'ornek-listesi';
    for (const o of ornekler) {
      const li = document.createElement('li');
      const b = document.createElement('button');
      b.type = 'button';
      const ad = document.createElement('strong');
      ad.textContent = o.ad;
      const aciklama = document.createElement('span');
      aciklama.textContent = o.aciklama;
      const kod = document.createElement('code');
      kod.textContent = o.sorgu;
      b.append(ad, aciklama, kod);
      b.addEventListener('click', () => {
        ornegiYukle(o.sorgu);
        cekmeceKapat();
      });
      li.append(b);
      ul.append(li);
    }
    parcalar.push(baslik, ul);
  }
  el.ornekListesi.replaceChildren(...parcalar);
}

function sorguyuAyarla(metin) {
  el.sorgu.value = metin;
  hataKonumu = null;
  el.hata.hidden = true;
  vurguyuGuncelle();
  yaz(KAYIT.sorgu, metin);
}

// İmlecin olduğu yere metin ekler; imleçKaydir, eklenen metnin sonundan geri gidilecek karakter sayısıdır.
function ekle(metin, imlecKaydir = 0) {
  const t = el.sorgu;
  const bas = t.selectionStart;
  const son = t.selectionEnd;
  const once = t.value.slice(0, bas);
  const bosluk = once && !/[\s(,]$/.test(once) ? ' ' : '';
  t.setRangeText(bosluk + metin, bas, son, 'end');
  t.selectionStart = t.selectionEnd = t.selectionEnd + imlecKaydir;
  t.focus();
  vurguyuGuncelle();
}

// ---------- Çekmece ----------

function cekmeceAc(ad) {
  const acik = !el.cekmece.hidden && el.cekmece.dataset.panel === ad;
  $$('[data-panel]').forEach((b) => b.setAttribute('aria-expanded', String(!acik && b.dataset.panel === ad)));
  if (acik) {
    el.cekmece.hidden = true;
    return;
  }
  el.cekmece.dataset.panel = ad;
  el.cekmece.hidden = false;
  el.cekmeceBaslik.textContent = { alanlar: 'Alanlar ve fonksiyonlar', gecmis: 'Geçmiş', ornekler: 'Örnek sorgular' }[ad];
  $('#cekmece-alanlar').hidden = ad !== 'alanlar';
  $('#cekmece-gecmis').hidden = ad !== 'gecmis';
  $('#cekmece-ornekler').hidden = ad !== 'ornekler';
  if (ad === 'gecmis') gecmisiGoster();
  if (ad === 'ornekler') ornekPaneliCiz();
}

function cekmeceKapat() {
  el.cekmece.hidden = true;
  $$('[data-panel]').forEach((b) => b.setAttribute('aria-expanded', 'false'));
}

function gecmiseEkle(sorgu) {
  const liste = oku(KAYIT.gecmis, []).filter((g) => g.sorgu !== sorgu);
  liste.unshift({ sorgu, zaman: Date.now() });
  yaz(KAYIT.gecmis, liste.slice(0, 30));
}

function gecmisiGoster() {
  const liste = oku(KAYIT.gecmis, []);
  el.gecmisBos.hidden = liste.length > 0;
  el.gecmisListesi.replaceChildren(...liste.map((g) => {
    const li = document.createElement('li');
    const b = document.createElement('button');
    b.type = 'button';
    const kod = document.createElement('code');
    kod.textContent = g.sorgu;
    const zaman = document.createElement('small');
    zaman.textContent = tarihBicim.format(new Date(g.zaman));
    b.append(kod, zaman);
    b.addEventListener('click', () => {
      ornegiYukle(g.sorgu);
      cekmeceKapat();
    });
    li.append(b);
    return li;
  }));
}

// ---------- Sorgu çalıştırma ----------

function hataGoster(sorgu, mesaj, konum = -1) {
  el.hata.hidden = false;
  el.hataMesaj.textContent = mesaj;
  if (sorgu && konum >= 0) {
    // Hatanın olduğu satırı ve altına işaret göster
    const satirlar = sorgu.split('\n');
    let kalan = konum;
    let satir = 0;
    for (const s of satirlar) {
      const uzunluk = [...s].length;
      if (kalan <= uzunluk) break;
      kalan -= uzunluk + 1;
      satir++;
    }
    el.hataKod.textContent = `${satirlar[satir] ?? ''}\n${' '.repeat(Math.max(0, kalan))}^`;
    el.hataKod.hidden = false;
    hataKonumu = runeKonumu(sorgu, konum);
  } else {
    el.hataKod.hidden = true;
    hataKonumu = null;
  }
  vurguyuGuncelle();
}

function calisiyorMu(evet) {
  el.calistir.disabled = evet;
  el.durdur.hidden = !evet;
  el.durum.classList.toggle('calisiyor', evet);
}

function durumYaz(html) {
  el.durum.innerHTML = html;
}

async function calistir() {
  const sorgu = el.sorgu.value.trim();
  if (!sorgu || durum.iptal) return;

  yaz(KAYIT.sorgu, el.sorgu.value);
  el.hata.hidden = true;
  el.uyari.hidden = true;
  hataKonumu = null;
  vurguyuGuncelle();

  durum.sutunlar = [];
  durum.satirlar = [];
  durum.siralama = null;
  durum.ilerleme = { scanned: 0, rows: 0 };
  durum.sonuc = null;
  durum.sanal.bas = durum.sanal.son = -1;
  durum.sanal.genislikler = [];
  el.govde.replaceChildren();
  el.tabloKap.scrollTop = 0;
  el.tablo.hidden = true;
  el.disaAktar.hidden = true;
  calisiyorMu(true);
  durumYaz('Başlatılıyor…');

  const iptal = new AbortController();
  durum.iptal = iptal;
  let bitti = false;

  try {
    const yanit = await fetch('/api/query', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ query: sorgu }),
      signal: iptal.signal,
    });
    if (!yanit.ok) {
      const govde = await yanit.json().catch(() => ({}));
      throw new Error(govde.hata || `HTTP ${yanit.status}`);
    }

    const okuyucu = yanit.body.getReader();
    const cozucu = new TextDecoder();
    let tampon = '';
    for (;;) {
      const { value, done } = await okuyucu.read();
      if (done) break;
      tampon += cozucu.decode(value, { stream: true });
      let satirSonu;
      while ((satirSonu = tampon.indexOf('\n')) >= 0) {
        const satir = tampon.slice(0, satirSonu);
        tampon = tampon.slice(satirSonu + 1);
        if (satir) bitti = mesajIsle(JSON.parse(satir), sorgu) || bitti;
      }
    }
  } catch (hata) {
    if (hata.name === 'AbortError') {
      const bulunan = Math.max(durum.ilerleme.rows, durum.satirlar.length);
      const taranan = Math.max(durum.ilerleme.scanned, bulunan);
      let html = `Durduruldu: <strong>${sayi.format(bulunan)}</strong> sonuç, ${sayi.format(taranan)} öğe tarandı`;
      if (bulunan > durum.satirlar.length) html += ` (${sayi.format(durum.satirlar.length)} tanesi gösteriliyor)`;
      durumYaz(html + '.');
      durum.sonuc = 'durduruldu';
      if (durum.satirlar.length) el.disaAktar.hidden = false;
    } else {
      hataGoster(null, hata.message);
      durumYaz('');
    }
  } finally {
    durum.iptal = null;
    calisiyorMu(false);
    if (bitti) gecmiseEkle(sorgu);
  }
}

// Sunucudan gelen her NDJSON satırını işler; sorgu başarıyla bittiyse true döner.
function mesajIsle(m, sorgu) {
  if (m.columns) {
    durum.sutunlar = m.columns;
    durum.sonSorgu = sorgu;
    basligiOlustur();
    el.tablo.hidden = false;
    el.bos.hidden = true;
  } else if (m.rows) {
    for (const r of m.rows) durum.satirlar.push(r);
    cizimIste();
  } else if (m.progress) {
    durum.ilerleme = m.progress;
    durumYaz(`Taranıyor: <strong>${sayi.format(m.progress.scanned)}</strong> öğe, <strong>${sayi.format(m.progress.rows)}</strong> sonuç`);
  } else if (m.error) {
    hataGoster(sorgu, m.error.message, m.error.pos);
    durumYaz('');
    el.tablo.hidden = durum.satirlar.length === 0;
    if (!durum.sutunlar.length) el.bos.hidden = false;
  } else if (m.done) {
    sonucuYaz(m.done);
    return true;
  }
  return false;
}

function sonucuYaz(d) {
  const sn = (d.elapsedMs / 1000).toLocaleString('tr-TR', { minimumFractionDigits: 2, maximumFractionDigits: 2 });
  const toplam = d.total ?? d.rows;
  let html = toplam === 0 ? 'Sonuç yok. ' : `<strong>${sayi.format(toplam)}</strong> sonuç, `;
  html += `${sayi.format(d.scanned)} öğe tarandı, ${sn} sn`;
  if (d.denied > 0) {
    const liste = (d.deniedPaths || []).join('\n');
    html += `; <span class="erisilemeyen" title="${kacis(liste)}">erişim izni olmayan ${sayi.format(d.denied)} klasör atlandı</span>`;
  }
  durumYaz(html);
  durum.sonuc = d.truncated ? 'kesik' : 'tam';
  el.disaAktar.hidden = d.rows === 0;
  if (d.truncated) {
    el.uyari.textContent = `${sayi.format(toplam)} sonuçtan ${sayi.format(d.rows)} tanesi gösteriliyor. Tamamı için dışa aktarın `
      + '(sorgu bunun için yeniden çalışır) ya da sorguyu daraltın (WHERE, LIMIT).';
    el.uyari.hidden = false;
  }
  // Sorgu bitince tablo bir sonraki kareyi beklemeden güncel olsun.
  cancelAnimationFrame(durum.sanal.istek);
  ciz();
  if (d.rows === 0) {
    el.tablo.hidden = true;
  }
}

function durdur() {
  durum.iptal?.abort();
}

// ---------- Tablo ----------

function yolSutunlari() {
  const adlar = durum.sutunlar.map((c) => c.name.toLowerCase());
  return { yol: adlar.indexOf('path'), klasor: adlar.indexOf('dir'), ad: adlar.indexOf('name') };
}

function satirYolu(satir, s) {
  if (s.yol >= 0 && typeof satir[s.yol] === 'string') return satir[s.yol];
  if (s.klasor >= 0 && s.ad >= 0 && typeof satir[s.klasor] === 'string' && typeof satir[s.ad] === 'string') {
    const ayrac = durum.sema?.separator || '\\';
    return satir[s.klasor].replace(/[\\/]+$/, '') + ayrac + satir[s.ad];
  }
  return null;
}

function basligiOlustur() {
  const s = yolSutunlari();
  const hucreler = durum.sutunlar.map((c, i) => {
    const th = document.createElement('th');
    th.scope = 'col';
    if (c.isSize) th.className = 'sag';
    const b = document.createElement('button');
    b.type = 'button';
    b.textContent = c.name;
    b.addEventListener('click', () => {
      if (durum.satirlar.length < 50_000) return sirala(i);
      // Büyük listede sıralama bir iki saniye sürebilir; önce bilgiyi çiz.
      const onceki = el.durum.innerHTML;
      durumYaz('Sıralanıyor…');
      document.body.style.cursor = 'progress';
      setTimeout(() => {
        sirala(i);
        durumYaz(onceki);
        document.body.style.cursor = '';
      }, 30);
    });
    th.append(b);
    return th;
  });
  if (s.yol >= 0 || (s.klasor >= 0 && s.ad >= 0)) {
    const th = document.createElement('th');
    th.setAttribute('aria-label', 'İşlemler');
    hucreler.push(th);
  }
  el.baslik.replaceChildren(...hucreler);
}

function satirOlustur(satir, s, yolVar) {
  const tr = document.createElement('tr');
  durum.sutunlar.forEach((c, i) => {
    const td = document.createElement('td');
    const v = satir[i];
    const metin = hucreMetni(v, c.isSize);
    td.textContent = metin;
    if (c.isSize) td.className = 'sag';
    else if (typeof v === 'number') td.className = 'sag';
    const ad = c.name.toLowerCase();
    if (ad === 'path' || ad === 'dir') {
      td.className = 'yol';
      td.title = metin;
    }
    if (metin === '') td.classList.add('bos-hucre');
    tr.append(td);
  });
  if (yolVar) {
    const td = document.createElement('td');
    td.className = 'islem';
    const yol = satirYolu(satir, s);
    if (yol) {
      const b = document.createElement('button');
      b.type = 'button';
      b.className = 'goster';
      b.textContent = 'Klasörde göster';
      b.addEventListener('click', () => klasordeGoster(yol));
      td.append(b);
    }
    tr.append(td);
  }
  return tr;
}

// Sanal kaydırma: yüz binlerce satırda da yalnızca ekranda görünen satırlar (ve birkaç
// yedek) DOM'a eklenir; üstte ve altta boşluk satırları kaydırma çubuğunun boyunu korur.
// Tarayıcılar çok yüksek öğeleri çizemediği için toplam yükseklik MAKS_YUKSEKLIK ile
// sınırlanır; daha uzun listelerde kaydırma konumu orantılı olarak satıra çevrilir.
const MAKS_YUKSEKLIK = 8_000_000;
const YEDEK = 8;

function cizimIste(zorla = false) {
  if (zorla) durum.sanal.bas = -1;
  if (!durum.sanal.istek) durum.sanal.istek = requestAnimationFrame(ciz);
}

function boslukSatiri(yukseklik, sutunSayisi) {
  const tr = document.createElement('tr');
  tr.className = 'bosluk';
  tr.setAttribute('aria-hidden', 'true');
  const td = document.createElement('td');
  td.colSpan = sutunSayisi;
  td.style.height = `${yukseklik}px`;
  tr.append(td);
  return tr;
}

function ciz() {
  const v = durum.sanal;
  v.istek = 0;
  const n = durum.satirlar.length;
  if (!n || el.tablo.hidden) {
    el.govde.replaceChildren();
    v.bas = v.son = -1;
    return;
  }
  const h = v.satirY || 33;
  const gorunur = Math.max(el.tabloKap.clientHeight - el.baslik.offsetHeight, h);
  const gercek = n * h;
  const toplam = Math.min(gercek, MAKS_YUKSEKLIK);
  const oran = toplam > gorunur ? (gercek - gorunur) / (toplam - gorunur) : 1;
  const k = Math.min(el.tabloKap.scrollTop, Math.max(0, toplam - gorunur));
  const gercekKonum = k * oran;
  const ilk = Math.min(n - 1, Math.floor(gercekKonum / h));
  const olcekli = oran > 1.0001;
  const bas = olcekli ? ilk : Math.max(0, ilk - YEDEK);
  const son = Math.min(n, ilk + Math.ceil(gorunur / h) + 1 + YEDEK);
  const ust = olcekli ? Math.max(0, k - (gercekKonum - ilk * h)) : bas * h;
  const alt = Math.max(0, toplam - ust - (son - bas) * h);
  const sutunSayisi = el.baslik.children.length || 1;

  if (bas === v.bas && son === v.son && el.govde.firstChild) {
    // Yalnızca liste uzadı: görünen satırlar aynı, boşlukları güncelle.
    el.govde.firstChild.firstChild.style.height = `${ust}px`;
    el.govde.lastChild.firstChild.style.height = `${alt}px`;
    return;
  }
  v.bas = bas;
  v.son = son;

  const s = yolSutunlari();
  const yolVar = s.yol >= 0 || (s.klasor >= 0 && s.ad >= 0);
  const parca = document.createDocumentFragment();
  parca.append(boslukSatiri(ust, sutunSayisi));
  for (let i = bas; i < son; i++) parca.append(satirOlustur(durum.satirlar[i], s, yolVar));
  parca.append(boslukSatiri(alt, sutunSayisi));
  el.govde.replaceChildren(parca);

  // Satır yüksekliğini gerçek çizimden öğren (yakınlaştırma, yazı tipi farkları).
  const olculen = el.govde.children[1]?.getBoundingClientRect().height;
  if (olculen && Math.abs(olculen - h) > 0.5) {
    v.satirY = olculen;
    cizimIste(true);
  }
  // Sütunlar yalnızca genişleyebilir; kaydırırken tablonun zıplamasını önler.
  [...el.baslik.children].forEach((th, i) => {
    const g = th.getBoundingClientRect().width;
    if (g > (v.genislikler[i] || 0)) {
      v.genislikler[i] = g;
      th.style.minWidth = `${Math.ceil(g)}px`;
    }
  });
}

function sirala(sutun) {
  const artan = durum.siralama?.sutun === sutun ? !durum.siralama.artan : true;
  durum.siralama = { sutun, artan };
  const satirlar = durum.satirlar;
  // Anahtarlar bir kez hesaplanır; karşılaştırma başına tarih ayrıştırmak büyük listede yavaş.
  const anahtar = satirlar.map((r) => siralamaDegeri(r[sutun]));
  const sira = Array.from(satirlar.keys());
  sira.sort((a, b) => {
    const x = anahtar[a];
    const y = anahtar[b];
    if (x === null && y === null) return 0;
    if (x === null) return 1; // boşlar her zaman sonda
    if (y === null) return -1;
    const c = typeof x === 'string' || typeof y === 'string' ? karsilastir(String(x), String(y)) : x - y;
    return artan ? c : -c;
  });
  durum.satirlar = sira.map((i) => satirlar[i]);
  [...el.baslik.children].forEach((th, i) => {
    if (i < durum.sutunlar.length) th.setAttribute('aria-sort', i === sutun ? (artan ? 'ascending' : 'descending') : 'none');
  });
  el.tabloKap.scrollTop = 0;
  cizimIste(true);
}

async function klasordeGoster(yol) {
  try {
    const yanit = await fetch('/api/reveal', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ path: yol }),
    });
    const govde = await yanit.json();
    if (!yanit.ok) throw new Error(govde.hata);
    bildir('Dosya gezgininde açıldı');
  } catch (hata) {
    bildir(hata.message);
  }
}

// ---------- Dışa aktarma ----------
// Sonuçların tamamı tarayıcıdaysa dosya buradan, tablodaki sıralamayla anında üretilir.
// Biçimler sunucunun (fq --format) çıktısıyla aynıdır. Yalnızca sonuç satır sınırını
// aştıysa tamamı için sorgu sunucuda yeniden çalıştırılır.

// Go'nun strconv.FormatFloat(f, 'f', -1, 64) çıktısı: en kısa gösterim, üssüz.
function sayiMetni(x) {
  const m = String(x);
  const u = /^(-?)(\d)(?:\.(\d+))?e([+-]\d+)$/.exec(m);
  if (!u) return m;
  const rakamlar = u[2] + (u[3] || '');
  const nokta = 1 + Number(u[4]);
  if (nokta <= 0) return `${u[1]}0.${'0'.repeat(-nokta)}${rakamlar}`;
  if (nokta >= rakamlar.length) return u[1] + rakamlar + '0'.repeat(nokta - rakamlar.length);
  return `${u[1]}${rakamlar.slice(0, nokta)}.${rakamlar.slice(nokta)}`;
}

const iki = (n) => String(n).padStart(2, '0');

function hamDeger(v) {
  if (v === null || v === undefined) return '';
  if (typeof v === 'boolean') return String(v);
  if (typeof v === 'number') return sayiMetni(v);
  if (typeof v === 'object' && 't' in v) return v.t;
  if (typeof v === 'object' && 'd' in v) return sayiMetni(v.d);
  return String(v);
}

function excelDegeri(v) {
  if (typeof v === 'boolean') return v ? 'evet' : 'hayır';
  if (typeof v === 'number' && !Number.isInteger(v)) return sayiMetni(v).replace('.', ',');
  if (v && typeof v === 'object' && 't' in v) {
    const t = new Date(v.t);
    return `${iki(t.getDate())}.${iki(t.getMonth() + 1)}.${t.getFullYear()} ${iki(t.getHours())}:${iki(t.getMinutes())}:${iki(t.getSeconds())}`;
  }
  return hamDeger(v);
}

// Go encoding/csv kurallarıyla tek alan.
function csvAlani(f, ayrac, crlf) {
  if (f === '') return f;
  if (!(f === '\\.' || f.includes(ayrac) || f.includes('"') || /[\r\n]/.test(f) || /^\s/u.test(f))) return f;
  let q = f.replace(/"/g, '""');
  if (crlf) q = q.replace(/\r/g, '').replace(/\n/g, '\r\n');
  return `"${q}"`;
}

function dosyaUret(bicim) {
  const adlar = durum.sutunlar.map((c) => c.name);
  const parcalar = [];
  if (bicim === 'json') {
    const anahtarlar = adlar.map((a) => JSON.stringify(a));
    parcalar.push('[');
    durum.satirlar.forEach((r, j) => {
      const alanlar = r.map((v, i) => {
        let d = v ?? null;
        if (d && typeof d === 'object') d = 't' in d ? d.t : d.d;
        return `${anahtarlar[i]}: ${JSON.stringify(d)}`;
      });
      parcalar.push(`${j ? ',' : ''}\n  {${alanlar.join(', ')}}`);
    });
    parcalar.push(durum.satirlar.length ? '\n]\n' : ']\n');
    return { parcalar, tur: 'application/json;charset=utf-8', uzanti: 'json' };
  }
  const excel = bicim === 'excel';
  const ayrac = excel ? ';' : ',';
  const son = excel ? '\r\n' : '\n';
  const deger = excel ? excelDegeri : hamDeger;
  const satir = (alanlar) => alanlar.map((f) => csvAlani(f, ayrac, excel)).join(ayrac) + son;
  if (excel) parcalar.push('﻿');
  parcalar.push(satir(adlar));
  for (const r of durum.satirlar) parcalar.push(satir(r.map(deger)));
  return { parcalar, tur: 'text/csv;charset=utf-8', uzanti: 'csv' };
}

function dosyaAdi(uzanti) {
  const t = new Date();
  return `filequery-${t.getFullYear()}${iki(t.getMonth() + 1)}${iki(t.getDate())}-${iki(t.getHours())}${iki(t.getMinutes())}.${uzanti}`;
}

function disaAktar(bicim) {
  const { parcalar, tur, uzanti } = dosyaUret(bicim);
  const url = URL.createObjectURL(new Blob(parcalar, { type: tur }));
  const a = document.createElement('a');
  a.href = url;
  a.download = dosyaAdi(uzanti);
  document.body.append(a);
  a.click();
  a.remove();
  setTimeout(() => URL.revokeObjectURL(url), 60_000);
}

function disaAktarBaglantilari() {
  $$('[data-bicim]').forEach((a) => {
    a.addEventListener('click', (e) => {
      if (!durum.sonSorgu || durum.iptal) {
        e.preventDefault();
        return;
      }
      if (durum.sonuc === 'kesik') {
        // Tamamı tarayıcıda yok: sunucu sorguyu yeniden çalıştırıp dosyayı akıtır.
        a.href = `/api/export?format=${a.dataset.bicim}&q=${encodeURIComponent(durum.sonSorgu)}`;
        bildir('Tüm sonuçlar için sorgu yeniden çalıştırılıyor; dosya hazır olunca inecek.');
        return;
      }
      e.preventDefault();
      disaAktar(a.dataset.bicim);
    });
  });
}

// ---------- Tema ----------

function temaUygula(tema) {
  if (tema) document.documentElement.dataset.tema = tema;
  else delete document.documentElement.dataset.tema;
}

function temaBaslat() {
  const kayitli = oku(KAYIT.tema, null);
  const sistemKoyu = window.matchMedia('(prefers-color-scheme: dark)').matches;
  temaUygula(kayitli || (sistemKoyu ? 'koyu' : null));
  $('#tema').addEventListener('click', () => {
    const koyu = document.documentElement.dataset.tema === 'koyu';
    const yeni = koyu ? 'acik' : 'koyu';
    temaUygula(yeni === 'koyu' ? 'koyu' : null);
    yaz(KAYIT.tema, yeni === 'koyu' ? 'koyu' : 'acik');
  });
}

// ---------- Başlat ----------

function baslat() {
  temaBaslat();
  disaAktarBaglantilari();

  el.sorgu.value = oku(KAYIT.sorgu, '');
  vurguyuGuncelle();

  el.sorgu.addEventListener('input', () => {
    hataKonumu = null;
    vurguyuGuncelle();
    yaz(KAYIT.sorgu, el.sorgu.value);
    if (durum.sekme === 'olusturucu' && durum.uretilenSorgu !== null && el.sorgu.value !== durum.uretilenSorgu) {
      el.sekmeNot.textContent = 'Sorgu elle değiştirildi. Oluşturucuda bir değişiklik yaparsanız onun yerine geçer.';
    }
  });

  $$('[data-sekme]').forEach((b) => b.addEventListener('click', () => sekmeAc(b.dataset.sekme)));
  $$('[data-sistem]').forEach((b) => b.addEventListener('click', () => {
    yaz(KAYIT.sistem, b.dataset.sistem === algilananSistem() ? null : b.dataset.sistem);
    ornekleriOlustur();
    ornekPaneliCiz();
  }));
  sekmeAc(oku(KAYIT.sekme, 'sql') === 'olusturucu' ? 'olusturucu' : 'sql');
  el.sorgu.addEventListener('scroll', () => {
    el.vurgu.parentElement.scrollTop = el.sorgu.scrollTop;
  });
  new ResizeObserver(() => {
    el.vurgu.parentElement.style.height = `${el.sorgu.offsetHeight}px`;
  }).observe(el.sorgu);

  el.tabloKap.addEventListener('scroll', () => cizimIste(), { passive: true });
  new ResizeObserver(() => cizimIste()).observe(el.tabloKap);

  el.calistir.addEventListener('click', calistir);
  el.durdur.addEventListener('click', durdur);
  $$('[data-panel]').forEach((b) => b.addEventListener('click', () => cekmeceAc(b.dataset.panel)));
  $('#cekmece-kapat').addEventListener('click', cekmeceKapat);

  document.addEventListener('keydown', (e) => {
    if ((e.ctrlKey || e.metaKey) && e.key === 'Enter') {
      e.preventDefault();
      calistir();
    } else if (e.key === 'Escape') {
      if (durum.iptal) durdur();
      else if (!el.cekmece.hidden) cekmeceKapat();
    }
  });

  semaYukle();
  el.sorgu.focus();
}

baslat();
