// FileQuery örnek sorguları: işletim sistemine ve kullanıcının gerçek klasörlerine göre üretilir.
'use strict';

(function () {
  // Algılanmayan bir sistem seçildiğinde kullanılan genel yollar.
  const SABLONLAR = {
    windows: {
      home: '%USERPROFILE%', desktop: '%USERPROFILE%\\Desktop', documents: '%USERPROFILE%\\Documents',
      downloads: '%USERPROFILE%\\Downloads', pictures: '%USERPROFILE%\\Pictures', music: '%USERPROFILE%\\Music',
      videos: '%USERPROFILE%\\Videos', temp: '%TEMP%', kok: 'C:\\',
    },
    linux: {
      home: '~', desktop: '~/Desktop', documents: '~/Documents', downloads: '~/Downloads',
      pictures: '~/Pictures', music: '~/Music', videos: '~/Videos', temp: '/tmp', kok: '/',
    },
    darwin: {
      home: '~', desktop: '~/Desktop', documents: '~/Documents', downloads: '~/Downloads',
      pictures: '~/Pictures', music: '~/Music', videos: '~/Movies', temp: '/tmp', kok: '/',
    },
  };

  const SISTEMLER = { windows: 'Windows', linux: 'Linux', darwin: 'macOS' };

  // SQL metin sabiti: tek tırnaklar ikilenir.
  function q(metin) {
    return `'${String(metin).replace(/'/g, "''")}'`;
  }

  // Seçilen sistem için klasör yollarını döndürür. Algılanan sistemde sunucunun bulduğu gerçek
  // yollar (OneDrive'a taşınmış Masaüstü, Türkçe adlı klasörler) şablonların yerini alır.
  function yollar(sistem, algilanan, yerler) {
    const y = { ...(SABLONLAR[sistem] || SABLONLAR.linux) };
    if (sistem !== algilanan || !Array.isArray(yerler)) return y;
    for (const yer of yerler) {
      if (yer.key === 'drive' || yer.key === 'root') {
        if (!y.kokGercek) {
          y.kok = yer.path;
          y.kokGercek = true;
        }
      } else if (yer.key in y) {
        y[yer.key] = yer.path;
      }
    }
    // Olmayan klasörler için ev klasörüne düş
    for (const k of ['desktop', 'documents', 'downloads', 'pictures', 'videos']) {
      if (!yerler.some((yer) => yer.key === k)) y[k] = y.home;
    }
    return y;
  }

  const KOD = "('go', 'cs', 'js', 'ts', 'py', 'java', 'sql', 'php', 'html', 'css', 'cpp', 'c', 'rs')";
  const KURULUM = {
    windows: "('exe', 'msi', 'msix', 'zip', '7z', 'rar')",
    linux: "('deb', 'rpm', 'appimage', 'run', 'sh', 'gz', 'xz', 'zip')",
    darwin: "('dmg', 'pkg', 'zip')",
  };

  // Her örnek: { grup, ad, aciklama, sorgu }
  function uret(sistem, algilanan, yerler) {
    const y = yollar(sistem, algilanan, yerler);
    const s = (yol) => q(yol);
    const liste = [
      {
        grup: 'Sık kullanılanlar',
        ad: 'Bu hafta indirilenler',
        aciklama: 'İndirilenler klasörüne son 7 günde gelen dosyalar. Geliş tarihi (created) ve değişiklik tarihine birlikte bakar.',
        sorgu: `SELECT name, size, created, modified FROM ${s(y.downloads)}\nWHERE type = 'file' AND (created > NOW() - 7d OR modified > NOW() - 7d)\nORDER BY created DESC`,
      },
      {
        grup: 'Sık kullanılanlar',
        ad: 'En büyük dosyalar',
        aciklama: 'Ev klasöründeki 100 MB üzeri dosyalar, büyükten küçüğe.',
        sorgu: `SELECT name, size, modified, dir FROM ${s(y.home)}\nWHERE type = 'file' AND size > 100 MB\nORDER BY size DESC LIMIT 50`,
      },
      {
        grup: 'Sık kullanılanlar',
        ad: 'Son değişen belgeler',
        aciklama: 'Belgeler klasöründe son 30 günde değişen Office, PDF ve metin dosyaları.',
        sorgu: `SELECT name, modified, size, dir FROM ${s(y.documents)}\nWHERE ext IN @belge AND modified > NOW() - 30d\nORDER BY modified DESC`,
      },
      {
        grup: 'Sık kullanılanlar',
        ad: 'Masaüstündekiler',
        aciklama: 'Masaüstünün doğrudan içeriği (alt klasörlere inmez).',
        sorgu: `SELECT name, type, size, modified FROM ${s(y.desktop)} DEPTH 1\nWHERE NOT hidden\nORDER BY modified DESC`,
      },
      {
        grup: 'Sık kullanılanlar',
        ad: 'Ada göre ara',
        aciklama: 'Adında "fatura" veya "sözleşme" geçen PDF ve Word dosyaları. Kelimeleri değiştirip kullanın.',
        sorgu: `SELECT name, modified, dir FROM ${s(y.home)}\nWHERE ext IN ('pdf', 'docx', 'doc') AND (name LIKE '%fatura%' OR name LIKE '%sözleşme%')\nORDER BY modified DESC`,
      },
      {
        grup: 'Disk kullanımı',
        ad: 'Türlere göre yer kullanımı',
        aciklama: 'Hangi dosya türü ne kadar yer kaplıyor?',
        sorgu: `SELECT ext, COUNT(*) AS adet, SUM(size) AS toplam FROM ${s(y.home)}\nWHERE type = 'file'\nGROUP BY ext ORDER BY toplam DESC LIMIT 20`,
      },
      {
        grup: 'Disk kullanımı',
        ad: 'İndirilenler klasörlere göre',
        aciklama: 'İndirilenler içindeki alt klasörlerin dosya sayısı ve boyutu.',
        sorgu: `SELECT dir, COUNT(*) AS adet, SUM(size) AS toplam FROM ${s(y.downloads)}\nWHERE type = 'file'\nGROUP BY dir ORDER BY toplam DESC LIMIT 20`,
      },
      {
        grup: 'Disk kullanımı',
        ad: 'Müzik kütüphanesi',
        aciklama: 'Ses dosyalarının türlere göre sayısı ve boyutu (@ses: mp3, flac, m4a, wav...).',
        sorgu: `SELECT ext, COUNT(*) AS adet, SUM(size) AS toplam FROM ${s(y.home)}\nWHERE ext IN @ses\nGROUP BY ext ORDER BY toplam DESC`,
      },
      {
        grup: 'Resim ve video',
        ad: 'Tüm resimler',
        aciklama: '@resim listesindeki bütün uzantılar (jpg, png, gif, heic, webp, RAW...), büyükten küçüğe.',
        sorgu: `SELECT name, size, modified, dir FROM ${s(y.home)}\nWHERE ext IN @resim AND NOT hidden\nORDER BY size DESC LIMIT 200`,
      },
      {
        grup: 'Resim ve video',
        ad: 'Simgeler hariç resimler',
        aciklama: 'Hazır listeden istemediğiniz uzantıları çıkarma örneği: .ico, .cur ve .svg hariç.',
        sorgu: `SELECT name, size, modified, dir FROM ${s(y.home)}\nWHERE ext IN @resim AND ext NOT IN ('ico', 'cur', 'svg', 'svgz') AND NOT hidden\nORDER BY modified DESC`,
      },
      {
        grup: 'Resim ve video',
        ad: 'Resimler yıllara göre',
        aciklama: 'Resimler klasöründeki resimlerin yıllara göre sayısı ve boyutu.',
        sorgu: `SELECT YEAR(modified) AS yil, COUNT(*) AS adet, SUM(size) AS toplam FROM ${s(y.pictures)}\nWHERE ext IN @resim\nGROUP BY YEAR(modified) ORDER BY yil`,
      },
      {
        grup: 'Resim ve video',
        ad: 'Tüm videolar',
        aciklama: '@video listesindeki bütün uzantılar (mp4, mkv, mov, avi, webm...), büyükten küçüğe.',
        sorgu: `SELECT name, size, modified, dir FROM ${s(y.home)}\nWHERE ext IN @video\nORDER BY size DESC`,
      },
      {
        grup: 'Resim ve video',
        ad: 'Video türlerine göre yer',
        aciklama: 'Hangi video biçimi ne kadar yer kaplıyor?',
        sorgu: `SELECT ext, COUNT(*) AS adet, SUM(size) AS toplam FROM ${s(y.home)}\nWHERE ext IN @video\nGROUP BY ext ORDER BY toplam DESC`,
      },
      {
        grup: 'Resim ve video',
        ad: 'Son 30 günde gelen resim ve videolar',
        aciklama: 'İki hazır liste birlikte: ext IN (@resim, @video).',
        sorgu: `SELECT name, size, created, dir FROM ${s(y.home)}\nWHERE ext IN (@resim, @video) AND (created > NOW() - 30d OR modified > NOW() - 30d)\nORDER BY created DESC`,
      },
      {
        grup: 'Temizlik',
        ad: 'Kopya olabilecek dosyalar',
        aciklama: '"dosya (1).pdf", "dosya - Kopya.docx", "dosya (copy).txt" gibi adlar.',
        sorgu: `SELECT name, size, modified, dir FROM ${s(y.home)}\nWHERE type = 'file' AND name MATCHES '(?i)( \\(\\d+\\)| - (kopya|copy)( \\(\\d+\\))?| \\((kopya|copy)\\))\\.[^.]+$'\nORDER BY size DESC`,
      },
      {
        grup: 'Temizlik',
        ad: 'Bir yıldır değişmeyen büyük dosyalar',
        aciklama: '1 yıldan uzun süredir değişmemiş, 100 MB üzeri dosyalar: arşivlemek veya silmek için adaylar.',
        sorgu: `SELECT name, size, modified, dir FROM ${s(y.home)}\nWHERE type = 'file' AND size > 100 MB AND modified < NOW() - 365d\nORDER BY size DESC`,
      },
      {
        grup: 'Temizlik',
        ad: 'Kurulum dosyaları',
        aciklama: 'İndirilenler klasöründe kalmış kurulum ve arşiv dosyaları.',
        sorgu: `SELECT name, size, created, modified FROM ${s(y.downloads)}\nWHERE ext IN ${KURULUM[sistem] || KURULUM.linux}\nORDER BY size DESC`,
      },
      {
        grup: 'Temizlik',
        ad: 'Arşiv dosyaları',
        aciklama: 'Zip, rar, 7z, iso gibi arşiv ve disk kalıpları (@arsiv), büyükten küçüğe.',
        sorgu: `SELECT name, size, modified, dir FROM ${s(y.home)}\nWHERE ext IN @arsiv\nORDER BY size DESC`,
      },
      {
        grup: 'Temizlik',
        ad: 'Boş dosyalar',
        aciklama: '0 bayt boyutunda dosyalar.',
        sorgu: `SELECT name, modified, dir FROM ${s(y.home)}\nWHERE type = 'file' AND size = 0 AND NOT hidden\nORDER BY dir`,
      },
      {
        grup: 'Temizlik',
        ad: 'Geçici dosyaların toplamı',
        aciklama: 'Geçici dosyalar klasörü ne kadar yer kaplıyor?',
        sorgu: `SELECT COUNT(*) AS adet, SUM(size) AS toplam, MIN(modified) AS en_eski FROM ${s(y.temp)}\nWHERE type = 'file'`,
      },
      {
        grup: 'Geliştirici',
        ad: 'Kod dosyaları türlere göre',
        aciklama: 'Ev klasöründeki kaynak kod dosyalarının dile göre sayısı.',
        sorgu: `SELECT ext, COUNT(*) AS adet, SUM(size) AS toplam FROM ${s(y.home)}\nWHERE ext IN ${KOD} AND NOT hidden\nGROUP BY ext ORDER BY adet DESC`,
      },
      {
        grup: 'Geliştirici',
        ad: 'Bugün değişen kod',
        aciklama: 'Bugün kaydedilmiş kaynak kod dosyaları.',
        sorgu: `SELECT name, modified, dir FROM ${s(y.home)}\nWHERE ext IN ${KOD} AND modified >= TODAY()\nORDER BY modified DESC`,
      },
    ];

    // Sisteme özel örnekler
    if (sistem === 'windows') {
      liste.push(
        {
          grup: 'Windows',
          ad: `${y.kok.slice(0, 2)} sürücüsündeki en büyük dosyalar`,
          aciklama: 'Bütün sürücüde 1 GB üzeri dosyalar. Sürücü büyükse birkaç dakika sürebilir.',
          sorgu: `SELECT name, size, modified, dir FROM ${s(y.kok)}\nWHERE type = 'file' AND size > 1 GB\nORDER BY size DESC LIMIT 30`,
        },
        {
          grup: 'Windows',
          ad: 'Masaüstü kısayolları',
          aciklama: 'Masaüstündeki .lnk ve .url kısayolları.',
          sorgu: `SELECT name, modified FROM ${s(y.desktop)} DEPTH 1\nWHERE ext IN ('lnk', 'url')\nORDER BY name`,
        },
        {
          grup: 'Windows',
          ad: 'Gizli ve sistem dosyaları',
          aciklama: 'Ev klasörünün ilk iki seviyesindeki gizli veya sistem öznitelikli öğeler.',
          sorgu: `SELECT name, type, hidden, system, dir FROM ${s(y.home)} DEPTH 2\nWHERE hidden OR system\nORDER BY dir, name`,
        },
      );
    } else if (sistem === 'linux') {
      liste.push(
        {
          grup: 'Linux',
          ad: 'Büyük log dosyaları',
          aciklama: '/var/log altındaki log ve sıkıştırılmış log dosyaları.',
          sorgu: `SELECT name, size, modified, dir FROM '/var/log'\nWHERE type = 'file' AND (ext IN ('log', 'gz', 'journal') OR name LIKE '%.log.%')\nORDER BY size DESC LIMIT 30`,
        },
        {
          grup: 'Linux',
          ad: 'Gizli yapılandırma klasörleri',
          aciklama: 'Ev klasöründeki noktayla başlayan klasörler.',
          sorgu: `SELECT name, modified FROM ${s(y.home)} DEPTH 1\nWHERE type = 'dir' AND hidden\nORDER BY name`,
        },
        {
          grup: 'Linux',
          ad: 'Çalıştırılabilir dosyalar',
          aciklama: 'Ev klasöründe sahibinin çalıştırma izni olan dosyalar.',
          sorgu: `SELECT name, mode, owner, dir FROM ${s(y.home)} DEPTH 3\nWHERE type = 'file' AND mode LIKE '__x%' AND NOT hidden\nORDER BY dir, name`,
        },
      );
    } else if (sistem === 'darwin') {
      liste.push(
        {
          grup: 'macOS',
          ad: 'Uygulamalar',
          aciklama: '/Applications klasöründeki uygulamalar.',
          sorgu: `SELECT name, modified FROM '/Applications' DEPTH 1\nWHERE ext = 'app'\nORDER BY name`,
        },
        {
          grup: 'macOS',
          ad: 'Önbellek boyutu',
          aciklama: '~/Library/Caches klasörü ne kadar yer kaplıyor?',
          sorgu: `SELECT COUNT(*) AS adet, SUM(size) AS toplam FROM '~/Library/Caches'\nWHERE type = 'file'`,
        },
        {
          grup: 'macOS',
          ad: 'Gizli dosyalar',
          aciklama: 'Ev klasörünün doğrudan içindeki gizli öğeler.',
          sorgu: `SELECT name, type, modified FROM ${s(y.home)} DEPTH 1\nWHERE hidden\nORDER BY name`,
        },
      );
    }
    return liste;
  }

  window.FQOrnekler = { uret, yollar, SISTEMLER, q };
})();
