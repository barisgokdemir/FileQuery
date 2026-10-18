// FileQuery sorgu oluşturucu: formdan SQL üretir.
// sqlUret() DOM'a dokunmaz; arayüz kur() ile oluşturulur.
'use strict';

(function () {
  // ---------- Model ----------

  const TURLER = {
    name: 'metin', ext: 'uzanti', path: 'metin', dir: 'metin', owner: 'metin', group: 'metin', mode: 'metin',
    size: 'boyut', modified: 'tarih', created: 'tarih', accessed: 'tarih', depth: 'sayi', links: 'sayi',
    hidden: 'mantik', readonly: 'mantik', system: 'mantik', type: 'metin',
  };

  const ETIKETLER = {
    name: 'Ad', ext: 'Uzantı', path: 'Tam yol', dir: 'Klasör', type: 'Tür', size: 'Boyut',
    modified: 'Değiştirilme tarihi', created: 'Oluşturulma / geliş tarihi', accessed: 'Son erişim',
    depth: 'Derinlik', hidden: 'Gizli', readonly: 'Salt okunur', system: 'Sistem dosyası',
    owner: 'Sahip', group: 'Grup', mode: 'İzinler', links: 'Hardlink sayısı',
  };

  // Koşul satırında seçilebilecek alanlar (tür ayrı bir seçimle belirlendiği için type yok)
  const KOSUL_ALANLARI = ['name', 'ext', 'size', 'modified', 'created', 'accessed', 'dir', 'path', 'hidden',
    'readonly', 'system', 'depth', 'owner', 'group', 'mode', 'links'];

  const OPERATORLER = {
    metin: [['icerir', 'içerir'], ['baslar', 'ile başlar'], ['biter', 'ile biter'], ['esit', 'eşittir'],
      ['esitdegil', 'eşit değildir'], ['liste', 'şunlardan biri'], ['regex', 'düzenli ifadeye uyar']],
    uzanti: [['liste', 'şunlardan biri'], ['listedegil', 'şunlardan biri değil']],
    boyut: [['buyuk', 'büyüktür'], ['kucuk', 'küçüktür'], ['arasinda', 'arasında'], ['sifir', 'boş (0 bayt)']],
    tarih: [['son', 'son … içinde'], ['eski', '… önceden eski'], ['bugun', 'bugün'], ['sonra', 'şu tarihten sonra'],
      ['once', 'şu tarihten önce'], ['arasinda', 'iki tarih arasında'], ['yil', 'şu yılda']],
    sayi: [['=', 'eşittir'], ['>', 'büyüktür'], ['<', 'küçüktür'], ['>=', 'büyük veya eşit'], ['<=', 'küçük veya eşit']],
    mantik: [['evet', 'evet'], ['hayir', 'hayır']],
  };

  const BOYUT_BIRIMLERI = ['KB', 'MB', 'GB', 'TB', 'B'];
  const SURE_BIRIMLERI = [['gun', 'gün'], ['hafta', 'hafta'], ['ay', 'ay'], ['yil', 'yıl'], ['saat', 'saat']];

  const GRUPLAR = [
    ['', 'Gruplama yok'],
    ['ext', 'Uzantıya göre'],
    ['dir', 'Klasöre göre'],
    ['YEAR(modified)', 'Yıla göre (değiştirilme)'],
    ['YEAR(created)', 'Yıla göre (geliş)'],
    ['type', 'Türe göre'],
    ['owner', 'Sahibe göre'],
  ];
  const GRUP_ADI = { 'YEAR(modified)': 'yil', 'YEAR(created)': 'yil' };

  const DERINLIKLER = [[0, 'Tüm alt klasörler'], [1, 'Sadece bu klasör'], [2, '2 seviye'], [3, '3 seviye'], [5, '5 seviye']];

  const VARSAYILAN_SUTUNLAR = ['name', 'size', 'modified', 'dir'];

  function varsayilanDurum(kaynak) {
    return {
      kaynak: kaynak || '~',
      derinlik: 0,
      tur: 'file',
      baglac: 'AND',
      kosullar: [{ alan: 'ext', op: 'liste', deger: 'pdf' }],
      sutunlar: [...VARSAYILAN_SUTUNLAR],
      grup: '',
      sirala: 'modified',
      yon: 'DESC',
      limit: '',
    };
  }

  function varsayilanKosul(alan) {
    const op = OPERATORLER[TURLER[alan]][0][0];
    const k = { alan, op, deger: '', deger2: '' };
    if (TURLER[alan] === 'boyut') k.birim = 'MB';
    if (TURLER[alan] === 'tarih') k.birim = 'gun';
    return k;
  }

  // ---------- SQL üretimi ----------

  const q = (metin) => `'${String(metin).replace(/'/g, "''")}'`;
  const reKacis = (s) => s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');

  function sayiOku(v, tamSayi) {
    const s = String(v ?? '').trim().replace(',', '.');
    if (!/^\d+(\.\d+)?$/.test(s)) return null;
    const n = Number(s);
    if (!Number.isFinite(n) || (tamSayi && !Number.isInteger(n))) return null;
    return s;
  }

  function listeOku(v, uzantiMi) {
    return String(v ?? '')
      .split(uzantiMi ? /[,;\s]+/ : /,/)
      .map((x) => x.trim())
      .map((x) => (uzantiMi ? x.replace(/^\*?\.+/, '').toLowerCase() : x))
      .filter(Boolean);
  }

  function sure(n, birim) {
    const k = Number(n);
    switch (birim) {
      case 'saat': return `${k}h`;
      case 'hafta': return `${k}w`;
      case 'ay': return `${k * 30}d`;
      case 'yil': return `${k * 365}d`;
      default: return `${k}d`;
    }
  }

  const TARIH = /^\d{4}-\d{2}-\d{2}$/;

  // Bir koşulu SQL'e çevirir; eksik doldurulmuşsa null döner.
  function kosulSql(k) {
    const f = k.alan;
    const tur = TURLER[f];
    const v = String(k.deger ?? '').trim();

    switch (tur) {
      case 'metin': {
        if (k.op === 'liste') {
          const l = listeOku(v, false);
          return l.length ? `${f} IN (${l.map(q).join(', ')})` : null;
        }
        if (!v) return null;
        // LIKE'ın joker karakterleri metinde geçiyorsa harf duyarsız düzenli ifadeye geç
        const joker = /[%_]/.test(v);
        switch (k.op) {
          case 'icerir': return joker ? `${f} MATCHES ${q(`(?i)${reKacis(v)}`)}` : `${f} LIKE ${q(`%${v}%`)}`;
          case 'baslar': return joker ? `${f} MATCHES ${q(`(?i)^${reKacis(v)}`)}` : `${f} LIKE ${q(`${v}%`)}`;
          case 'biter': return joker ? `${f} MATCHES ${q(`(?i)${reKacis(v)}$`)}` : `${f} LIKE ${q(`%${v}`)}`;
          case 'esit': return `${f} = ${q(v)}`;
          case 'esitdegil': return `${f} != ${q(v)}`;
          case 'regex': return `${f} MATCHES ${q(v)}`;
        }
        return null;
      }
      case 'uzanti': {
        const l = listeOku(v, true);
        if (!l.length) return null;
        if (k.op === 'listedegil') return l.length === 1 ? `${f} != ${q(l[0])}` : `${f} NOT IN (${l.map(q).join(', ')})`;
        return l.length === 1 ? `${f} = ${q(l[0])}` : `${f} IN (${l.map(q).join(', ')})`;
      }
      case 'boyut': {
        if (k.op === 'sifir') return `${f} = 0`;
        const b = BOYUT_BIRIMLERI.includes(k.birim) ? k.birim : 'MB';
        const a = sayiOku(k.deger, false);
        if (a === null) return null;
        if (k.op === 'arasinda') {
          const c = sayiOku(k.deger2, false);
          return c === null ? null : `${f} BETWEEN ${a} ${b} AND ${c} ${b}`;
        }
        return `${f} ${k.op === 'kucuk' ? '<' : '>'} ${a} ${b}`;
      }
      case 'tarih': {
        switch (k.op) {
          case 'bugun': return `${f} >= TODAY()`;
          case 'son':
          case 'eski': {
            const n = sayiOku(k.deger, true);
            if (n === null || Number(n) <= 0) return null;
            return `${f} ${k.op === 'son' ? '>' : '<'} NOW() - ${sure(n, k.birim)}`;
          }
          case 'sonra': return TARIH.test(v) ? `${f} >= ${q(v)}` : null;
          case 'once': return TARIH.test(v) ? `${f} < ${q(v)}` : null;
          case 'arasinda': {
            const v2 = String(k.deger2 ?? '').trim();
            return TARIH.test(v) && TARIH.test(v2) ? `${f} BETWEEN ${q(`${v} 00:00:00`)} AND ${q(`${v2} 23:59:59`)}` : null;
          }
          case 'yil': {
            const n = sayiOku(k.deger, true);
            return n === null ? null : `YEAR(${f}) = ${n}`;
          }
        }
        return null;
      }
      case 'sayi': {
        const n = sayiOku(k.deger, true);
        const op = ['=', '>', '<', '>=', '<='].includes(k.op) ? k.op : '=';
        return n === null ? null : `${f} ${op} ${n}`;
      }
      case 'mantik':
        return k.op === 'hayir' ? `NOT ${f}` : f;
    }
    return null;
  }

  // Durumdan SQL üretir. { sql, eksikler } döner; sql null ise üretilemedi.
  function sqlUret(d) {
    const kaynak = String(d.kaynak ?? '').trim();
    if (!kaynak) return { sql: null, eksikler: [], mesaj: 'Aranacak klasörü seçin.' };

    const eksikler = [];
    const kosullar = [];
    (d.kosullar || []).forEach((k, i) => {
      const s = kosulSql(k);
      if (s) kosullar.push(s);
      else eksikler.push(i);
    });

    const where = [];
    if (d.tur === 'file') where.push("type = 'file'");
    else if (d.tur === 'dir') where.push("type = 'dir'");
    if (kosullar.length) {
      if (d.baglac === 'OR' && kosullar.length > 1) where.push(`(${kosullar.join(' OR ')})`);
      else where.push(...kosullar);
    }

    const satirlar = [];
    let sirala = '';
    if (d.grup) {
      const ad = GRUP_ADI[d.grup];
      satirlar.push(`SELECT ${d.grup}${ad ? ` AS ${ad}` : ''}, COUNT(*) AS adet, SUM(size) AS toplam`);
      const anahtar = ad || d.grup;
      const gecerli = ['adet', 'toplam', anahtar];
      const s = gecerli.includes(d.sirala) ? d.sirala : 'toplam';
      sirala = `${s} ${d.yon === 'ASC' ? 'ASC' : 'DESC'}`;
    } else {
      const sutunlar = (d.sutunlar || []).filter((s) => s in TURLER);
      satirlar.push(`SELECT ${(sutunlar.length ? sutunlar : ['name']).join(', ')}`);
      if (d.sirala && d.sirala in TURLER) sirala = `${d.sirala} ${d.yon === 'ASC' ? 'ASC' : 'DESC'}`;
    }

    const derinlik = Number(d.derinlik) > 0 ? ` DEPTH ${Number(d.derinlik)}` : '';
    satirlar[0] += ` FROM ${q(kaynak)}${derinlik}`;
    if (where.length) satirlar.push(`WHERE ${where.join(' AND ')}`);
    if (d.grup) satirlar.push(`GROUP BY ${d.grup}`);
    if (sirala) satirlar.push(`ORDER BY ${sirala}`);
    const limit = sayiOku(d.limit, true);
    if (limit !== null && Number(limit) > 0) satirlar.push(`LIMIT ${limit}`);

    return { sql: satirlar.join('\n'), eksikler };
  }

  // ---------- Arayüz ----------

  function h(etiket, nitelikler = {}, ...cocuklar) {
    const e = document.createElement(etiket);
    for (const [k, v] of Object.entries(nitelikler)) {
      if (v === undefined || v === null || v === false) continue;
      if (k === 'class') e.className = v;
      else if (k.startsWith('on')) e.addEventListener(k.slice(2), v);
      else if (k === 'value') e.value = v;
      else if (k === 'checked') e.checked = v;
      else e.setAttribute(k, v === true ? '' : v);
    }
    for (const c of cocuklar.flat()) {
      if (c !== null && c !== undefined && c !== false) e.append(c);
    }
    return e;
  }

  function secim(secenekler, deger, degisince, nitelikler = {}) {
    const s = h('select', nitelikler,
      secenekler.map(([v, ad]) => h('option', { value: v }, ad)));
    s.value = String(deger);
    s.addEventListener('change', () => degisince(s.value));
    return s;
  }

  // kok: yerleşim kabı; ayarlar: { yerler, kaydet(durum), sqlDegisti(sql), dogrula(sql) -> Promise }
  function kur(kok, ayarlar) {
    let d = ayarlar.durum || varsayilanDurum(ayarlar.varsayilanKaynak);
    let digerKlasor = !ayarlar.yerler.some((y) => y.path === d.kaynak);

    function degisti(yenidenCiz = false) {
      ayarlar.kaydet(d);
      if (yenidenCiz) ciz();
      else uret();
    }

    let dogrulamaZamani = null;
    const durumSatiri = h('p', { class: 'olusturucu-durum', 'aria-live': 'polite' });

    function uret() {
      const { sql, eksikler, mesaj } = sqlUret(d);
      kok.querySelectorAll('.kosul').forEach((satir, i) => satir.classList.toggle('eksik', eksikler.includes(i)));
      if (!sql) {
        durumSatiri.className = 'olusturucu-durum uyari';
        durumSatiri.textContent = mesaj;
        return;
      }
      ayarlar.sqlDegisti(sql);
      durumSatiri.className = 'olusturucu-durum';
      durumSatiri.textContent = eksikler.length ? `${eksikler.length} koşul eksik olduğu için sorguya eklenmedi.` : 'Denetleniyor…';
      clearTimeout(dogrulamaZamani);
      dogrulamaZamani = setTimeout(async () => {
        const sonuc = await ayarlar.dogrula(sql);
        if (sonuc.ok) {
          durumSatiri.className = 'olusturucu-durum gecerli';
          durumSatiri.textContent = eksikler.length
            ? `Sorgu geçerli; ${eksikler.length} eksik koşul dahil edilmedi.`
            : 'Sorgu geçerli. Çalıştırmak için Ctrl+Enter.';
        } else {
          durumSatiri.className = 'olusturucu-durum hatali';
          durumSatiri.textContent = sonuc.mesaj;
        }
      }, 250);
    }

    function kaynakAlani() {
      const secenekler = ayarlar.yerler.map((y) => [y.path, `${y.name} — ${y.path}`]);
      secenekler.push(['__diger', 'Başka bir klasör…']);
      const sec = secim(secenekler, digerKlasor ? '__diger' : d.kaynak, (v) => {
        if (v === '__diger') {
          digerKlasor = true;
          degisti(true);
          kok.querySelector('.kaynak-girdi')?.focus();
          return;
        }
        digerKlasor = false;
        d.kaynak = v;
        degisti(true);
      }, { 'aria-label': 'Klasör', class: 'kaynak-secim' });

      const girdi = digerKlasor
        ? h('input', {
          type: 'text', class: 'kaynak-girdi', value: d.kaynak, spellcheck: 'false',
          placeholder: "C:\\Projeler veya ~/Belgeler", 'aria-label': 'Klasör yolu',
          oninput: (e) => { d.kaynak = e.target.value; degisti(); },
        })
        : null;

      const derinlik = secim(DERINLIKLER, d.derinlik, (v) => { d.derinlik = Number(v); degisti(); }, { 'aria-label': 'Derinlik' });
      const tur = secim([['file', 'Dosyalar'], ['dir', 'Klasörler'], ['all', 'Dosya ve klasörler']], d.tur,
        (v) => { d.tur = v; degisti(); }, { 'aria-label': 'Ne aranıyor' });

      return bolum('Nerede', h('div', { class: 'satir-grup' }, sec, girdi, derinlik, tur));
    }

    function degerAlanlari(k, i) {
      const tur = TURLER[k.alan];
      const guncelle = (alan) => (e) => { k[alan] = e.target.value; degisti(); };
      const metinGirdi = (alan, yer) => h('input', {
        type: 'text', value: k[alan] ?? '', placeholder: yer, spellcheck: 'false',
        'aria-label': 'Değer', oninput: guncelle(alan),
      });
      const sayiGirdi = (alan, yer) => h('input', {
        type: 'text', inputmode: 'decimal', class: 'kisa', value: k[alan] ?? '', placeholder: yer,
        'aria-label': 'Sayı', oninput: guncelle(alan),
      });
      const tarihGirdi = (alan) => h('input', { type: 'date', value: k[alan] ?? '', 'aria-label': 'Tarih', oninput: guncelle(alan) });

      switch (tur) {
        case 'metin':
          return [metinGirdi('deger', k.op === 'liste' ? 'virgülle ayırın: a, b' : k.op === 'regex' ? '^rapor-\\d+' : 'metin')];
        case 'uzanti':
          return [metinGirdi('deger', 'pdf, docx, xlsx')];
        case 'boyut': {
          if (k.op === 'sifir') return [];
          const birim = secim(BOYUT_BIRIMLERI.map((b) => [b, b]), k.birim || 'MB', (v) => { k.birim = v; degisti(); }, { 'aria-label': 'Birim' });
          return k.op === 'arasinda'
            ? [sayiGirdi('deger', '10'), h('span', { class: 'ara-yazi' }, 've'), sayiGirdi('deger2', '100'), birim]
            : [sayiGirdi('deger', '100'), birim];
        }
        case 'tarih':
          switch (k.op) {
            case 'son':
            case 'eski':
              return [sayiGirdi('deger', '7'),
                secim(SURE_BIRIMLERI, k.birim || 'gun', (v) => { k.birim = v; degisti(); }, { 'aria-label': 'Süre birimi' })];
            case 'sonra':
            case 'once':
              return [tarihGirdi('deger')];
            case 'arasinda':
              return [tarihGirdi('deger'), h('span', { class: 'ara-yazi' }, 've'), tarihGirdi('deger2')];
            case 'yil':
              return [sayiGirdi('deger', String(new Date().getFullYear()))];
          }
          return [];
        case 'sayi':
          return [sayiGirdi('deger', '1')];
      }
      return [];
    }

    function kosulSatiri(k, i) {
      const alan = secim(KOSUL_ALANLARI.map((a) => [a, ETIKETLER[a]]), k.alan, (v) => {
        d.kosullar[i] = varsayilanKosul(v);
        degisti(true);
      }, { 'aria-label': 'Alan' });
      const op = secim(OPERATORLER[TURLER[k.alan]], k.op, (v) => {
        k.op = v;
        if (TURLER[k.alan] === 'tarih') { k.deger = ''; k.deger2 = ''; }
        degisti(true);
      }, { 'aria-label': 'Koşul' });
      const sil = h('button', {
        type: 'button', class: 'kosul-sil', title: 'Koşulu kaldır', 'aria-label': 'Koşulu kaldır',
        onclick: () => { d.kosullar.splice(i, 1); degisti(true); },
      }, '×');
      return h('li', { class: 'kosul' }, alan, op, ...degerAlanlari(k, i), sil);
    }

    function kosulAlani() {
      const liste = h('ol', { class: 'kosul-listesi' }, d.kosullar.map(kosulSatiri));
      const ekle = h('button', {
        type: 'button', class: 'dugme-kucuk',
        onclick: () => { d.kosullar.push(varsayilanKosul('name')); degisti(true); },
      }, '+ Koşul ekle');
      const baglac = d.kosullar.length > 1
        ? secim([['AND', 'Hepsi sağlansın (VE)'], ['OR', 'Herhangi biri yeter (VEYA)']], d.baglac,
          (v) => { d.baglac = v; degisti(); }, { 'aria-label': 'Koşulların birleşimi', class: 'baglac' })
        : null;
      return bolum('Koşullar', h('div', {}, liste, h('div', { class: 'satir-grup' }, ekle, baglac)));
    }

    function sutunAlani() {
      const grup = secim(GRUPLAR, d.grup, (v) => {
        d.grup = v;
        d.sirala = v ? 'toplam' : 'modified';
        d.yon = 'DESC';
        degisti(true);
      }, { 'aria-label': 'Gruplama' });

      let icerik;
      if (d.grup) {
        icerik = h('p', { class: 'olusturucu-not' }, 'Her grup için dosya sayısı (adet) ve toplam boyut (toplam) gösterilir.');
      } else {
        icerik = h('div', { class: 'sutun-listesi' }, Object.keys(ETIKETLER).map((a) => h('label', { class: 'sutun' },
          h('input', {
            type: 'checkbox', checked: d.sutunlar.includes(a), value: a,
            onchange: (e) => {
              // Yeni işaretlenen sütun sona eklenir; mevcut sıra korunur
              d.sutunlar = d.sutunlar.filter((x) => x !== a);
              if (e.target.checked) d.sutunlar.push(a);
              degisti();
            },
          }),
          ETIKETLER[a])));
      }
      return bolum('Göster', h('div', {}, h('div', { class: 'satir-grup' }, grup), icerik));
    }

    function siralamaAlani() {
      let secenekler;
      if (d.grup) {
        const anahtar = GRUP_ADI[d.grup] || d.grup;
        secenekler = [['toplam', 'Toplam boyut'], ['adet', 'Dosya sayısı'], [anahtar, GRUPLAR.find((g) => g[0] === d.grup)[1]]];
      } else {
        secenekler = [['', 'Sıralama yok'], ...Object.keys(ETIKETLER).map((a) => [a, ETIKETLER[a]])];
      }
      const alan = secim(secenekler, d.sirala || '', (v) => { d.sirala = v; degisti(); }, { 'aria-label': 'Sıralama alanı' });
      const yon = secim([['DESC', 'Büyükten küçüğe / yeniden eskiye'], ['ASC', 'Küçükten büyüğe / eskiden yeniye']], d.yon,
        (v) => { d.yon = v; degisti(); }, { 'aria-label': 'Sıralama yönü' });
      const limit = h('input', {
        type: 'text', inputmode: 'numeric', class: 'kisa', value: d.limit, placeholder: 'hepsi',
        'aria-label': 'En fazla sonuç', oninput: (e) => { d.limit = e.target.value; degisti(); },
      });
      return bolum('Sırala', h('div', { class: 'satir-grup' }, alan, yon, h('span', { class: 'ara-yazi' }, 'en fazla'), limit, h('span', { class: 'ara-yazi' }, 'sonuç')));
    }

    function bolum(baslik, icerik) {
      return h('div', { class: 'olusturucu-bolum' }, h('div', { class: 'olusturucu-etiket' }, baslik), icerik);
    }

    function ciz() {
      const sifirla = h('button', {
        type: 'button', class: 'dugme-sade',
        onclick: () => { d = varsayilanDurum(ayarlar.varsayilanKaynak); digerKlasor = false; degisti(true); },
      }, 'Sıfırla');
      kok.replaceChildren(
        kaynakAlani(),
        kosulAlani(),
        sutunAlani(),
        siralamaAlani(),
        h('div', { class: 'olusturucu-alt' }, durumSatiri, sifirla),
      );
      uret();
    }

    ciz();
    return { yenile: uret };
  }

  window.FQOlusturucu = { kur, sqlUret, kosulSql, varsayilanDurum, varsayilanKosul, TURLER, OPERATORLER, KOSUL_ALANLARI, GRUPLAR };
})();
