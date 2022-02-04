package templates

import (
	"text/template"

	"github.com/envoyproxy/protoc-gen-validate/module/gencontext"
	"github.com/envoyproxy/protoc-gen-validate/templates/cc"
	"github.com/envoyproxy/protoc-gen-validate/templates/ccnop"
	golang "github.com/envoyproxy/protoc-gen-validate/templates/go"
	"github.com/envoyproxy/protoc-gen-validate/templates/java"
	"github.com/envoyproxy/protoc-gen-validate/templates/shared"
	pgs "github.com/lyft/protoc-gen-star"
	pgsgo "github.com/lyft/protoc-gen-star/lang/go"
)

type (
	RegisterFn func(tpl *template.Template, params pgs.Parameters)
	FilePathFn func(f pgs.File, ctx pgsgo.Context, tpl *template.Template) *pgs.FilePath
)

func makeTemplate(ext string, fn RegisterFn, params pgs.Parameters, genCtx gencontext.GenContext) *template.Template {
	tpl := template.New(ext)
	shared.RegisterFunctions(tpl, params, genCtx)
	fn(tpl, params)
	return tpl
}

func Template(params pgs.Parameters, genCtx gencontext.GenContext) map[string][]*template.Template {
	return map[string][]*template.Template{
		"cc":    {makeTemplate("h", cc.RegisterHeader, params, genCtx), makeTemplate("cc", cc.RegisterModule, params, genCtx)},
		"ccnop": {makeTemplate("h", ccnop.RegisterHeader, params, genCtx), makeTemplate("cc", ccnop.RegisterModule, params, genCtx)},
		"go":    {makeTemplate("go", golang.Register, params, genCtx)},
		"java":  {makeTemplate("java", java.Register, params, genCtx)},
	}
}

func FilePathFor(tpl *template.Template) FilePathFn {
	switch tpl.Name() {
	case "h":
		return cc.CcFilePath
	case "ccnop":
		return cc.CcFilePath
	case "cc":
		return cc.CcFilePath
	case "java":
		return java.JavaFilePath
	default:
		return func(f pgs.File, ctx pgsgo.Context, tpl *template.Template) *pgs.FilePath {
			out := ctx.OutputPath(f)
			out = out.SetExt(".validate." + tpl.Name())
			return &out
		}
	}
}
