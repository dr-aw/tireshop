package catalogsource

import (
	"archive/zip"
	"bytes"
	"testing"
)

func TestWorkbookPreservesValuesAndFormulas(t *testing.T) {
	parts := map[string]string{
		"xl/workbook.xml":            `<workbook xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="Товары" r:id="rId1"/></sheets></workbook>`,
		"xl/_rels/workbook.xml.rels": `<Relationships><Relationship Id="rId1" Target="worksheets/sheet1.xml"/></Relationships>`,
		"xl/sharedStrings.xml":       `<sst><si><r><t>SH-</t></r><r><t>TEST</t></r></si></sst>`,
		"xl/worksheets/sheet1.xml":   `<worksheet><sheetData><row r="2"><c r="A2" t="s"><v>0</v></c><c r="B2" t="inlineStr"><is><t>Имя &amp; текст</t></is></c><c r="C2"><f>2*3</f><v>6</v></c><c r="D2"><f>1+1</f></c></row></sheetData></worksheet>`,
	}
	var b bytes.Buffer
	w := zip.NewWriter(&b)
	for name, text := range parts {
		f, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = f.Write([]byte(text)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	z, err := zip.NewReader(bytes.NewReader(b.Bytes()), int64(b.Len()))
	if err != nil {
		t.Fatal(err)
	}
	book, err := readZip(z)
	if err != nil {
		t.Fatal(err)
	}
	if len(book.Sheets) != 1 || book.Sheets[0].Name != "Товары" {
		t.Fatal("sheet names lost")
	}
	row := book.Sheets[0].Rows[0]
	if row.Number != 2 || row.Cells[0].Value != "SH-TEST" || row.Cells[1].Value != "Имя & текст" || row.Cells[2].Value != "6" || row.Cells[2].Formula != "2*3" || row.Cells[3].Value != "" || row.Cells[3].Formula != "1+1" {
		t.Fatalf("source not preserved: %+v", row)
	}
}
