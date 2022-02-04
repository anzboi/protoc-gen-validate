package gencontext

import (
	"errors"

	pgs "github.com/lyft/protoc-gen-star"
	"google.golang.org/genproto/googleapis/api/annotations"
)

type GenContext interface {
	pgs.DebuggerCommon
	SearchResourceReference(pgs.Field) (Resource, bool)
}

type Resource interface {
	Message() pgs.Message
	ResourceDescriptor() *annotations.ResourceDescriptor
	NameField() pgs.Field
}

type genCtx struct {
	pgs.Visitor
	pgs.DebuggerCommon
	resources map[string]*resource
}

func New(d pgs.DebuggerCommon, pkgs map[string]pgs.Package) *genCtx {
	g := &genCtx{
		DebuggerCommon: d,
		resources:      make(map[string]*resource),
	}
	g.Visitor = pgs.PassThroughVisitor(g)
	for _, pkg := range pkgs {
		pgs.Walk(g, pkg)
	}
	return g
}

func (g *genCtx) SearchResourceReference(f pgs.Field) (Resource, bool) {
	var ref *annotations.ResourceReference
	if ok, err := f.Extension(annotations.E_ResourceReference, &ref); err != nil || !ok {
		return nil, false
	}
	msg, ok := g.resources[ref.Type]
	return msg, ok
}

func (g *genCtx) VisitMessage(msg pgs.Message) (pgs.Visitor, error) {
	var res annotations.ResourceDescriptor
	if ok, err := msg.Extension(annotations.E_Resource, &res); err != nil {
		return g, err
	} else if !ok {
		return g, nil
	}

	resource := &resource{
		msg:  msg,
		desc: &res,
	}

	// Find the field corresponding
	nameField := "name"
	if res.NameField != "" {
		nameField = res.NameField
	}
	for _, field := range msg.Fields() {
		if field.Name() == pgs.Name(nameField) {
			resource.nameField = field
			break
		}
	}
	if resource.nameField == nil {
		return nil, errors.New("resource does not have a name field")
	}

	// return
	g.resources[res.Type] = resource
	return g, nil
}

type resource struct {
	msg       pgs.Message
	desc      *annotations.ResourceDescriptor
	nameField pgs.Field
}

func (r *resource) Message() pgs.Message {
	return r.msg
}

func (r *resource) ResourceDescriptor() *annotations.ResourceDescriptor {
	return r.desc
}

func (r *resource) NameField() pgs.Field {
	return r.nameField
}
