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
  hata: $('#hata'),
  hataKod: $('#hata-kod'),
  hataMesaj: $('#hata-mesaj'),
  durum: $('#durum'),
  uyari: $('#uyari'),
  disaAktar: $('#disa-aktar'),
  tablo: $('#tablo'),
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

const KAYIT = { sorgu: 'fq.sorgu', gecmis: 'fq.gecmis', tema: 'fq.tema' };

const durum = {
  sema: null,
  alanlar: new Set(),
  fonksiyonlar: new Set(),
  sutunlar: [],
  satirlar: [],
  siralama: null, // { sutun, artan }
  iptal: null,
  sonSorgu: '',
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
  const desen = /(--[^\n]*)|('(?:[^']|'')*'?|"(?:[^"]|"")*"?)|(\d+(?:\.\d+)?(?:\s+[KkMmGgTt]?[Bb](?![\p{L}\p{N}_])|[\p{L}]*))|([\p{L}_][\p{L}\p{N}_]*)|(\s+)|([\s\S])/gu;
  let html = '';
  let m;
  while ((m = desen.exec(metin)) !== null) {
    const [parca, yorum, metinSabit, sayiSabit, kelime] = m;
    const baslangic = m.index;
    let sinif = '';
    if (yorum) sinif = 't-yorum';
    else if (metinSabit) sinif = 't-metin';
    else if (sayiSabit) sinif = 't-sayi';
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

const ORNEKLER = [
  ['Büyük dosyalar', "SELECT name, size, modified, dir FROM '~'\nWHERE type = 'file' AND size > 500 MB\nORDER BY size DESC LIMIT 50"],
  ['Son 7 gün', "SELECT name, modified, dir FROM '~'\nWHERE type = 'file' AND modified > NOW() - 7d AND NOT hidden\nORDER BY modified DESC LIMIT 200"],
  ['Türlere göre yer', "SELECT ext, COUNT(*) AS adet, SUM(size) AS toplam FROM '~'\nWHERE type = 'file'\nGROUP BY ext ORDER BY toplam DESC LIMIT 20"],
  ['PDF ara', "SELECT name, size, modified, dir FROM '~'\nWHERE ext = 'pdf' AND name LIKE '%fatura%'"],
  ['Fotoğraflar (yıllara göre)', "SELECT YEAR(modified) AS yil, COUNT(*) AS adet, SUM(size) AS toplam FROM '~'\nWHERE ext IN ('jpg', 'jpeg', 'png', 'heic')\nGROUP BY YEAR(modified) ORDER BY yil"],
];

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

  vurguyuGuncelle();
}

function ornekleriOlustur() {
  el.ornekler.replaceChildren(...ORNEKLER.map(([ad, sorgu]) => {
    const b = document.createElement('button');
    b.type = 'button';
    b.className = 'ornek';
    b.textContent = ad;
    b.title = sorgu.replace(/\n/g, ' ');
    b.addEventListener('click', () => {
      sorguyuAyarla(sorgu);
      el.sorgu.focus();
    });
    return b;
  }));
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
  el.cekmeceBaslik.textContent = ad === 'alanlar' ? 'Alanlar ve fonksiyonlar' : 'Geçmiş';
  $('#cekmece-alanlar').hidden = ad !== 'alanlar';
  $('#cekmece-gecmis').hidden = ad !== 'gecmis';
  if (ad === 'gecmis') gecmisiGoster();
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
      sorguyuAyarla(g.sorgu);
      cekmeceKapat();
      el.sorgu.focus();
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
  el.govde.replaceChildren();
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
      durumYaz(`Durduruldu. <strong>${sayi.format(durum.satirlar.length)}</strong> sonuç gösteriliyor.`);
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
    const bas = durum.satirlar.length;
    durum.satirlar.push(...m.rows);
    satirlariEkle(bas);
  } else if (m.progress) {
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
  let html = d.rows === 0 ? 'Sonuç yok. ' : `<strong>${sayi.format(d.rows)}</strong> sonuç, `;
  html += `${sayi.format(d.scanned)} öğe tarandı, ${sn} sn`;
  if (d.denied > 0) {
    const liste = (d.deniedPaths || []).join('\n');
    html += `; <span class="erisilemeyen" title="${kacis(liste)}">erişim izni olmayan ${sayi.format(d.denied)} klasör atlandı</span>`;
  }
  durumYaz(html);
  el.disaAktar.hidden = d.rows === 0;
  if (d.truncated) {
    el.uyari.textContent = `İlk ${sayi.format(d.maxRows)} sonuç gösteriliyor. Tamamını almak için dışa aktarın ya da sorguyu daraltın (WHERE, LIMIT).`;
    el.uyari.hidden = false;
  }
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
    b.addEventListener('click', () => sirala(i));
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

function satirlariEkle(bas) {
  const s = yolSutunlari();
  const yolVar = s.yol >= 0 || (s.klasor >= 0 && s.ad >= 0);
  const parca = document.createDocumentFragment();
  for (let i = bas; i < durum.satirlar.length; i++) {
    parca.append(satirOlustur(durum.satirlar[i], s, yolVar));
  }
  el.govde.append(parca);
}

function sirala(sutun) {
  const artan = durum.siralama?.sutun === sutun ? !durum.siralama.artan : true;
  durum.siralama = { sutun, artan };
  durum.satirlar.sort((a, b) => {
    const x = siralamaDegeri(a[sutun]);
    const y = siralamaDegeri(b[sutun]);
    if (x === null && y === null) return 0;
    if (x === null) return 1; // boşlar her zaman sonda
    if (y === null) return -1;
    const c = typeof x === 'string' || typeof y === 'string' ? karsilastir(String(x), String(y)) : x - y;
    return artan ? c : -c;
  });
  [...el.baslik.children].forEach((th, i) => {
    if (i < durum.sutunlar.length) th.setAttribute('aria-sort', i === sutun ? (artan ? 'ascending' : 'descending') : 'none');
  });
  el.govde.replaceChildren();
  satirlariEkle(0);
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

function disaAktarBaglantilari() {
  $$('[data-bicim]').forEach((a) => {
    a.addEventListener('click', (e) => {
      if (!durum.sonSorgu) {
        e.preventDefault();
        return;
      }
      a.href = `/api/export?format=${a.dataset.bicim}&q=${encodeURIComponent(durum.sonSorgu)}`;
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
  ornekleriOlustur();
  disaAktarBaglantilari();

  el.sorgu.value = oku(KAYIT.sorgu, ORNEKLER[0][1]);
  vurguyuGuncelle();

  el.sorgu.addEventListener('input', () => {
    hataKonumu = null;
    vurguyuGuncelle();
    yaz(KAYIT.sorgu, el.sorgu.value);
  });
  el.sorgu.addEventListener('scroll', () => {
    el.vurgu.parentElement.scrollTop = el.sorgu.scrollTop;
  });
  new ResizeObserver(() => {
    el.vurgu.parentElement.style.height = `${el.sorgu.offsetHeight}px`;
  }).observe(el.sorgu);

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
