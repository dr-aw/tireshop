package catalogsource

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"tireshop/internal/catalogdata"
)

type Product struct {
	ID         string              `json:"id"`
	Name       string              `json:"name"`
	Price      int64               `json:"unit_price_rub"`
	Stock      int64               `json:"stock"`
	Available  bool                `json:"available"`
	Details    catalogdata.Details `json:"details"`
	Photos     []string            `json:"photos"`
	SourceFile string              `json:"source_file"`
	Sheet      string              `json:"sheet"`
	Row        Row                 `json:"source_row"`
}
type Batch struct {
	Hash      string     `json:"hash"`
	Workbooks []Workbook `json:"workbooks"`
	Products  []Product  `json:"products"`
}

var percentPattern = regexp.MustCompile(`\d{1,3}\s*%`)

func Normalize(books []Workbook) (Batch, error) {
	batch := Batch{Workbooks: books, Products: []Product{}}
	sum := sha256.New()
	sum.Write([]byte("tireshop-initial-catalog-v1"))
	seen := map[string]bool{}
	for _, book := range books {
		sum.Write([]byte(book.SHA256))
		if len(book.Sheets) == 0 {
			return batch, fmt.Errorf("empty workbook")
		}
		sheet := book.Sheets[0]
		if len(sheet.Rows) == 0 {
			return batch, fmt.Errorf("empty product sheet")
		}
		headers := map[string]string{}
		for _, c := range sheet.Rows[0].Cells {
			headers[strings.TrimRight(c.Ref, "0123456789")] = strings.TrimSpace(c.Value)
		}
		for _, row := range sheet.Rows[1:] {
			values := map[string]string{}
			for _, c := range row.Cells {
				if c.Type == "e" || c.Formula != "" && c.Value == "" {
					return batch, fmt.Errorf("uncached/error formula: %s row %d", sheet.Name, row.Number)
				}
				values[headers[strings.TrimRight(c.Ref, "0123456789")]] = strings.TrimSpace(c.Value)
			}
			sku := values["Артикул"]
			if sku == "" {
				if len(values) > 0 {
					return batch, fmt.Errorf("missing SKU row %d", row.Number)
				}
				continue
			}
			if seen[sku] {
				return batch, fmt.Errorf("duplicate SKU %s", sku)
			}
			seen[sku] = true
			p := Product{ID: ProductID(sku), Name: values["Название"], Available: true, SourceFile: book.Source, Sheet: sheet.Name, Row: row, Photos: []string{}}
			d := catalogdata.Details{SKU: sku, UnitsPerSale: 1, Description: values["Описание"]}
			var err error
			number := func(key string) *float64 {
				raw := strings.ReplaceAll(values[key], ",", ".")
				if raw == "" {
					return nil
				}
				v, e := strconv.ParseFloat(raw, 64)
				if e != nil {
					err = fmt.Errorf("%s invalid %s", sku, key)
				}
				return &v
			}
			integer := func(key string) int64 {
				v, e := strconv.ParseInt(values[key], 10, 64)
				if e != nil || v < 0 {
					err = fmt.Errorf("%s invalid %s", sku, key)
				}
				return v
			}
			if strings.HasPrefix(sku, "SH-") {
				d.Kind = "tyre"
				d.Brand = values["Бренд"]
				d.Model = values["Модель"]
				d.Size = values["Размер"]
				d.Season = values["Сезонность"]
				d.LoadIndex = values["Нагрузка"]
				d.SpeedIndex = values["Скоростной индекс"]
				d.Location = values["Место"]
				d.TreadPercent = number("Остаток")
				if values["Год выпуска"] != "" {
					y := int(integer("Год выпуска"))
					d.ProductionYear = &y
				}
				p.Price = integer("Цена")
				p.Stock = integer("Количество")
				if sku == "SH-0010" || sku == "SH-0076" {
					p.Name = percentPattern.ReplaceAllString(p.Name, values["Остаток"]+"%")
					d.ReviewNotes = "Название исправлено по столбцу процента протектора; перепроверить."
				}
				if regexp.MustCompile(`(?i)нов(ая|ые|ый)`).MatchString(p.Name) && strings.Contains(strings.ToLower(d.Description), "б/у") {
					p.Available = false
					d.ReviewNotes = "Требует проверки: название говорит о новых шинах, описание — б/у. Не публиковать до уточнения."
				}
			} else {
				d.Kind = "wheel"
				p.Price = integer("Цена(Руб)")
				p.Stock = integer("Количество комплектов")
				d.UnitsPerSale = int(integer("Количество дисков в комплекте"))
				d.Condition = values["Состояние"]
				d.Diameter = number("Диаметр")
				d.Width = number("Ширина")
				d.WheelOffset = number("Вылет")
				d.PCD = strings.ReplaceAll(strings.ReplaceAll(values["Сверловка"], "х", "x"), "*", "x")
				d.WheelType = values["Тип"]
				d.CenterBore = number("Диаметр ЦО(Dia)")
				if sku == "SU-1709" {
					p.Name = strings.ReplaceAll(p.Name, "ET-118.5", "ET+118.5")
					d.ReviewNotes = "Вылет +118,5 мм подтверждён; название исправлено."
				}
			}
			if err != nil {
				return batch, err
			}
			if p.Name == "" || p.Price > 9007199254740991 || p.Stock > 9007199254740991 {
				return batch, fmt.Errorf("invalid product %s", sku)
			}
			if err = d.Validate(); err != nil {
				return batch, fmt.Errorf("%s: %w", sku, err)
			}
			for _, photo := range strings.Split(values["Фото"], ",") {
				photo = strings.TrimSpace(photo)
				if photo != "" {
					if filepath.Base(photo) != photo || strings.ContainsAny(photo, `/\`) {
						return batch, fmt.Errorf("unsafe image reference %s", sku)
					}
					p.Photos = append(p.Photos, photo)
				}
			}
			p.Details = d
			batch.Products = append(batch.Products, p)
		}
	}
	batch.Hash = hex.EncodeToString(sum.Sum(nil))
	return batch, nil
}
func ProductID(sku string) string {
	b := sha256.Sum256([]byte("tireshop:catalog:sku:" + sku))
	b[6] = (b[6] & 15) | 128
	b[8] = (b[8] & 63) | 128
	h := hex.EncodeToString(b[:16])
	return h[:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:32]
}
