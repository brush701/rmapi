package annotations

import (
	"bytes"
	"errors"
	"image/color"
	"io"
	"math"
	"strconv"

	"os"

	"github.com/juruen/rmapi/archive"
	"github.com/juruen/rmapi/encoding/rm"
	"github.com/phpdave/gofpdf"
	"github.com/phpdave/gofpdf/contrib/gofpdi"
)

const (
	DeviceWidth  = 1404.0
	DeviceHeight = 1872.0
	DPI          = 226.0
	PtPerPx      = 72.0 / DPI //PDF standard says 1 pt = 1/72 in
)

var rmPageSize = gofpdf.SizeType{Wd: DeviceWidth * PtPerPx, Ht: DeviceHeight * PtPerPx} //gopdf.Rect{445, 594}

type PdfGenerator struct {
	zipName        string
	outputFilePath string
	options        PdfGeneratorOptions
	template       bool
}

type PdfGeneratorOptions struct {
	AddPageNumbers  bool
	AllPages        bool
	AnnotationsOnly bool //export the annotations without the background/pdf
}

var (
	Black  = color.Black
	White  = color.White
	Grey   = color.Gray16{0x8000}
	Yellow = color.RGBA{R: 255, G: 240, B: 102, A: 77}
)

var CMap = map[rm.BrushColor]color.Color{
	rm.Black: Black,
	rm.Grey:  Grey,
	rm.White: White,
}

type Highlight struct {
	Contents   string
	Rect       []float32
	QuadPoints []float32
	Color      []float32
	Opacity    float32
	Author     string
}

func _scaleColors(r uint32, g uint32, b uint32, pressure float32) (int, int, int) {
	scaler := uint32(pressure * 100)
	rscaled := int(255 - (255-r*0xff/0xffff)*scaler/100)
	gscaled := int(255 - (255-g*0xff/0xffff)*scaler/100)
	bscaled := int(255 - (255-b*0xff/0xffff)*scaler/100)

	return rscaled, gscaled, bscaled
}

func transformAnnots(annots []Highlight, scale float32, pageHeight float32) []Highlight {

	xformed := make([]Highlight, len(annots))
	for i, note := range annots {
		var qp QuadPoints
		for i := 0; i < len(note.QuadPoints)-1; i += 2 {
			qp.Points = append(qp.Points, Point{note.QuadPoints[i] * PtPerPx * scale, pageHeight - note.QuadPoints[i+1]*PtPerPx*scale})
		}

		rect := Rect{LL: Point{note.Rect[0] * PtPerPx * scale, float32(pageHeight) - note.Rect[1]*PtPerPx*scale},
			UR: Point{note.Rect[2] * PtPerPx * scale, float32(pageHeight) - note.Rect[3]*PtPerPx*scale}}

		xformed[i] = Highlight{
			Contents:   note.Contents,
			Rect:       rect.ToList(),
			QuadPoints: qp.ToList(),
			Color:      note.Color,
			Opacity:    note.Opacity,
			Author:     note.Author,
		}
	}
	return xformed
}

func PaintStroke(stroke rm.Stroke, pdf *gofpdf.Fpdf, highlights *[]Highlight) error {
	// Beware! Here lie magic numbers aplenty. Based on RMRL
	// and hand tuned to get more-or-less correct appearance
	r, g, b, _ := CMap[stroke.BrushColor].RGBA()

	if stroke.BrushType == rm.Highlighter || stroke.BrushType == rm.HighlighterV5 {
		var rect Rect
		for i, segment := range stroke.Segments {
			if i == 0 {
				rect = Rect{LL: Point{X: segment.X - segment.Width/2, Y: segment.Y - segment.Width/2},
					UR: Point{X: segment.X + segment.Width/2, Y: segment.Y + segment.Width/2}}
			} else {
				newRect := Rect{LL: Point{X: segment.X - segment.Width/2, Y: segment.Y - segment.Width/2},
					UR: Point{X: segment.X + segment.Width/2, Y: segment.Y + segment.Width/2}}
				rect = rect.Union(newRect)
			}
		}

		qp := rect.ToQuadPoints()

		*highlights = append(*highlights, Highlight{
			Rect:       rect.ToList(),
			QuadPoints: qp.ToList(),
			Color:      []float32{float32(Yellow.R) / 255, float32(Yellow.G) / 255, float32(Yellow.B) / 255},
			Opacity:    float32(Yellow.A) / 255,
			Author:     "reMarkable",
		})

		return nil
	} else {
		pdf.SetLineCapStyle("round")
		pdf.SetLineJoinStyle("round")
		pdf.SetAlpha(1.0, "Normal")
	}
	for idx, segment := range stroke.Segments {
		if idx == 0 {
			pdf.MoveTo(float64(segment.X)*PtPerPx, float64(segment.Y)*PtPerPx)
			continue
		}
		prev := stroke.Segments[idx-1]

		var width float64
		switch stroke.BrushType {
		case rm.MechanicalPencil, rm.MechanicalPencilV5:
			pdf.SetDrawColor(_scaleColors(r, g, b, segment.Pressure))
			width = float64(segment.Width) * 1.5

		case rm.Pencil, rm.PencilV5:
			pdf.SetDrawColor(_scaleColors(r, g, b, segment.Pressure))
			width = float64(segment.Width) * 0.58

		case rm.Brush, rm.BrushV5:
			// Set the width
			modwidth := segment.Width * 0.75
			maxdelta := modwidth * 0.75
			delta := (segment.Pressure - 1) * maxdelta
			width := float64(modwidth + delta)

			press_mod := segment.Pressure * (1 - (segment.Speed / 150))
			pdf.SetDrawColor(_scaleColors(r, g, b, press_mod))

			distance := math.Sqrt(math.Pow(float64(segment.X-prev.X), 2) + math.Pow(float64(segment.Y-prev.Y), 2))
			if distance < width {
				pdf.SetLineCapStyle("round") // Rounded
			} else {
				pdf.SetLineCapStyle("square") // Flat
			}

		case rm.Marker, rm.MarkerV5:
			width = float64(segment.Width)

		case rm.BallPoint, rm.BallPointV5:
			maxdelta := segment.Width / 2
			delta := (segment.Pressure - 1) * maxdelta
			width = float64(segment.Width + delta)

		case rm.EraseArea, rm.Eraser:
			continue

		default:
			width = float64(segment.Width)
			pdf.SetDrawColor(_scaleColors(r, g, b, 1.0))
		}

		pdf.SetLineWidth(width / 2)
		pdf.LineTo(float64(segment.X)*PtPerPx, float64(segment.Y)*PtPerPx)
	}
	pdf.DrawPath("S")
	return nil
}

func CreatePdfGenerator(zipName, outputFilePath string, options PdfGeneratorOptions) *PdfGenerator {
	return &PdfGenerator{zipName: zipName, outputFilePath: outputFilePath, options: options}
}

func (p *PdfGenerator) Generate() error {
	file, err := os.Open(p.zipName)
	if err != nil {
		return err
	}

	defer file.Close()

	zip := archive.NewZip()

	fi, err := file.Stat()
	if err != nil {
		return err
	}

	err = zip.Read(file, fi.Size())
	if err != nil {
		return err
	}

	if zip.Content.FileType == "epub" {
		return errors.New("only pdf and notebooks supported")
	}

	if len(zip.Pages) == 0 {
		return errors.New("the document has no pages")
	}

	pdf := gofpdf.NewCustom(&gofpdf.InitType{
		UnitStr: "pt",
		Size:    rmPageSize,
	})

	// TODO: outlines
	/*
		if p.pdfReader != nil && p.options.AllPages {
			p.pdfReader.AddOutline()
			outlines := p.pdfReader.GetOutlineTree()
			c.SetOutlineTree(outlines)
		}
	*/
	pdf.OpenLayerPane()
	var seeker io.ReadSeeker

	layers := make([]int, 2)
	layers[0] = pdf.AddLayer("Background", true)
	layers[1] = pdf.AddLayer("Layer 1", true)

	annotations := make([][]Highlight, 0, 2)

	if zip.Content.FileType == "pdf" && zip.Payload != nil {
		seeker = io.ReadSeeker(bytes.NewReader(zip.Payload))
	}

	for i, page := range zip.Pages {
		hasContent := page.Data != nil

		// do not add a page when there are no annotations
		if !p.options.AllPages && !hasContent {
			continue
		}

		scale := float64(100)
		newHeight := float64(rmPageSize.Ht)
		newWidth := float64(rmPageSize.Wd)
		if zip.Content.FileType == "pdf" && zip.Payload != nil && seeker != nil {
			tpl1 := gofpdi.ImportPageFromStream(pdf, &seeker, i+1, "/MediaBox")
			sizes := gofpdi.GetPageSizes()
			w := sizes[i+1]["/MediaBox"]["w"]
			h := sizes[i+1]["/MediaBox"]["h"]
			var orientation string
			if w > h {
				orientation = "L"
			} else {
				orientation = "P"
			}

			// Need to resize the page so that it has the same aspect ratio as the reMarkable
			pdfRatio := w / h
			rmRatio := rmPageSize.Wd / rmPageSize.Ht

			if pdfRatio <= rmRatio {
				newWidth = rmRatio * h
				newHeight = h
				scale = h / rmPageSize.Ht * 100
			} else {
				newHeight = w / rmRatio
				newWidth = w
				scale = w / rmPageSize.Wd * 100
			}

			pdf.AddPageFormat(orientation, gofpdf.SizeType{Wd: newWidth, Ht: newHeight})
			pdf.BeginLayer(layers[0])
			gofpdi.UseImportedTemplate(pdf, tpl1, 0, 0, w, h)
			pdf.EndLayer()
		} else { // No underlying PDF
			pdf.AddPage()
			scale = 100
		}

		if !hasContent {
			continue
		}

		for idx, layer := range page.Data.Layers {
			if idx+1 >= len(layers) {
				layers = append(layers, pdf.AddLayer("Layer "+strconv.Itoa(idx), true))
			}
			annotations = append(annotations, make([]Highlight, 0))

			pdf.BeginLayer(layers[idx+1]) // layer 0 is background, idx is also zero based
			pdf.TransformBegin()
			pdf.TransformScaleXY(scale, 0, 0)
			for _, stroke := range layer.Strokes {

				if len(stroke.Segments) < 1 {
					continue
				}

				err = PaintStroke(stroke, pdf, &annotations[idx])
				if err != nil {
					continue
				}

				// Handle new highlights format from v2.7+
				if len(page.Highlights.LayerHighlights) > idx { //might be an off-by-one here, not sure if layers in the .highlights file are zero indexed?
					if len(page.Highlights.LayerHighlights[idx]) > 0 {
						var note Highlight
						cursor := -1

						for n, h := range page.Highlights.LayerHighlights[idx] {
							// Need to find the bounding box for each highlight
							// and build the list of QuadPoints
							qp := make([]float32, 0, 4)

							var rect Rect
							for i, r := range h.Rects { // There could in theory be multiple, though it appears there is only 1 right now
								ll := Point{X: r.X, Y: r.Y}
								ur := Point{X: r.X + r.Width, Y: r.Y + r.Height}
								if i == 0 {
									rect = Rect{LL: ll, UR: ur}
								} else {
									rect = rect.Union(Rect{LL: ll, UR: ur})
								}

								// Transform doesn't get applied to the Annotations, so we need to apply scale manually. Also for whatever reason
								// Annotations seem to have a different coordinate system than lines? It's inverted wrt the PDF spec, so we
								// flip them here
								qp = append(qp,
									r.X, r.Y+r.Height,
									r.X+r.Width, r.Y+r.Height,
									r.X, r.Y,
									r.X+r.Width, r.Y)
							}

							highlight := Highlight{
								Rect:       rect.ToList(),
								QuadPoints: qp,
								Color:      []float32{float32(Yellow.R) / 255, float32(Yellow.G) / 255, float32(Yellow.B) / 255},
								Opacity:    float32(Yellow.A) / 255,
								Author:     "reMarkable",
							}

							if cursor > 0 && h.Start-cursor > 10 {
								annotations[idx] = append(annotations[idx], note)
								note = highlight
							} else {
								if n == 0 {
									note = highlight
								} else {
									note = note.Union(highlight)
								}
							}

							cursor = h.Start + h.Length
						}

					}
				}
			}

			grouped := groupHighlights(annotations[idx])
			xformed := transformAnnots(grouped, float32(scale/100), float32(newHeight))
			for _, annot := range xformed {
				pdf.AddHighlightAnnotation(gofpdf.Highlight(annot))
			}

			pdf.TransformEnd()
			pdf.EndLayer()
		}
	}

	return pdf.OutputFileAndClose(p.outputFilePath)
}

func groupHighlights(list []Highlight) []Highlight {
	groupedStrokes := make([]Highlight, 0, len(list))

outer:
	for _, h := range list {
		for j, b := range groupedStrokes {
			if h.Intersects(b) {
				groupedStrokes[j] = b.Union(h)
				continue outer
			}
		}
		groupedStrokes = append(groupedStrokes, h)
	}

	if len(groupedStrokes) != len(list) {
		return groupHighlights(groupedStrokes)
	} else {
		return groupedStrokes
	}
}
