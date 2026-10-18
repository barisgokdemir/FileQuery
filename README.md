# FileQuery

[![CI](https://github.com/barisgokdemir/FileQuery/actions/workflows/ci.yml/badge.svg)](https://github.com/barisgokdemir/FileQuery/actions/workflows/ci.yml)

Dosya sisteminde SQL ile arama. Windows, Linux ve macOS'ta çalışan, tek dosyalık, bağımlılıksız bir komut satırı aracı.

```
> fq "SELECT name, size, modified FROM 'C:\Users\baris\Downloads' WHERE size > 500 MB ORDER BY size DESC LIMIT 3"

name                         size  modified
────────────────────────  ───────  ────────────────
ubuntu-24.04-desktop.iso   5,7 GB  2024-05-02 21:14
yedek-2023.zip             2,1 GB  2024-01-15 09:30
sunum-kaydi.mp4          812,4 MB  2024-03-08 17:02
3 sonuç, 48.213 öğe tarandı, 0,41 sn
```

## Kurulum

[Releases](https://github.com/barisgokdemir/FileQuery/releases) sayfasından işletim sisteminize uygun dosyayı indirin
ve içindeki `fq.exe` (Windows) veya `fq` dosyasını PATH'teki bir klasöre koyun. Go yüklüyse:

```bash
go install github.com/barisgokdemir/FileQuery/cmd/fq@latest
```

## Örnekler

```sql
-- Son bir haftada değişen fotoğraflar
SELECT path FROM '%USERPROFILE%' WHERE ext IN ('jpg', 'png', 'heic') AND modified > NOW() - 7d

-- Adı "fatura" ile başlayan PDF'ler, gizli dosyalar hariç
SELECT name, modified FROM 'D:\Belgeler' WHERE name LIKE 'fatura%' AND ext = 'pdf' AND NOT hidden

-- Diskte en çok yer kaplayan dosya türleri
SELECT ext, COUNT(*) AS adet, SUM(size) AS toplam FROM 'C:\Projeler' WHERE type = 'file'
GROUP BY ext ORDER BY toplam DESC LIMIT 10

-- Sadece iki seviye derinlikte, düzenli ifadeyle log dosyaları
SELECT * FROM 'C:\Logs' DEPTH 2 WHERE name MATCHES '^app-\d{4}-\d{2}\.log$'

-- 2023'te oluşturulmuş, 10-100 MB arası videolar; iki klasörde birden
SELECT path, size FROM 'D:\Videolar', 'E:\Arsiv' WHERE created BETWEEN '01.01.2023' AND '31.12.2023'
AND size BETWEEN 10 MB AND 100 MB AND ext IN ('mp4', 'mkv')

-- Yıllara göre dosya sayısı
SELECT YEAR(modified) AS yil, COUNT(*) FROM '~/Documents' GROUP BY YEAR(modified) ORDER BY yil
```

Sonuçları başka araçlara aktarmak için `--csv` veya `--json`:

```bash
fq --csv "SELECT path, size FROM '.' WHERE size > 1 GB" > buyuk-dosyalar.csv
fq --json "SELECT name, modified FROM '.' WHERE ext = 'log'" | jq '.[].name'
```

`fq` komutunu sorgusuz çalıştırırsanız etkileşimli kip açılır: sorguyu yazıp Enter'a basın, uzun sorguları `\` ile
satırlara bölün, uzun süren bir aramayı Ctrl+C ile durdurun. `.yardim`, `.alanlar`, `.bicim csv` ve `.cikis`
komutları kullanılabilir.

## Sorgu dili

```
SELECT * | ifade [AS ad], ...
FROM 'klasör' [, 'klasör' ...] [DEPTH n]
[WHERE koşul]
[GROUP BY ifade, ...]
[ORDER BY ifade [ASC | DESC], ...]
[LIMIT n]
```

- Klasör yolları tek tırnak içinde yazılır; ters bölü işaretinin özel anlamı yoktur: `'C:\Program Files'`.
  `~` ev klasörüne, `%DEGISKEN%` ortam değişkenine açılır. `'C:'` tek başına sürücünün köküdür.
- `DEPTH n`: en fazla kaç seviye aşağı inileceği. `DEPTH 1` sadece klasörün doğrudan içeriğidir.
- Anahtar kelimeler büyük/küçük harf duyarsızdır. `--` ile satır sonuna kadar yorum yazılabilir.

### Alanlar

| Alan | Tür | Açıklama |
|---|---|---|
| `name` | metin | dosya veya klasör adı |
| `ext` | metin | uzantı, noktasız ve küçük harf: `pdf` |
| `path` | metin | tam yol |
| `dir` | metin | bulunduğu klasör |
| `type` | metin | `'file'`, `'dir'` veya `'link'` |
| `size` | boyut | bayt; klasörlerde boş (NULL) |
| `modified` | tarih | son değiştirilme |
| `created` | tarih | oluşturulma (Windows, macOS; Linux'ta boş) |
| `accessed` | tarih | son erişim |
| `depth` | sayı | aranan klasöre göre derinlik |
| `hidden` | mantıksal | gizli mi (Windows özniteliği; Linux/macOS'ta adı `.` ile başlayanlar) |
| `readonly` | mantıksal | salt okunur mu |
| `system` | mantıksal | sistem dosyası mı (Windows) |
| `owner` | metin | sahibi (Windows: `DOMAIN\kullanıcı`) |
| `group` | metin | grubu (Linux/macOS) |
| `mode` | metin | izinler, ör. `rw-r--r--` |
| `links` | sayı | sabit bağlantı (hardlink) sayısı |

`SELECT *` şu sütunları gösterir: `name`, `size`, `modified`, `dir`.

### Operatörler ve değerler

| | |
|---|---|
| Karşılaştırma | `=`, `!=` (`<>`), `<`, `<=`, `>`, `>=` |
| Mantık | `AND`, `OR`, `NOT`, parantez |
| Metin | `LIKE 'fatura%'` (`%` herhangi bir dizi, `_` tek karakter), `MATCHES 'düzenli ifade'` |
| Liste ve aralık | `IN ('pdf', 'docx')`, `BETWEEN 1 MB AND 10 MB` |
| Boş değer | `IS NULL`, `IS NOT NULL` |
| Aritmetik | tarih ± süre: `NOW() - 7d`; boyutlar: `size + 1 MB` |
| Boyut | `500 B`, `500 KB`, `2 MB`, `1.5 GB`, `1 TB` (1024 tabanlı) |
| Süre | `30s`, `15m`, `12h`, `7d`, `2w` (Türkçe: `sn`, `dk`, `sa`, `g`, `hf`) |
| Tarih | `'2024-03-15'`, `'2024-03-15 14:30'`, `'15.03.2024'` |

Metin karşılaştırmaları ve `LIKE`, Windows'taki dosya adları gibi **büyük/küçük harf duyarsızdır** (`İ/ı/I/i` dahil).
`MATCHES` Go düzenli ifadelerini kullanır ve harf duyarlıdır; duyarsız arama için `(?i)` ekleyin.
NULL içeren karşılaştırmalar SQL'deki gibi yanlış sayılır; bu yüzden `size > 1 MB` klasörleri hiçbir zaman döndürmez.

### Fonksiyonlar

`NOW()`, `TODAY()`, `LOWER(x)`, `UPPER(x)`, `LEN(x)`, `YEAR(t)`, `MONTH(t)`, `DAY(t)`
ve toplama fonksiyonları `COUNT(*)`, `COUNT(x)`, `SUM(x)`, `MIN(x)`, `MAX(x)`, `AVG(x)`.

### Hatalar

Hatalı bir sorguda hatanın yeri işaretlenir ve mümkünse düzeltme önerilir:

```
SELECT nmae FROM 'C:\Users' WHERE ext = 'pdf'
       ^
8. karakter: bilinmeyen alan "nmae" (name mi demek istediniz?)
```

## Nasıl çalışır

- **Sorgu dili:** El yazımı bir lexer ve özyinelemeli inişli (recursive descent) parser, sorguyu bir sözdizimi ağacına
  çevirir. Alan adları, fonksiyonlar ve `GROUP BY` kuralları sorgu çalışmadan önce denetlenir.
- **Tarama:** Sabit sayıda işçi goroutine'i, ortak bir iş kuyruğundan klasör alarak paralel tarar. Klasörler
  sıralanmadan ve parça parça okunur. Dosya bilgisi (boyut, tarih) ve pahalı bilgiler (sahip, hardlink sayısı)
  sadece sorgu onlara ihtiyaç duyarsa okunur.
- **Güvenlik ve dayanıklılık:** Sembolik bağlantılar ve Windows junction'ları izlenmez, bu yüzden tarama döngüye girmez.
  Erişim izni olmayan klasörler atlanır ve sonunda raporlanır (`-v` ile liste).
- **Akış:** `ORDER BY` veya `GROUP BY` yoksa CSV/JSON sonuçları bulundukça yazılır; `LIMIT` dolduğunda tarama durur.
- **Bağımlılık yok:** Sadece Go standart kütüphanesi. Windows'a özgü bilgiler (öznitelikler, sahip, hardlink)
  `syscall` ile okunur.

```
cmd/fq/            komut satırı ve etkileşimli kip
internal/query/    lexer, parser, sözdizimi ağacı, değer tipleri
internal/engine/   paralel tarayıcı, ifade değerlendirici, toplama; platforma özel dosya bilgileri
internal/output/   tablo, CSV ve JSON çıktısı
```

## Geliştirme

```bash
go test ./...          # birim ve entegrasyon testleri
go run ./cmd/fq "SELECT * FROM '.' LIMIT 5"
```

CI testleri Windows, Linux ve macOS'ta çalıştırır. Yeni sürüm çıkarmak için `VERSION` dosyasındaki numarayı
artırıp `main`'e göndermek yeterli: CI etiketi oluşturur ve Windows, Linux, macOS (amd64 ve arm64) için derlenmiş
dosyaları Release'e ekler.

## Geçmiş

**v2.0 (2026):** Go ile sıfırdan yazıldı: gerçek bir sorgu dili, paralel tarama, toplama fonksiyonları,
etkileşimli kip, testler ve çapraz platform derleme.

**v1.0 (2021):** İlk sürüm, sorguları bir web formundan alan bir PHP uygulamasıydı. O günkü hali
[`v1.0`](https://github.com/barisgokdemir/FileQuery/releases/tag/v1.0) etiketinde duruyor. Form girdisiyle sunucuda
dosya oluşturup silebildiği için bir web sunucusunda çalıştırılmamalıdır; v2 bu nedenle şimdilik yalnızca okuma yapar.

## Lisans

[MIT](LICENSE)
