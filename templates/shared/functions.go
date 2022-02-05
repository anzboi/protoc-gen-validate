package shared

import (
	"text/template"

	"github.com/anzboi/protoc-gen-validate/module/gencontext"
	pgs "github.com/lyft/protoc-gen-star"
)

func RegisterFunctions(tpl *template.Template, params pgs.Parameters, genCtx gencontext.GenContext) {
	tpl.Funcs(map[string]interface{}{
		"disabled":  Disabled,
		"ignored":   Ignored,
		"required":  RequiredOneOf,
		"context":   rulesContext(genCtx),
		"render":    Render(tpl),
		"has":       Has,
		"needs":     Needs,
		"fileneeds": FileNeeds,
	})
}
