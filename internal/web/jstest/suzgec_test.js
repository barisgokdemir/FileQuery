// Sütun süzgeci ayrıştırıcısının testleri: node internal/web/jstest/suzgec_test.js
'use strict';
process.env.TZ = 'Europe/Istanbul';
const { suzgecDerle } = require('../static/suzgec.js');

const simdi = new Date(2026, 9, 18, 12).getTime();
const T = (y, ay, g, sa = 0, dk = 0) => ({ t: new Date(y, ay - 1, g, sa, dk).toISOString() });
const MB = 1024 ** 2;
const GB = 1024 ** 3;

// [süzgeç, tür, değer, beklenen]; beklenen null ise süzgeç hata vermeli
const vakalar = [
  ['pdf', 'metin', 'Rapor.PDF', true],
  ['=rapor.pdf', 'metin', 'Rapor.PDF', true],
  ['=rapor', 'metin', 'Rapor.PDF', false],
  ['^rap', 'metin', 'Rapor.pdf', true],
  ['.pdf$', 'metin', 'a.pdf', true],
  ['.pdf$', 'metin', 'a.pdf.bak', false],
  ['fat*2024', 'metin', 'Fatura-2024', true],
  ['fat?ra', 'metin', 'Fatura', true],
  ['!tmp', 'metin', 'a.tmp', false],
  ['pdf | docx', 'metin', 'x.docx', true],
  ['İSTANBUL', 'metin', 'istanbul.txt', true],
  ['IŞIK', 'metin', 'ışık.txt', true],
  ['!=a', 'metin', 'a', false],
  ['!=a', 'metin', 'ab', true],
  ['a.b', 'metin', 'axb', false], // nokta düz karakterdir
  ['pdf', 'metin', null, false],
  ['!pdf', 'metin', null, true],
  ['> 10 MB', 'boyut', 11 * MB, true],
  ['> 10 MB', 'boyut', 10 * MB, false],
  ['>1 GB', 'boyut', 1.02 * GB, true],
  ['>=1 GB', 'boyut', GB, true],
  ['<=500 KB', 'boyut', 500 * 1024, true],
  ['1,5 MB', 'boyut', 1.52 * MB, true],
  ['1,5 MB', 'boyut', 1.56 * MB, false],
  ['1.5 GB', 'boyut', 1.5 * GB, true],
  ['1 GB..2 GB', 'boyut', 2 * GB, true],
  ['1 GB..2 GB', 'boyut', 2 * GB + 1, false],
  ['100', 'boyut', 100, true],
  ['100', 'boyut', 101, false],
  ['!0', 'boyut', 0, false],
  ['5..10', 'sayi', 10, true],
  ['5..10', 'sayi', 11, false],
  ['1.500', 'sayi', 1500, true],
  ['1.234,5', 'sayi', 1234.5, true],
  ['2928,13', 'sayi', 2928.1265, true],
  ['<>3', 'sayi', 3, false],
  ['-5', 'sayi', -5, true],
  ['2024', 'tarih', T(2024, 6, 1, 10), true],
  ['2024', 'tarih', T(2025, 1, 1), false],
  ['03.2024', 'tarih', T(2024, 4, 1), false],
  ['15.03.2024', 'tarih', T(2024, 3, 15, 23, 59), true],
  ['15.03.2024 14:30', 'tarih', T(2024, 3, 15, 14, 30), true],
  ['15.03.2024 14:30', 'tarih', T(2024, 3, 15, 14, 31), false],
  ['2024-03-15', 'tarih', T(2024, 3, 15, 9), true],
  ['2024-03', 'tarih', T(2024, 3, 31, 8), true],
  ['>01.06.2024', 'tarih', T(2024, 6, 1, 10), false],
  ['>=01.06.2024', 'tarih', T(2024, 6, 1, 10), true],
  ['<=03.2024', 'tarih', T(2024, 3, 31, 23), true],
  ['>2024', 'tarih', T(2024, 12, 31), false],
  ['01.01.2024..31.03.2024', 'tarih', T(2024, 3, 31, 20), true],
  ['01.01.2024..31.03.2024', 'tarih', T(2024, 4, 1), false],
  ['son 7g', 'tarih', T(2026, 10, 15), true],
  ['son 7g', 'tarih', T(2026, 10, 1), false],
  ['bugün', 'tarih', T(2026, 10, 18, 8), true],
  ['dün', 'tarih', T(2026, 10, 17, 8), true],
  ['dün', 'tarih', T(2026, 10, 18, 8), false],
  ['2021 | 2023', 'tarih', T(2023, 5, 5), true],
  ['>7g', 'sure', { d: 8 * 86400 }, true],
  ['<12sa', 'sure', { d: 13 * 3600 }, false],
  ['1g 6sa', 'sure', { d: 30 * 3600 + 60 }, true],
  ['1g..3g', 'sure', { d: 2 * 86400 }, true],
  ['evet', 'mantik', true, true],
  ['hayır', 'mantik', true, false],
  ['7', 'sure', { d: 7 }, null],
  ['> 10 XB', 'boyut', 1, null],
  ['31.13.2024', 'tarih', T(2024, 1, 1), null],
  ['2 GB..1 GB', 'boyut', 1, null],
  ['dün akşam', 'tarih', T(2024, 1, 1), null],
  ['abc', 'sayi', 1, null],
];

let hata = 0;
for (const [metin, tur, deger, beklenen] of vakalar) {
  const s = suzgecDerle(metin, tur, simdi);
  const sonuc = s?.hata ? null : s.f(deger);
  if (sonuc !== beklenen) {
    hata++;
    console.error(`HATA: ${tur} ${JSON.stringify(metin)} ${JSON.stringify(deger)} → ${s?.hata || sonuc}, beklenen ${beklenen}`);
  }
}
if (suzgecDerle('  ', 'metin') !== null) {
  hata++;
  console.error('HATA: boş süzgeç null dönmeli');
}
console.log(`${vakalar.length} vaka, ${hata} hata`);
process.exit(hata ? 1 : 0);
