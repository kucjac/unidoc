package model

/*
import (
	"errors"

	"github.com/unidoc/unidoc/pdf/contentstream"
	"github.com/unidoc/unidoc/pdf/core"
	"github.com/unidoc/unidoc/pdf/model/fonts"
)

// TextFieldOptions defines optional parameter for a text field in a form.
type TextFieldOptions struct {
	MaxLen int    // Ignored if <= 0.
	Value  string // Ignored if empty ("").
}

// NewTextField generates a new text field with partial name `name` at location
// specified by `rect` on given `page` and with field specific options `opt`.
func NewTextField(page *PdfPage, name string, rect []float64, opt TextFieldOptions) (*PdfFieldText, error) {
	if page == nil {
		return nil, errors.New("Page not specified")
	}
	if len(name) <= 0 {
		return nil, errors.New("Required attribute not specified")
	}
	if len(rect) != 4 {
		return nil, errors.New("Invalid range")
	}

	field := NewPdfField()
	textfield := &PdfFieldText{}
	field.SetContext(textfield)
	textfield.PdfField = field

	textfield.T = core.MakeString(name)

	if opt.MaxLen > 0 {
		textfield.MaxLen = core.MakeInteger(int64(opt.MaxLen))
	}
	if len(opt.Value) > 0 {
		textfield.V = core.MakeString(opt.Value)
	}

	widget := NewPdfAnnotationWidget()
	widget.Rect = core.MakeArrayFromFloats(rect) //[]float64{144.0, 595.89, 294.0, 617.9})
	widget.P = page.ToPdfObject()
	widget.F = core.MakeInteger(4) // 4 (100 -> Print/show annotations).
	widget.Parent = textfield.ToPdfObject()

	//*form.Fields = append(*form.Fields, field)

	textfield.Annotations = append(textfield.Annotations, widget)
	//page.Annotations = append(page.Annotations, widget.PdfAnnotation)

	return textfield, nil
}

// CheckboxFieldOptions defines optional parameters for a checkbox field a form.
type CheckboxFieldOptions struct {
	Checked bool
}

// NewCheckboxField generates a new checkbox field with partial name `name` at location `rect`
// on specified `page` and with field specific options `opt`.
func NewCheckboxField(page *PdfPage, name string, rect []float64, opt CheckboxFieldOptions) (*PdfFieldButton, error) {
	if page == nil {
		return nil, errors.New("Page not specified")
	}
	if len(name) <= 0 {
		return nil, errors.New("Required attribute not specified")
	}
	if len(rect) != 4 {
		return nil, errors.New("Invalid range")
	}

	zapfdb := fonts.NewFontZapfDingbats()

	field := NewPdfField()
	buttonfield := &PdfFieldButton{}
	field.SetContext(buttonfield)
	buttonfield.PdfField = field

	buttonfield.T = core.MakeString(name)
	buttonfield.SetType(ButtonTypeCheckbox)

	state := "Off"
	if opt.Checked {
		state = "Yes"
	}

	buttonfield.V = core.MakeName(state)

	widget := NewPdfAnnotationWidget()
	widget.Rect = core.MakeArrayFromFloats(rect)
	widget.P = page.ToPdfObject()
	widget.F = core.MakeInteger(4)
	widget.Parent = buttonfield.ToPdfObject()

	w := rect[2] - rect[0]
	h := rect[3] - rect[1]

	// Off state.
	var cs bytes.Buffer
	cs.WriteString("q\n")
	cs.WriteString("0 0 1 rg\n")
	cs.WriteString("BT\n")
	cs.WriteString("/ZaDb 12 Tf\n")
	cs.WriteString("ET\n")
	cs.WriteString("Q\n")


	cc := contentstream.NewContentCreator()
	cc.Add_q()
	cc.Add_rg(0, 0, 1)
	cc.Add_BT()
	cc.Add_Tf(*core.MakeName("ZaDb"), 12)
	cc.Add_Td(0, 0)
	cc.Add_ET()
	cc.Add_Q()

	xformOff := NewXObjectForm()
	xformOff.SetContentStream(cc.Bytes(), core.NewRawEncoder())
	xformOff.BBox = core.MakeArrayFromFloats([]float64{0, 0, w, h})
	xformOff.Resources = NewPdfPageResources()
	xformOff.Resources.SetFontByName("ZaDb", zapfdb.ToPdfObject())

	// On state (Yes).
	cc = contentstream.NewContentCreator()
	cc.Add_q()
	cc.Add_re(0, 0, w, h)
	cc.Add_W().Add_n()
	cc.Add_rg(0, 0, 1)
	cc.Translate(0, 3.0)
	cc.Add_BT()
	cc.Add_Tf(*core.MakeName("ZaDb"), 12)
	cc.Add_Td(0, 0)
	cc.Add_Tj(*core.MakeString("\064"))
	cc.Add_ET()
	cc.Add_Q()

	xformOn := NewXObjectForm()
	xformOn.SetContentStream(cc.Bytes(), core.NewRawEncoder())
	xformOn.BBox = core.MakeArrayFromFloats([]float64{0, 0, w, h})
	xformOn.Resources = NewPdfPageResources()
	xformOn.Resources.SetFontByName("ZaDb", zapfdb.ToPdfObject())

	dchoiceapp := core.MakeDict()
	dchoiceapp.Set("Off", xformOff.ToPdfObject())
	dchoiceapp.Set("Yes", xformOn.ToPdfObject())

	appearance := core.MakeDict()
	appearance.Set("N", dchoiceapp)

	widget.AP = appearance
	widget.AS = core.MakeName(state)

	buttonfield.Annotations = append(buttonfield.Annotations, widget)

	return buttonfield, nil
}

// ComboboxFieldOptions defines optional parameters for a combobox form field.
type ComboboxFieldOptions struct {
	// Choices is the list of string values that can be selected.
	Choices []string
}

// NewComboboxField generates a new combobox form field with partial name `name` at location `rect`
// on specified `page` and with field specific options `opt`.
func NewComboboxField(page *PdfPage, name string, rect []float64, opt ComboboxFieldOptions) (*PdfFieldChoice, error) {
	if page == nil {
		return nil, errors.New("Page not specified")
	}
	if len(name) <= 0 {
		return nil, errors.New("Required attribute not specified")
	}
	if len(rect) != 4 {
		return nil, errors.New("Invalid range")
	}

	field := NewPdfField()
	chfield := &PdfFieldChoice{}
	field.SetContext(chfield)
	chfield.PdfField = field

	chfield.T = core.MakeString(name)
	chfield.Opt = core.MakeArray()
	for _, choicestr := range opt.Choices {
		chfield.Opt.Append(core.MakeString(choicestr))
	}
	chfield.SetFlag(FieldFlagCombo)

	widget := NewPdfAnnotationWidget()
	widget.Rect = core.MakeArrayFromFloats(rect)
	widget.P = page.ToPdfObject()
	widget.F = core.MakeInteger(4) // TODO: Make flags for these values and a way to set.
	widget.Parent = chfield.ToPdfObject()

	chfield.Annotations = append(chfield.Annotations, widget)

	return chfield, nil
}
*/