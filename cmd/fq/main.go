// Command fq searches the file system with SQL-like queries.
//
//	fq "SELECT name, size FROM 'C:\Users' WHERE ext = 'pdf' AND size > 5 MB ORDER BY size DESC"
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"

	"github.com/barisgokdemir/FileQuery/internal/engine"
	"github.com/barisgokdemir/FileQuery/internal/output"
	"github.com/barisgokdemir/FileQuery/internal/query"
)

// version is set at build time: -ldflags "-X main.version=2.0.0"
var version = "geliştirme"

const usageText = `FileQuery: dosya sisteminde SQL ile arama

Kullanım:
  fq [seçenekler] "SORGU"      sorguyu çalıştırır
  fq                           etkileşimli kipi açar
  echo "SORGU" | fq            sorguyu standart girdiden okur

Seçenekler:
  --format tablo|csv|json      çıktı biçimi (varsayılan: tablo)
  --csv, --json                --format kısayolları
  -v                           erişilemeyen klasörleri listeler
  --alanlar                    sorgularda kullanılabilecek alanları listeler
  --version                    sürümü gösterir

Örnekler:
  fq "SELECT name, size FROM 'C:\Users\baris\Downloads' WHERE size > 500 MB ORDER BY size DESC"
  fq "SELECT path FROM '%USERPROFILE%' WHERE ext IN ('jpg', 'png') AND modified > NOW() - 7d"
  fq "SELECT ext, COUNT(*), SUM(size) FROM 'D:\' GROUP BY ext ORDER BY SUM(size) DESC LIMIT 10"
  fq --csv "SELECT * FROM '.' WHERE name LIKE 'fatura%'" > faturalar.csv

Sorgu dili:
  SELECT * | ifade [AS ad], ...
  FROM 'klasör' [, 'klasör'...] [DEPTH n]
  [WHERE koşul] [GROUP BY ifade, ...] [ORDER BY ifade [ASC|DESC], ...] [LIMIT n]

  Operatörler: = != < <= > >=  AND OR NOT  LIKE  MATCHES  IN (...)  BETWEEN .. AND ..  IS [NOT] NULL
  Birimler:    500 KB, 2 MB, 1.5 GB   |   7d, 12h, 30m, 2w (NOW() - 7d)
  Tarihler:    '2024-03-15', '15.03.2024', '2024-03-15 14:30'
  Fonksiyonlar: NOW() TODAY() LOWER() UPPER() LEN() YEAR() MONTH() DAY()
                COUNT() SUM() MIN() MAX() AVG()
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr, isTerminal(os.Stdin)))
}

func isTerminal(f *os.File) bool {
	info, err := f.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

type config struct {
	format  string
	verbose bool
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer, interactive bool) int {
	fs := flag.NewFlagSet("fq", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { io.WriteString(stderr, usageText) }

	var (
		cfg         config
		csvFlag     bool
		jsonFlag    bool
		showVersion bool
		showFields  bool
	)
	fs.StringVar(&cfg.format, "format", "tablo", "")
	fs.BoolVar(&csvFlag, "csv", false, "")
	fs.BoolVar(&jsonFlag, "json", false, "")
	fs.BoolVar(&cfg.verbose, "v", false, "")
	fs.BoolVar(&showVersion, "version", false, "")
	fs.BoolVar(&showFields, "alanlar", false, "")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	switch {
	case csvFlag:
		cfg.format = "csv"
	case jsonFlag:
		cfg.format = "json"
	}
	if _, err := output.New(cfg.format, io.Discard); err != nil {
		fmt.Fprintln(stderr, "fq:", err)
		return 2
	}

	if showVersion {
		fmt.Fprintln(stdout, "fq", version)
		return 0
	}
	if showFields {
		printFields(stdout)
		return 0
	}

	src := strings.TrimSpace(strings.Join(fs.Args(), " "))
	if src == "" {
		if interactive {
			return repl(stdin, stdout, stderr, &cfg)
		}
		data, err := io.ReadAll(stdin)
		if err != nil {
			fmt.Fprintln(stderr, "fq:", err)
			return 1
		}
		src = strings.TrimSpace(string(data))
		if src == "" {
			fs.Usage()
			return 2
		}
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := execute(ctx, src, cfg, stdout, stderr); err != nil {
		return 1
	}
	return 0
}

// execute parses and runs one query, printing results to stdout and the
// summary or errors to stderr.
func execute(ctx context.Context, src string, cfg config, stdout, stderr io.Writer) error {
	q, err := query.Parse(src)
	if err != nil {
		var qe *query.Error
		if errors.As(err, &qe) {
			fmt.Fprintln(stderr, qe.Format(src))
		} else {
			fmt.Fprintln(stderr, "fq:", err)
		}
		return err
	}

	w, _ := output.New(cfg.format, stdout)
	cols := engine.Columns(q)
	if err := w.Begin(cols); err != nil {
		return err
	}
	isTable := cfg.format == "tablo" || cfg.format == "table"
	stats, runErr := engine.Run(ctx, q, engine.Options{SortByPath: isTable}, w.Row)
	if err := w.End(); err != nil && runErr == nil {
		runErr = err
	}

	if runErr != nil {
		if errors.Is(runErr, context.Canceled) {
			fmt.Fprintln(stderr, "\nsorgu iptal edildi")
		} else {
			fmt.Fprintln(stderr, "hata:", runErr)
		}
		return runErr
	}
	if ctx.Err() != nil {
		fmt.Fprintln(stderr, "\nsorgu iptal edildi; sonuçlar eksik olabilir")
	}

	printSummary(stderr, stats, cfg, isTable)
	return nil
}

func printSummary(w io.Writer, s engine.Stats, cfg config, isTable bool) {
	if !isTable && !cfg.verbose {
		return // keep CSV/JSON pipelines quiet unless asked
	}
	msg := fmt.Sprintf("%s sonuç, %s öğe tarandı, %s sn",
		group(int64(s.Rows)), group(s.Entries), strings.Replace(fmt.Sprintf("%.2f", s.Elapsed.Seconds()), ".", ",", 1))
	if isTable && s.Rows == 0 {
		msg = "Sonuç yok. " + msg
	}
	if s.Denied > 0 {
		msg += fmt.Sprintf("; erişim izni olmayan %s klasör atlandı", group(s.Denied))
		if !cfg.verbose {
			msg += " (-v ile listelenir)"
		}
	}
	fmt.Fprintln(w, msg)
	if cfg.verbose {
		for _, p := range s.DeniedPaths {
			fmt.Fprintln(w, "  erişilemedi:", p)
		}
		if int64(len(s.DeniedPaths)) < s.Denied {
			fmt.Fprintf(w, "  ... ve %s klasör daha\n", group(s.Denied-int64(len(s.DeniedPaths))))
		}
	}
}

// group formats an integer with Turkish thousands separators: 12.345
func group(n int64) string {
	s := fmt.Sprint(n)
	if len(s) <= 3 {
		return s
	}
	var b strings.Builder
	pre := len(s) % 3
	if pre > 0 {
		b.WriteString(s[:pre])
	}
	for i := pre; i < len(s); i += 3 {
		if b.Len() > 0 {
			b.WriteByte('.')
		}
		b.WriteString(s[i : i+3])
	}
	return b.String()
}

func printFields(w io.Writer) {
	fmt.Fprintln(w, "Alanlar:")
	for _, f := range query.Schema {
		fmt.Fprintf(w, "  %-9s %-10s %s\n", f.Name, f.Kind, f.Desc)
	}
	fmt.Fprintln(w, "\nSELECT * şu sütunları gösterir:", strings.Join(query.DefaultColumns, ", "))
}
