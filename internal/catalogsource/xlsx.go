// Package catalogsource reads source workbooks without executing formulas or
// changing originals. Cached values and formula text are retained separately.
package catalogsource

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path"
	"strconv"
	"strings"
)

type Cell struct {
	Ref     string `json:"ref"`
	Value   string `json:"value"`
	Formula string `json:"formula,omitempty"`
	Type    string `json:"type,omitempty"`
}
type Row struct {
	Number int    `json:"number"`
	Cells  []Cell `json:"cells"`
}
type Sheet struct {
	Name string `json:"name"`
	Rows []Row  `json:"rows"`
}
type Workbook struct {
	Source string  `json:"source"`
	SHA256 string  `json:"sha256"`
	Sheets []Sheet `json:"sheets"`
}

func Read(file string) (Workbook, error) {
	f, err := os.Open(file)
	if err != nil {
		return Workbook{}, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return Workbook{}, err
	}
	if info.Size() > 32<<20 {
		return Workbook{}, fmt.Errorf("workbook exceeds 32 MiB")
	}
	sum := sha256.New()
	if _, err = io.Copy(sum, f); err != nil {
		return Workbook{}, err
	}
	z, err := zip.NewReader(f, info.Size())
	if err != nil {
		return Workbook{}, err
	}
	book, err := readZip(z)
	book.Source = file
	book.SHA256 = hex.EncodeToString(sum.Sum(nil))
	return book, err
}
func readZip(z *zip.Reader) (Workbook, error) {
	files := map[string]*zip.File{}
	var total uint64
	for _, f := range z.File {
		if _, exists := files[f.Name]; exists {
			return Workbook{}, fmt.Errorf("duplicate ZIP member")
		}
		files[f.Name] = f
		total += f.UncompressedSize64
		if f.UncompressedSize64 > 16<<20 || total > 64<<20 {
			return Workbook{}, fmt.Errorf("expanded workbook too large")
		}
	}
	read := func(name string) ([]byte, error) {
		f := files[name]
		if f == nil {
			return nil, fmt.Errorf("missing XLSX part %s", name)
		}
		r, err := f.Open()
		if err != nil {
			return nil, err
		}
		defer r.Close()
		b, err := io.ReadAll(io.LimitReader(r, (16<<20)+1))
		if len(b) > 16<<20 {
			return nil, fmt.Errorf("XLSX part too large")
		}
		return b, err
	}
	var shared []string
	if files["xl/sharedStrings.xml"] != nil {
		b, err := read("xl/sharedStrings.xml")
		if err != nil {
			return Workbook{}, err
		}
		d := xml.NewDecoder(strings.NewReader(string(b)))
		for {
			t, err := d.Token()
			if err == io.EOF {
				break
			}
			if err != nil {
				return Workbook{}, err
			}
			if s, ok := t.(xml.StartElement); ok && s.Name.Local == "si" {
				v, err := richText(d, s)
				if err != nil {
					return Workbook{}, err
				}
				shared = append(shared, v)
			}
		}
	}
	b, err := read("xl/_rels/workbook.xml.rels")
	if err != nil {
		return Workbook{}, err
	}
	var rels struct {
		Items []struct {
			ID     string `xml:"Id,attr"`
			Target string `xml:"Target,attr"`
			Mode   string `xml:"TargetMode,attr"`
		} `xml:"Relationship"`
	}
	if err = xml.Unmarshal(b, &rels); err != nil {
		return Workbook{}, err
	}
	targets := map[string]string{}
	for _, r := range rels.Items {
		if r.Mode == "External" {
			continue
		}
		target := path.Clean(path.Join("xl", r.Target))
		if strings.HasPrefix(r.Target, "/") {
			target = strings.TrimPrefix(path.Clean(r.Target), "/")
		}
		if !strings.HasPrefix(target, "xl/") {
			return Workbook{}, fmt.Errorf("invalid workbook relationship")
		}
		targets[r.ID] = target
	}
	b, err = read("xl/workbook.xml")
	if err != nil {
		return Workbook{}, err
	}
	var wb struct {
		Sheets []struct {
			Name string `xml:"name,attr"`
			ID   string `xml:"id,attr"`
		} `xml:"sheets>sheet"`
	}
	if err = xml.Unmarshal(b, &wb); err != nil {
		return Workbook{}, err
	}
	out := Workbook{Sheets: []Sheet{}}
	for _, s := range wb.Sheets {
		b, err = read(targets[s.ID])
		if err != nil {
			return out, err
		}
		sheet := Sheet{Name: s.Name, Rows: []Row{}}
		var raw struct {
			Rows []struct {
				N     int `xml:"r,attr"`
				Cells []struct {
					Ref     string `xml:"r,attr"`
					Type    string `xml:"t,attr"`
					Value   string `xml:"v"`
					Formula string `xml:"f"`
					Inline  struct {
						Inner string `xml:",innerxml"`
					} `xml:"is"`
				} `xml:"c"`
			} `xml:"sheetData>row"`
		}
		if err = xml.Unmarshal(b, &raw); err != nil {
			return out, err
		}
		if len(raw.Rows) > 10000 {
			return out, fmt.Errorf("too many source rows")
		}
		for _, r := range raw.Rows {
			row := Row{Number: r.N, Cells: []Cell{}}
			for _, c := range r.Cells {
				v := c.Value
				switch c.Type {
				case "s":
					i, e := strconv.Atoi(v)
					if e != nil || i < 0 || i >= len(shared) {
						return out, fmt.Errorf("invalid shared string at %s", c.Ref)
					}
					v = shared[i]
				case "inlineStr":
					d := xml.NewDecoder(strings.NewReader("<is>" + c.Inline.Inner + "</is>"))
					t, e := d.Token()
					if e != nil {
						return out, e
					}
					v, err = richText(d, t.(xml.StartElement))
					if err != nil {
						return out, err
					}
				}
				row.Cells = append(row.Cells, Cell{Ref: c.Ref, Value: v, Formula: c.Formula, Type: c.Type})
			}
			sheet.Rows = append(sheet.Rows, row)
		}
		out.Sheets = append(out.Sheets, sheet)
	}
	return out, nil
}
func richText(d *xml.Decoder, start xml.StartElement) (string, error) {
	var b strings.Builder
	for {
		t, err := d.Token()
		if err != nil {
			return "", err
		}
		switch v := t.(type) {
		case xml.StartElement:
			if v.Name.Local == "t" {
				var text string
				if err := d.DecodeElement(&text, &v); err != nil {
					return "", err
				}
				b.WriteString(text)
			}
		case xml.EndElement:
			if v.Name == start.Name {
				return b.String(), nil
			}
		}
	}
}
