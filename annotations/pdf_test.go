package annotations

import (
	"fmt"
	"testing"
)

func test(name string, t *testing.T) {
	zip := fmt.Sprintf("testfiles/%s.zip", name)
	outfile := fmt.Sprintf("/tmp/%s.pdf", name)
	options := PdfGeneratorOptions{AddPageNumbers: true, AllPages: true, AnnotationsOnly: false}
	generator := CreatePdfGenerator(zip, outfile, options)
	err := generator.Generate()

	if err != nil {
		t.Error(err, name)
	}
}
func TestGenerateA3(t *testing.T) {
	test("a3", t)
}
func TestGenerateA4(t *testing.T) {
	test("a4", t)
}

func TestGenerateA5(t *testing.T) {
	test("a5", t)
}
func TestGenerateLetter(t *testing.T) {
	test("letter", t)
}
func TestGenerateRM(t *testing.T) {
	test("rm", t)
}
func TestGenerateTmpl(t *testing.T) {
	test("tmpl", t)
}
func TestGenerateStrangeBug(t *testing.T) {
	test("strange", t)
}

func TestHighlights(t *testing.T) {
	test("highlights", t)
}
