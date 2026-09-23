package utils

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
	"unicode"

	"choice-matrix-backend/internal/models"
)

func termTranslations(term models.Term) map[string]string {
	values := make(map[string]string, len(term.Translations))
	for _, translation := range term.Translations {
		values[translation.LanguageCode] = translation.Content
	}
	return values
}

func BuildLanguageJSON(terms []models.Term, languageCode string) ([]byte, error) {
	root := make(map[string]interface{})
	for _, term := range terms {
		value := termTranslations(term)[languageCode]
		if term.Module == "" {
			root[term.Key] = value
			continue
		}
		module, ok := root[term.Module].(map[string]string)
		if !ok {
			module = make(map[string]string)
			root[term.Module] = module
		}
		module[term.Key] = value
	}
	return json.MarshalIndent(root, "", "  ")
}

func writeZipFile(writer *zip.Writer, name string, content []byte) error {
	entry, err := writer.Create(name)
	if err != nil {
		return err
	}
	_, err = entry.Write(content)
	return err
}

func BuildLanguageJSONArchive(terms []models.Term, languageCodes []string) ([]byte, error) {
	var output bytes.Buffer
	archive := zip.NewWriter(&output)
	for _, languageCode := range languageCodes {
		content, err := BuildLanguageJSON(terms, languageCode)
		if err != nil {
			archive.Close()
			return nil, err
		}
		if err := writeZipFile(archive, languageCode+".json", content); err != nil {
			archive.Close()
			return nil, err
		}
	}
	if err := archive.Close(); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func excelColumnName(index int) string {
	name := ""
	for index > 0 {
		index--
		name = string(rune('A'+index%26)) + name
		index /= 26
	}
	return name
}

func validXMLText(value string) string {
	return strings.Map(func(r rune) rune {
		if r == '\t' || r == '\n' || r == '\r' || unicode.IsPrint(r) {
			return r
		}
		return -1
	}, value)
}

func writeInlineCell(writer io.Writer, reference string, value string, style int) error {
	var escaped bytes.Buffer
	if err := xml.EscapeText(&escaped, []byte(validXMLText(value))); err != nil {
		return err
	}
	_, err := fmt.Fprintf(writer, `<c r="%s" t="inlineStr" s="%d"><is><t xml:space="preserve">%s</t></is></c>`, reference, style, escaped.String())
	return err
}

func statusLabel(status models.TermStatus) string {
	switch status {
	case models.TermStatusDraft:
		return "草稿"
	case models.TermStatusPending:
		return "待翻译"
	case models.TermStatusReview:
		return "待发布"
	case models.TermStatusPublished:
		return "已发布"
	default:
		return string(status)
	}
}

func BuildTermXLSX(terms []models.Term, languages []models.Language) ([]byte, error) {
	if len(terms)+1 > 1048576 {
		return nil, fmt.Errorf("导出词条超过 Excel 单表上限")
	}

	headers := []string{"Key", "Module", "Description", "Status"}
	for _, language := range languages {
		headers = append(headers, language.Code)
	}

	var sheet bytes.Buffer
	sheet.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?><worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>`)
	sheet.WriteString(`<row r="1">`)
	for column, header := range headers {
		if err := writeInlineCell(&sheet, excelColumnName(column+1)+"1", header, 1); err != nil {
			return nil, err
		}
	}
	sheet.WriteString(`</row>`)

	for index, term := range terms {
		rowNumber := index + 2
		translations := termTranslations(term)
		values := []string{term.Key, term.Module, term.Description, statusLabel(term.Status)}
		for _, language := range languages {
			values = append(values, translations[language.Code])
		}
		fmt.Fprintf(&sheet, `<row r="%d">`, rowNumber)
		for column, value := range values {
			if err := writeInlineCell(&sheet, excelColumnName(column+1)+fmt.Sprint(rowNumber), value, 0); err != nil {
				return nil, err
			}
		}
		sheet.WriteString(`</row>`)
	}
	sheet.WriteString(`</sheetData></worksheet>`)

	files := map[string][]byte{
		"[Content_Types].xml":        []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/><Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/><Override PartName="/xl/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.styles+xml"/></Types>`),
		"_rels/.rels":                []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/></Relationships>`),
		"xl/workbook.xml":            []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?><workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="Translations" sheetId="1" r:id="rId1"/></sheets></workbook>`),
		"xl/_rels/workbook.xml.rels": []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/><Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/></Relationships>`),
		"xl/styles.xml":              []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?><styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><fonts count="2"><font><sz val="11"/><name val="Aptos"/></font><font><b/><sz val="11"/><name val="Aptos"/></font></fonts><fills count="1"><fill><patternFill patternType="none"/></fill></fills><borders count="1"><border/></borders><cellStyleXfs count="1"><xf numFmtId="0" fontId="0" fillId="0" borderId="0"/></cellStyleXfs><cellXfs count="2"><xf numFmtId="0" fontId="0" fillId="0" borderId="0" xfId="0"/><xf numFmtId="0" fontId="1" fillId="0" borderId="0" xfId="0" applyFont="1"/></cellXfs></styleSheet>`),
		"xl/worksheets/sheet1.xml":   sheet.Bytes(),
	}

	var output bytes.Buffer
	archive := zip.NewWriter(&output)
	for _, name := range []string{"[Content_Types].xml", "_rels/.rels", "xl/workbook.xml", "xl/_rels/workbook.xml.rels", "xl/styles.xml", "xl/worksheets/sheet1.xml"} {
		if err := writeZipFile(archive, name, files[name]); err != nil {
			archive.Close()
			return nil, err
		}
	}
	if err := archive.Close(); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}
