package shared

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	"github.com/anzboi/protoc-gen-validate/module/gencontext"
	"github.com/anzboi/protoc-gen-validate/validate"
	pgs "github.com/lyft/protoc-gen-star"
	"google.golang.org/genproto/googleapis/api/annotations"
	"google.golang.org/protobuf/proto"
)

type RuleContext struct {
	Field        pgs.Field
	Rules        proto.Message
	MessageRules *validate.MessageRules

	Typ        string
	WrapperTyp string

	OnKey            bool
	Index            string
	AccessorOverride string
}

func rulesContext(genCtx gencontext.GenContext) func(pgs.Field) (RuleContext, error) {
	return func(f pgs.Field) (out RuleContext, err error) {
		out.Field = f

		var rules validate.FieldRules
		var ok bool
		if ok, err = f.Extension(validate.E_Rules, &rules); err != nil {
			return
		} else if !ok {
			var resourceDesc annotations.ResourceDescriptor
			if ok, err = f.Message().Extension(annotations.E_Resource, &resourceDesc); err != nil {
				return
			} else if ok && string(f.Name()) == resourceNameField(resourceDesc.NameField) {
				// at this point, we have found a field that IS a resource name field but has no validation
				// we grab validation from the resource pattern
				rules.Type = &validate.FieldRules_String_{String_: basicResourceValidation(&resourceDesc)}
			}
		}

		if resource, ok := genCtx.SearchResourceReference(f); ok {
			if err = mergeResourceValidation(f, resource.NameField(), resource.ResourceDescriptor(), &rules); err != nil {
				return
			}
		}

		// genCtx.Debugf("rules for %s: %v", f.FullyQualifiedName(), rp)

		var wrapped bool
		if out.Typ, out.Rules, out.MessageRules, wrapped = resolveRules(f.Type(), &rules); wrapped {
			out.WrapperTyp = out.Typ
			out.Typ = "wrapper"
		}

		if out.Typ == "error" {
			err = fmt.Errorf("unknown rule type (%T)", rules.Type)
		}

		return
	}
}

// This guys job is to merge validation rules from the resource to the referrer in such a way that makes sense.
//
// The main nuanced case is repeated field validation. If the reference field is repeated, we want the field to be able.
// to define normal repeated field validation (eg: min/max len), which inheriting the per-item validation from the resource.
//
// Map fields are not supported. There is no way to know whether validation needs to be on the keys or values.
func mergeResourceValidation(base pgs.Field, resource pgs.Field, resourceDesc *annotations.ResourceDescriptor, rules *validate.FieldRules) error {
	var resValidation *validate.FieldRules
	emptyRules := &validate.FieldRules{}

	if ok, err := resource.Extension(validate.E_Rules, &resValidation); err != nil {
		return err
	} else if !ok {
		resValidation = &validate.FieldRules{Type: &validate.FieldRules_String_{String_: basicResourceValidation(resourceDesc)}}
	}

	// if the field we are targeting repeated and the annotation does not exist, we need to create it to fill it later
	if proto.Equal(rules, emptyRules) && base.Type().IsRepeated() {
		rules.Type = &validate.FieldRules_Repeated{Repeated: &validate.RepeatedRules{}}
	}

	switch t := rules.GetType().(type) {
	case *validate.FieldRules_Repeated:
		// This will preseve any other repeated validation already existing (eg: min/max len)
		t.Repeated.Items = resValidation
		return nil
	default:
		// we only want to override this field
		makeValidationIgnoreEmpty(resValidation)
		rules.Type = resValidation.Type
	}
	return nil
}

// build a regular expression for resource validation based on the given pattern
//
// This is not as good as having a regex provided by field validation, but its something
//
// regex is structured as a logical OR of all given patterns. For each pattern, splits it into path
// elements and detects if the path element is a variable by curly brackets. For path variables, regex
// allows any number (at least 1) character. for non variables, enforces an exact match.
func basicResourceValidation(resourceDesc *annotations.ResourceDescriptor) *validate.StringRules {
	regex := strings.Builder{}
	patterns := resourceDesc.GetPattern()

	// build the regex
	regex.WriteString(`^(`)
	for i, pattern := range patterns {
		regex.WriteString(`\A`)
		split := strings.Split(pattern, "/")
		for j, elem := range split {
			if elem[0] == '{' && elem[len(elem)-1] == '}' {
				regex.WriteString(`[^/]+`)
			} else {
				regex.WriteString(elem)
			}
			if j != len(split)-1 {
				regex.WriteRune('/')
			}
		}
		// regex += `\z`
		if i != len(patterns)-1 {
			regex.WriteString(`\z|`)
		} else {
			regex.WriteString(`\z`)
		}
	}
	regex.WriteString(`)$`)
	final := regex.String()
	ignoreEmpty := true
	return &validate.StringRules{Pattern: &final, IgnoreEmpty: &ignoreEmpty}
}

func resourceNameField(s string) string {
	if s == "" {
		return "name"
	}
	return s
}

func makeValidationIgnoreEmpty(rules *validate.FieldRules) {
	ignore := true
	switch t := rules.GetType().(type) {
	case *validate.FieldRules_String_:
		t.String_.IgnoreEmpty = &ignore
	case *validate.FieldRules_Bytes:
		t.Bytes.IgnoreEmpty = &ignore
	}
}

func (ctx RuleContext) Key(name, idx string) (out RuleContext, err error) {
	rules, ok := ctx.Rules.(*validate.MapRules)
	if !ok {
		err = fmt.Errorf("cannot get Key RuleContext from %T", ctx.Field)
		return
	}

	out.Field = ctx.Field
	out.AccessorOverride = name
	out.Index = idx

	out.Typ, out.Rules, out.MessageRules, _ = resolveRules(ctx.Field.Type().Key(), rules.GetKeys())

	if out.Typ == "error" {
		err = fmt.Errorf("unknown rule type (%T)", rules)
	}

	return
}

func (ctx RuleContext) Elem(name, idx string) (out RuleContext, err error) {
	out.Field = ctx.Field
	out.AccessorOverride = name
	out.Index = idx

	var rules *validate.FieldRules
	switch r := ctx.Rules.(type) {
	case *validate.MapRules:
		rules = r.GetValues()
	case *validate.RepeatedRules:
		rules = r.GetItems()
	default:
		err = fmt.Errorf("cannot get Elem RuleContext from %T", ctx.Field)
		return
	}

	var wrapped bool
	if out.Typ, out.Rules, out.MessageRules, wrapped = resolveRules(ctx.Field.Type().Element(), rules); wrapped {
		out.WrapperTyp = out.Typ
		out.Typ = "wrapper"
	}

	if out.Typ == "error" {
		err = fmt.Errorf("unknown rule type (%T)", rules)
	}

	return
}

func (ctx RuleContext) Unwrap(name string) (out RuleContext, err error) {
	if ctx.Typ != "wrapper" {
		err = fmt.Errorf("cannot unwrap non-wrapper type %q", ctx.Typ)
		return
	}

	return RuleContext{
		Field:            ctx.Field,
		Rules:            ctx.Rules,
		MessageRules:     ctx.MessageRules,
		Typ:              ctx.WrapperTyp,
		AccessorOverride: name,
	}, nil
}

func Render(tpl *template.Template) func(ctx RuleContext) (string, error) {
	return func(ctx RuleContext) (string, error) {
		var b bytes.Buffer
		err := tpl.ExecuteTemplate(&b, ctx.Typ, ctx)
		return b.String(), err
	}
}

func resolveRules(typ interface{ IsEmbed() bool }, rules *validate.FieldRules) (ruleType string, rule proto.Message, messageRule *validate.MessageRules, wrapped bool) {
	switch r := rules.GetType().(type) {
	case *validate.FieldRules_Float:
		ruleType, rule, wrapped = "float", r.Float, typ.IsEmbed()
	case *validate.FieldRules_Double:
		ruleType, rule, wrapped = "double", r.Double, typ.IsEmbed()
	case *validate.FieldRules_Int32:
		ruleType, rule, wrapped = "int32", r.Int32, typ.IsEmbed()
	case *validate.FieldRules_Int64:
		ruleType, rule, wrapped = "int64", r.Int64, typ.IsEmbed()
	case *validate.FieldRules_Uint32:
		ruleType, rule, wrapped = "uint32", r.Uint32, typ.IsEmbed()
	case *validate.FieldRules_Uint64:
		ruleType, rule, wrapped = "uint64", r.Uint64, typ.IsEmbed()
	case *validate.FieldRules_Sint32:
		ruleType, rule, wrapped = "sint32", r.Sint32, false
	case *validate.FieldRules_Sint64:
		ruleType, rule, wrapped = "sint64", r.Sint64, false
	case *validate.FieldRules_Fixed32:
		ruleType, rule, wrapped = "fixed32", r.Fixed32, false
	case *validate.FieldRules_Fixed64:
		ruleType, rule, wrapped = "fixed64", r.Fixed64, false
	case *validate.FieldRules_Sfixed32:
		ruleType, rule, wrapped = "sfixed32", r.Sfixed32, false
	case *validate.FieldRules_Sfixed64:
		ruleType, rule, wrapped = "sfixed64", r.Sfixed64, false
	case *validate.FieldRules_Bool:
		ruleType, rule, wrapped = "bool", r.Bool, typ.IsEmbed()
	case *validate.FieldRules_String_:
		ruleType, rule, wrapped = "string", r.String_, typ.IsEmbed()
	case *validate.FieldRules_Bytes:
		ruleType, rule, wrapped = "bytes", r.Bytes, typ.IsEmbed()
	case *validate.FieldRules_Enum:
		ruleType, rule, wrapped = "enum", r.Enum, false
	case *validate.FieldRules_Repeated:
		ruleType, rule, wrapped = "repeated", r.Repeated, false
	case *validate.FieldRules_Map:
		ruleType, rule, wrapped = "map", r.Map, false
	case *validate.FieldRules_Any:
		ruleType, rule, wrapped = "any", r.Any, false
	case *validate.FieldRules_Duration:
		ruleType, rule, wrapped = "duration", r.Duration, false
	case *validate.FieldRules_Timestamp:
		ruleType, rule, wrapped = "timestamp", r.Timestamp, false
	case nil:
		if ft, ok := typ.(pgs.FieldType); ok && ft.IsRepeated() {
			return "repeated", &validate.RepeatedRules{}, rules.Message, false
		} else if ok && ft.IsMap() && ft.Element().IsEmbed() {
			return "map", &validate.MapRules{}, rules.Message, false
		} else if typ.IsEmbed() {
			return "message", rules.GetMessage(), rules.GetMessage(), false
		}
		return "none", nil, nil, false
	default:
		ruleType, rule, wrapped = "error", nil, false
	}

	return ruleType, rule, rules.Message, wrapped
}
