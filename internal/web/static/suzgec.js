// FileQuery sonuç tablosu için sütun süzgeçleri.
// suzgecDerle(metin, tur) yazılan ifadeyi, satır değeri alıp true/false dönen bir işleve çevirir.
// Türler: 'metin', 'sayi', 'boyut', 'tarih', 'sure', 'mantik'.
//
// Ortak kurallar:
//   a | b        a veya b
//   !a           a olmayanlar (birden çok ! koşulunun hiçbiri tutmamalı)
// Metin:   pdf (içerir), =rapor.pdf (tam), ^fatura (ile başlar), .pdf$ (ile biter), fatura*2024 (joker * ?)
// Sıralı türler (sayı, boyut, tarih, süre):
//   5   =5   >5   >=5   <5   <=5   5..10 (aralık, iki ucu dahil)   !5
// Boyut: 500 KB, 1,5 GB (birimsiz sayı bayttır; birimli değer tablodaki yuvarlamayla eşleşir)
// Tarih: 2024, 03.2024, 15.03.2024, 15.03.2024 14:30, 2024-03-15, bugün, dün, son 7g
// Süre:  7g, 12sa, 30dk, 45sn, 1g 6sa
'use strict';

const SUZGEC_BOYUT = { b: 1, kb: 1024, mb: 1024 ** 2, gb: 1024 ** 3, tb: 1024 ** 4, pb: 1024 ** 5 };
const SUZGEC_SURE = {
  sn: 1, s: 1, saniye: 1, dk: 60, m: 60, dakika: 60, sa: 3600, h: 3600, saat: 3600,
  g: 86400, d: 86400, gün: 86400, gun: 86400, hf: 604800, w: 604800, hafta: 604800,
};

class SuzgecHatasi extends Error {}

const kucuk = (s) => s.toLocaleLowerCase('tr');

// Satır değerini karşılaştırılabilir sayıya çevirir (tarih: ms, süre: sn).
function suzgecDegeri(v) {
  if (v === null || v === undefined) return null;
  if (typeof v === 'object' && 't' in v) {
    if (v.ms === undefined) v.ms = Date.parse(v.t);
    return v.ms;
  }
  if (typeof v === 'object' && 'd' in v) return v.d;
  if (typeof v === 'boolean') return v ? 1 : 0;
  return v;
}

// Bir değerin süzgeç türü; boş değer için null.
function degerTuru(v, boyutMu) {
  if (v === null || v === undefined) return null;
  if (typeof v === 'boolean') return 'mantik';
  if (typeof v === 'number') return boyutMu ? 'boyut' : 'sayi';
  if (typeof v === 'object' && 't' in v) return 'tarih';
  if (typeof v === 'object' && 'd' in v) return 'sure';
  return 'metin';
}

// "1.234,5" "1,5" "1.5" "1.500" → sayı ve ondalık hane sayısı.
function sayiCoz(s) {
  let t = s.replace(/\s/g, '');
  if (!/^-?[\d.,]+$/.test(t)) return null;
  if (t.includes(',')) t = t.replace(/\./g, '').replace(',', '.');
  else if (/^-?\d{1,3}(\.\d{3})+$/.test(t)) t = t.replace(/\./g, '');
  if (!/^-?\d+(\.\d+)?$/.test(t)) return null;
  const ondalik = t.includes('.') ? t.split('.')[1].length : 0;
  return { x: Number(t), ondalik };
}

// Sayı + isteğe bağlı birim → [alt, üst) aralığı. Değer yazıldığı hassasiyetle eşleşir:
// "1,5 MB" tabloda 1,5 MB görünen her şeyi, "100" yalnızca 100'ü kapsar.
function birimliAralik(s, birimler, varsayilanBirim, ad) {
  const m = /^(-?[\d.,\s]*\d)\s*([a-zçğıöşü]*)$/i.exec(s.trim());
  if (!m) throw new SuzgecHatasi(`"${s}" ${ad} olarak okunamadı`);
  const n = sayiCoz(m[1]);
  if (!n) throw new SuzgecHatasi(`"${m[1]}" sayı değil`);
  const birimAdi = kucuk(m[2]);
  const birim = birimAdi ? birimler[birimAdi] : varsayilanBirim;
  if (!birim) {
    throw new SuzgecHatasi(birimAdi ? `bilinmeyen birim "${m[2]}"` : `birim yazın, ör. ${ad === 'süre' ? '7g, 12sa, 30dk' : '500 KB'}`);
  }
  const x = n.x * birim;
  const adim = birim * 10 ** -n.ondalik;
  return [x - adim / 2, x + adim / 2, x];
}

function sureAraligi(s) {
  const parcalar = s.trim().split(/\s+(?=-?[\d.,]+\s*[a-zçğıöşü])/i);
  if (parcalar.length === 1) return birimliAralik(s, SUZGEC_SURE, 0, 'süre');
  let toplam = 0;
  // "1g 6sa": toplam, en küçük birimin hassasiyetiyle
  let alt = 0;
  let ust = 0;
  let enKucuk = Infinity;
  for (const p of parcalar) {
    const [a, u, x] = birimliAralik(p, SUZGEC_SURE, 0, 'süre');
    toplam += x;
    alt += x;
    ust += x;
    enKucuk = Math.min(enKucuk, u - a);
  }
  return [alt - enKucuk / 2, ust + enKucuk / 2, toplam];
}

function tarihAraligi(s, simdi) {
  const t = kucuk(s.trim());
  const gunBasi = new Date(simdi);
  gunBasi.setHours(0, 0, 0, 0);
  if (t === 'bugün' || t === 'bugun') return [gunBasi.getTime(), gunBasi.getTime() + 86400000];
  if (t === 'dün' || t === 'dun') {
    const d = new Date(gunBasi);
    d.setDate(d.getDate() - 1);
    return [d.getTime(), gunBasi.getTime()];
  }
  let m = /^son\s+(.+)$/.exec(t);
  if (m) {
    return [simdi - sureAraligi(m[1])[2] * 1000, Infinity];
  }
  const yap = (y, ay, g, sa, dk) => new Date(y, ay - 1, g, sa, dk).getTime();
  m = /^(\d{1,2})[./](\d{1,2})[./](\d{4})(?:\s+(\d{1,2})[:.](\d{2}))?$/.exec(t);
  if (m) {
    const [g, ay, y] = [+m[1], +m[2], +m[3]];
    if (ay < 1 || ay > 12 || g < 1 || g > 31) throw new SuzgecHatasi(`"${s}" geçerli bir tarih değil`);
    if (m[4] !== undefined) return [yap(y, ay, g, +m[4], +m[5]), yap(y, ay, g, +m[4], +m[5] + 1)];
    return [yap(y, ay, g, 0, 0), yap(y, ay, g + 1, 0, 0)];
  }
  m = /^(\d{4})-(\d{1,2})(?:-(\d{1,2}))?$/.exec(t);
  if (m) {
    if (m[3] !== undefined) return [yap(+m[1], +m[2], +m[3], 0, 0), yap(+m[1], +m[2], +m[3] + 1, 0, 0)];
    return [yap(+m[1], +m[2], 1, 0, 0), yap(+m[1], +m[2] + 1, 1, 0, 0)];
  }
  m = /^(\d{1,2})[./](\d{4})$/.exec(t);
  if (m) {
    if (+m[1] < 1 || +m[1] > 12) throw new SuzgecHatasi(`"${s}" geçerli bir ay değil`);
    return [yap(+m[2], +m[1], 1, 0, 0), yap(+m[2], +m[1] + 1, 1, 0, 0)];
  }
  m = /^(\d{4})$/.exec(t);
  if (m) return [yap(+m[1], 1, 1, 0, 0), yap(+m[1] + 1, 1, 1, 0, 0)];
  throw new SuzgecHatasi(`"${s}" tarih olarak okunamadı; ör. 2024, 03.2024, 15.03.2024, son 7g`);
}

function aralikOkuyucu(tur, simdi) {
  switch (tur) {
    case 'sayi': return (s) => birimliAralik(s, {}, 1, 'sayı');
    case 'boyut': return (s) => birimliAralik(s, SUZGEC_BOYUT, 1, 'boyut');
    case 'sure': return sureAraligi;
    case 'tarih': return (s) => tarihAraligi(s, simdi);
  }
  return null;
}

// Tek bir sıralı koşul: karşılaştırma, aralık veya eşitlik.
// Sayı, boyut ve süre: eşitlik yazılan hassasiyetle ("1,5 MB" tabloda 1,5 MB görünen her şey),
// karşılaştırma ve aralık tam değerle (">1 GB" 1 GB'tan büyük olan her şey).
// Tarih: yazılan dönemle (">2024" 2024'ten sonrası, "<=03.2024" mart sonuna kadar).
function siraliKosul(ifade, oku) {
  let m = /^(>=|<=|<>|!=|>|<|=)\s*(.+)$/.exec(ifade);
  if (m) {
    const [a, u, x] = oku(m[2]);
    if (x !== undefined && m[1] !== '=' && m[1] !== '<>' && m[1] !== '!=') {
      switch (m[1]) {
        case '>': return (v) => v > x;
        case '>=': return (v) => v >= x;
        case '<': return (v) => v < x;
        default: return (v) => v <= x;
      }
    }
    switch (m[1]) {
      case '>': return (v) => v >= u;
      case '>=': return (v) => v >= a;
      case '<': return (v) => v < a;
      case '<=': return (v) => v < u;
      case '=': return (v) => v >= a && v < u;
      default: return (v) => v < a || v >= u;
    }
  }
  m = /^(.+?)\s*\.\.\s*(.+)$/.exec(ifade);
  if (m) {
    const [a, , xa] = oku(m[1]);
    const [, u, xu] = oku(m[2]);
    if (xa !== undefined) {
      if (xu < xa) throw new SuzgecHatasi('aralığın başı sonundan büyük olamaz');
      return (v) => v >= xa && v <= xu;
    }
    if (u <= a) throw new SuzgecHatasi('aralığın başı sonundan büyük olamaz');
    return (v) => v >= a && v < u;
  }
  const [a, u] = oku(ifade);
  return (v) => v >= a && v < u;
}

function metinKosulu(ifade) {
  const t = kucuk(ifade);
  if (t.startsWith('=')) {
    const h = t.slice(1);
    return (v) => v === h;
  }
  if (/[*?]/.test(t)) {
    const kalip = t.replace(/[.+^${}()|[\]\\]/g, '\\$&').replace(/\*/g, '.*').replace(/\?/g, '.');
    const re = new RegExp(`^${kalip}$`, 's');
    return (v) => re.test(v);
  }
  const bas = t.startsWith('^');
  const son = t.length > 1 && t.endsWith('$');
  const h = t.slice(bas ? 1 : 0, son ? -1 : undefined);
  if (bas && son) return (v) => v === h;
  if (bas) return (v) => v.startsWith(h);
  if (son) return (v) => v.endsWith(h);
  return (v) => v.includes(h);
}

// Süzgeç metnini derler. Boş metin için null, hatalıysa { hata } döner.
function suzgecDerle(metin, tur, simdi = Date.now()) {
  const ham = metin.trim();
  if (!ham) return null;
  if (tur === 'mantik') {
    const t = kucuk(ham);
    if (['evet', 'true', '1', 'e'].includes(t)) return { f: (v) => v === true };
    if (['hayır', 'hayir', 'false', '0', 'h'].includes(t)) return { f: (v) => v === false };
    return { hata: 'evet veya hayır yazın' };
  }
  try {
    const olumlu = [];
    const olumsuz = [];
    const oku = aralikOkuyucu(tur, simdi);
    for (const parca of ham.split('|')) {
      let p = parca.trim();
      if (!p) continue;
      let degil = false;
      if (!oku && p.startsWith('!=')) {
        degil = true;
        p = `=${p.slice(2).trim()}`;
      } else if (p.startsWith('!') && !p.startsWith('!=')) {
        degil = true;
        p = p.slice(1).trim();
        if (!p) continue;
      }
      const f = oku ? siraliKosul(p, oku) : metinKosulu(p);
      (degil ? olumsuz : olumlu).push(f);
    }
    if (!olumlu.length && !olumsuz.length) return null;
    const cevir = oku
      ? (v) => suzgecDegeri(v)
      : (v) => (v === null || v === undefined ? null : kucuk(typeof v === 'object' ? JSON.stringify(v) : String(v)));
    return {
      f: (ham2) => {
        const v = cevir(ham2);
        if (v === null) return olumlu.length === 0;
        if (olumlu.length && !olumlu.some((f) => f(v))) return false;
        return !olumsuz.some((f) => f(v));
      },
    };
  } catch (e) {
    if (e instanceof SuzgecHatasi) return { hata: e.message };
    throw e;
  }
}

const SUZGEC_IPUCU = {
  metin: 'İçerir: pdf · tam: =rapor.pdf · başlar: ^fatura · biter: .pdf$ · joker: fatura*2024 · değil: !tmp · veya: pdf | docx',
  sayi: 'Eşit: 5 · büyük: >5 · küçük eşit: <=10 · aralık: 5..10 · değil: !0 · veya: 1 | 2',
  boyut: 'Büyük: >10 MB · küçük: <500 KB · aralık: 1 GB..2 GB · görünen değer: 1,5 MB · birimsiz sayı bayttır',
  tarih: 'Yıl: 2024 · ay: 03.2024 · gün: 15.03.2024 · sonra: >01.06.2024 · aralık: 01.01.2024..31.03.2024 · son 7g · bugün · dün',
  sure: 'Büyük: >7g · küçük: <12sa · aralık: 1g..3g · birimler: g, sa, dk, sn',
  mantik: 'evet veya hayır',
};
const SUZGEC_ORNEK = {
  metin: 'ara…', sayi: '> 10', boyut: '> 10 MB', tarih: '> 01.01.2024', sure: '> 7g', mantik: '',
};

if (typeof module !== 'undefined') module.exports = { suzgecDerle, degerTuru, sayiCoz };
