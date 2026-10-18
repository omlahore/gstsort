package main

import (
	"bytes"
	"encoding/base64"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	_ "image/jpeg"
	"image/png"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

type Invoice struct {
	File          string   `json:"-"`
	SupplierName  string   `json:"supplier_name"`
	SupplierGSTIN string   `json:"supplier_gstin"`
	BuyerGSTIN    string   `json:"buyer_gstin"`
	InvoiceNumber string   `json:"invoice_number"`
	InvoiceDate   string   `json:"invoice_date"`
	TaxableValue  float64  `json:"taxable_value"`
	CGST          float64  `json:"cgst"`
	SGST          float64  `json:"sgst"`
	IGST          float64  `json:"igst"`
	Cess          float64  `json:"cess"`
	Total         float64  `json:"total"`
	HSNSAC        []string `json:"hsn_sac"`
	Issues        []string `json:"-"`
	Notes         []string `json:"-"`
}

const prompt = `This is an Indian GST tax invoice. Read it and fill in the JSON.
supplier_gstin is the seller's 15-character GSTIN, buyer_gstin is the GSTIN of the customer it is billed to (empty if there is none).
invoice_date as printed. Amounts in rupees as plain numbers, 0 if a tax is not on the invoice.
taxable_value is the value before tax. total is the final invoice amount.
Copy GSTINs and the invoice number character by character, do not guess.`

func schema() map[string]any {
	str := map[string]any{"type": "string"}
	num := map[string]any{"type": "number"}
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"supplier_name": str, "supplier_gstin": str, "buyer_gstin": str,
			"invoice_number": str, "invoice_date": str,
			"taxable_value": num, "cgst": num, "sgst": num, "igst": num, "cess": num, "total": num,
			"hsn_sac": map[string]any{"type": "array", "items": str},
		},
		"required": []string{"supplier_name", "supplier_gstin", "buyer_gstin", "invoice_number", "invoice_date",
			"taxable_value", "cgst", "sgst", "igst", "cess", "total", "hsn_sac"},
	}
}

func main() {
	server := flag.String("server", "http://127.0.0.1:8091", "llama-server (or any OpenAI-compatible) base URL")
	model := flag.String("model", "", "model name to send, if the server needs one")
	mine := flag.String("gstin", "", "your own GSTIN, to check invoices are billed to you")
	out := flag.String("o", "invoices.csv", "CSV output path")
	flag.Parse()
	if flag.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "usage: gstsort [-gstin YOURGSTIN] [-o out.csv] invoices/ or files...")
		os.Exit(2)
	}
	*mine = strings.ToUpper(strings.TrimSpace(*mine))
	if *mine != "" && !validGSTIN(*mine) {
		log.Fatalf("-gstin %s is not a valid GSTIN", *mine)
	}

	var invs []*Invoice
	for _, f := range files(flag.Args()) {
		start := time.Now()
		pages, err := toImages(f)
		if err != nil {
			log.Printf("%s: %v", f, err)
			continue
		}
		inv, err := extract(*server, *model, pages)
		if err != nil {
			log.Printf("%s: %v", f, err)
			continue
		}
		inv.File = filepath.Base(f)
		invs = append(invs, inv)
		log.Printf("%s: %s %s ₹%.2f (%s)", inv.File, inv.SupplierName, inv.InvoiceNumber, inv.Total, time.Since(start).Round(time.Second))
	}
	check(invs, *mine)
	if err := write(*out, invs); err != nil {
		log.Fatal(err)
	}
	summary(invs, *out)
}

func files(args []string) []string {
	var fs []string
	for _, a := range args {
		filepath.Walk(a, func(p string, info os.FileInfo, err error) error {
			if err == nil && !info.IsDir() {
				switch strings.ToLower(filepath.Ext(p)) {
				case ".pdf", ".png", ".jpg", ".jpeg", ".webp":
					fs = append(fs, p)
				}
			}
			return nil
		})
	}
	sort.Strings(fs)
	return fs
}

// toImages caps the long side at 1400px; a 4000px phone photo overflows a small vision model's context.
func toImages(path string) ([][]byte, error) {
	if strings.EqualFold(filepath.Ext(path), ".pdf") {
		dir, err := os.MkdirTemp("", "gstsort")
		if err != nil {
			return nil, err
		}
		defer os.RemoveAll(dir)
		if out, err := exec.Command("pdftoppm", "-png", "-r", "150", "-l", "2", "-scale-to", "1400", path, filepath.Join(dir, "p")).CombinedOutput(); err != nil {
			return nil, fmt.Errorf("pdftoppm (install poppler): %v %s", err, out)
		}
		ps, _ := filepath.Glob(filepath.Join(dir, "p*.png"))
		sort.Strings(ps)
		var pages [][]byte
		for _, p := range ps {
			b, err := os.ReadFile(p)
			if err != nil {
				return nil, err
			}
			pages = append(pages, b)
		}
		return pages, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return nil, err
	}
	b := img.Bounds()
	if s := 1400 / math.Max(float64(b.Dx()), float64(b.Dy())); s < 1 {
		dst := image.NewRGBA(image.Rect(0, 0, int(float64(b.Dx())*s), int(float64(b.Dy())*s)))
		draw.CatmullRom.Scale(dst, dst.Bounds(), img, b, draw.Over, nil)
		img = dst
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return [][]byte{buf.Bytes()}, nil
}

func extract(server, model string, pages [][]byte) (*Invoice, error) {
	content := []map[string]any{{"type": "text", "text": prompt}}
	for _, p := range pages {
		content = append(content, map[string]any{"type": "image_url",
			"image_url": map[string]any{"url": "data:image/png;base64," + base64.StdEncoding.EncodeToString(p)}})
	}
	body, _ := json.Marshal(map[string]any{
		"model":       model,
		"temperature": 0,
		"messages":    []map[string]any{{"role": "user", "content": content}},
		"response_format": map[string]any{"type": "json_schema",
			"json_schema": map[string]any{"name": "invoice", "schema": schema()}},
	})
	resp, err := http.Post(strings.TrimRight(server, "/")+"/v1/chat/completions", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %s", resp.Status, raw)
	}
	var r struct {
		Choices []struct {
			Message struct{ Content string } `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(raw, &r); err != nil || len(r.Choices) == 0 {
		return nil, fmt.Errorf("bad response: %s", raw)
	}
	inv := &Invoice{}
	if err := json.Unmarshal([]byte(r.Choices[0].Message.Content), inv); err != nil {
		return nil, fmt.Errorf("model returned %q: %v", r.Choices[0].Message.Content, err)
	}
	inv.SupplierGSTIN = cleanGSTIN(inv.SupplierGSTIN)
	inv.BuyerGSTIN = cleanGSTIN(inv.BuyerGSTIN)
	inv.InvoiceDate = normDate(inv.InvoiceDate)
	return inv, nil
}

func cleanGSTIN(s string) string {
	return strings.ToUpper(strings.Join(strings.Fields(s), ""))
}

func normDate(s string) string {
	for _, l := range []string{"02-01-2006", "02/01/2006", "02.01.2006", "2006-01-02", "02 Jan 2006", "2 Jan 2006", "Jan 2, 2006", "02-Jan-2006", "2 January 2006", "02 January 2006", "January 2, 2006"} {
		if t, err := time.Parse(l, strings.TrimSpace(s)); err == nil {
			return t.Format("2006-01-02")
		}
	}
	return s
}

const gstChars = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"

// validGSTIN checks the format and the mod-36 check digit, which catches nearly any single misread character.
func validGSTIN(g string) bool {
	if len(g) != 15 || g[13] != 'Z' {
		return false
	}
	if st, err := strconv.Atoi(g[:2]); err != nil || st < 1 || st > 99 {
		return false
	}
	for i, c := range g[2:12] {
		letter := c >= 'A' && c <= 'Z'
		if (i < 5 || i == 9) != letter || (!letter && (c < '0' || c > '9')) {
			return false
		}
	}
	return checkChar(g[:14]) == g[14]
}

var asLetter = map[byte]byte{'0': 'O', '1': 'I', '2': 'Z', '5': 'S', '6': 'G', '8': 'B'}
var asDigit = map[byte]byte{'O': '0', 'D': '0', 'Q': '0', 'I': '1', 'L': '1', 'Z': '2', 'S': '5', 'G': '6', 'B': '8'}

// repairGSTIN swaps look-alikes into the slots the format demands and keeps the result only if the check digit agrees.
func repairGSTIN(g string) (string, bool) {
	if validGSTIN(g) {
		return g, true
	}
	if len(g) != 15 {
		return g, false
	}
	b := []byte(g)
	for i := 0; i < 14; i++ {
		letter := (i >= 2 && i <= 6) || i == 11
		digit := i < 2 || (i >= 7 && i <= 10)
		if r, ok := asLetter[b[i]]; letter && ok {
			b[i] = r
		} else if r, ok := asDigit[b[i]]; digit && ok {
			b[i] = r
		} else if i == 13 && b[i] == '2' {
			b[i] = 'Z'
		}
	}
	return string(b), validGSTIN(string(b))
}

func checkChar(s string) byte {
	sum := 0
	for i := 0; i < len(s); i++ {
		v := strings.IndexByte(gstChars, s[i])
		if v < 0 {
			return 0
		}
		p := v * (i%2 + 1)
		sum += p/36 + p%36
	}
	return gstChars[(36-sum%36)%36]
}

var rates = []float64{0, 0.1, 0.25, 1, 1.5, 3, 5, 6, 12, 18, 28, 40}

func check(invs []*Invoice, mine string) {
	seen := map[string]string{}
	for _, v := range invs {
		add := func(f string, a ...any) { v.Issues = append(v.Issues, fmt.Sprintf(f, a...)) }
		if g, ok := repairGSTIN(v.SupplierGSTIN); ok && g != v.SupplierGSTIN {
			v.Notes = append(v.Notes, fmt.Sprintf("read supplier GSTIN as %s, fixed to %s (checksum confirms)", v.SupplierGSTIN, g))
			v.SupplierGSTIN = g
		} else if !ok {
			add("supplier GSTIN %q fails the checksum, recheck it on the invoice", v.SupplierGSTIN)
		}
		if g, ok := repairGSTIN(v.BuyerGSTIN); ok && g != v.BuyerGSTIN {
			v.Notes = append(v.Notes, fmt.Sprintf("read buyer GSTIN as %s, fixed to %s (checksum confirms)", v.BuyerGSTIN, g))
			v.BuyerGSTIN = g
		}
		switch {
		case v.BuyerGSTIN == "":
			add("no buyer GSTIN, so this is a B2C bill and the credit can't be claimed")
		case !validGSTIN(v.BuyerGSTIN):
			add("buyer GSTIN %q fails the checksum", v.BuyerGSTIN)
		case mine != "" && v.BuyerGSTIN != mine:
			add("billed to %s, not to you", v.BuyerGSTIN)
		}
		if len(v.SupplierGSTIN) == 15 && len(v.BuyerGSTIN) == 15 {
			intra := v.SupplierGSTIN[:2] == v.BuyerGSTIN[:2]
			if intra && v.IGST > 0 {
				add("same state (%s) but charged IGST, it should be CGST+SGST", v.SupplierGSTIN[:2])
			}
			if !intra && v.CGST+v.SGST > 0 {
				add("different states (%s to %s) but charged CGST/SGST, it should be IGST", v.SupplierGSTIN[:2], v.BuyerGSTIN[:2])
			}
		}
		if math.Abs(v.CGST-v.SGST) > 0.5 {
			add("CGST %.2f and SGST %.2f should be equal", v.CGST, v.SGST)
		}
		tax := v.CGST + v.SGST + v.IGST
		if d := v.TaxableValue + tax + v.Cess - v.Total; math.Abs(d) > 1 {
			add("taxable %.2f + tax %.2f + cess %.2f doesn't add up to total %.2f (off by %.2f)", v.TaxableValue, tax, v.Cess, v.Total, d)
		}
		if v.TaxableValue > 0 {
			r := tax / v.TaxableValue * 100
			ok := false
			for _, x := range rates {
				ok = ok || math.Abs(r-x) < 0.15
			}
			if !ok {
				add("effective GST rate %.2f%% isn't a GST slab", r)
			}
		}
		for _, c := range v.HSNSAC {
			if strings.HasPrefix(c, "99633") {
				add("restaurant service (SAC %s): credit is usually blocked under section 17(5), check with your CA", c)
				break
			}
		}
		k := v.SupplierGSTIN + "/" + strings.ToUpper(v.InvoiceNumber)
		if f, dup := seen[k]; dup {
			add("duplicate of %s", f)
		}
		seen[k] = v.File
	}
}

func claimable(v *Invoice) bool {
	return len(v.Issues) == 0
}

func write(path string, invs []*Invoice) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	w.Write([]string{"file", "supplier", "supplier_gstin", "buyer_gstin", "invoice_no", "invoice_date", "hsn_sac",
		"taxable", "cgst", "sgst", "igst", "cess", "total", "itc_claimable", "issues", "notes"})
	money := func(x float64) string { return strconv.FormatFloat(x, 'f', 2, 64) }
	for _, v := range invs {
		itc := "0.00"
		if claimable(v) {
			itc = money(v.CGST + v.SGST + v.IGST + v.Cess)
		}
		w.Write([]string{v.File, v.SupplierName, v.SupplierGSTIN, v.BuyerGSTIN, v.InvoiceNumber, v.InvoiceDate,
			strings.Join(v.HSNSAC, " "), money(v.TaxableValue), money(v.CGST), money(v.SGST), money(v.IGST), money(v.Cess),
			money(v.Total), itc, strings.Join(v.Issues, "; "), strings.Join(v.Notes, "; ")})
	}
	w.Flush()
	return w.Error()
}

func summary(invs []*Invoice, out string) {
	var ok, itc, held float64
	for _, v := range invs {
		t := v.CGST + v.SGST + v.IGST + v.Cess
		if claimable(v) {
			ok++
			itc += t
		} else {
			held += t
		}
		if len(v.Issues)+len(v.Notes) > 0 {
			fmt.Printf("\n%s (%s %s)\n", v.File, v.SupplierName, v.InvoiceNumber)
			for _, is := range v.Issues {
				fmt.Printf("  - %s\n", is)
			}
			for _, n := range v.Notes {
				fmt.Printf("  ~ %s\n", n)
			}
		}
	}
	fmt.Printf("\n%d invoices, %.0f clean. Input tax credit ready to claim: ₹%.2f. Held back for review: ₹%.2f\nWrote %s\n",
		len(invs), ok, itc, held, out)
}
