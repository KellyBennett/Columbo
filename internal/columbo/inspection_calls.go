package columbo

import (
	"go/ast"
	"go/types"
)

type inspectionCall struct {
	ReceiverForm string
	inspectionSpan
	Expression   string
	Resolution   string
	Target       *inspectionDeclaration
	Receiver     *inspectionSpan
	Binding      *inspectionSpan
	BindingLimit string
}

func (a *engine) inspectionCalls(d *declaration) []inspectionCall {
	calls := []inspectionCall{}
	d.inspectBody(func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if ok {
			calls = append(calls, a.inspectionCall(d, call))
		}
		return true
	})
	return calls
}
func (a *engine) inspectionCall(d *declaration, call *ast.CallExpr) inspectionCall {
	span := d.file.inspectionSpan(call)
	result := inspectionCall{inspectionSpan: span, Expression: string(d.file.data[span.StartOffset:span.EndOffset]), Resolution: "unresolved or outside selected local method scope", BindingLimit: "Static receiver expression and lexical identifier binding only; storage/alias origin, returned handle lifetime and effects require source inspection."}
	target := a.calls[call]
	if target != nil && target.hasReceiver() {
		decl := target.inspectionDeclaration()
		result.Target = &decl
		result.Resolution = "source-confirmed local method; one-hop body witness only"
	}
	selector, ok := calleeBase(call.Fun).(*ast.SelectorExpr)
	if !ok {
		return result
	}
	selection := d.file.typeInfo().Selections[selector]
	if selection == nil || selection.Kind() == types.FieldVal {
		return result
	}
	result.ReceiverForm = "method-value-or-invocation"
	if selection.Kind() == types.MethodExpr {
		result.ReceiverForm = "method-expression"
		result.BindingLimit += " Selector receiver syntax is a type expression; the actual receiver argument is not separately resolved."
	}
	receiver := d.file.inspectionSpan(selector.X)
	result.Receiver = &receiver
	result.Binding = a.inspectionBinding(d.file, selector.X)
	return result
}

func (a *engine) inspectionBinding(source *file, receiver ast.Expr) *inspectionSpan {
	id, ok := receiver.(*ast.Ident)
	if !ok {
		return nil
	}
	obj := source.typeInfo().ObjectOf(id)
	if obj == nil || !obj.Pos().IsValid() {
		return nil
	}
	for _, f := range a.files {
		if obj.Pos() < f.ast.Pos() || obj.Pos() > f.ast.End() {
			continue
		}
		name := &ast.Ident{NamePos: obj.Pos(), Name: obj.Name()}
		binding := f.inspectionSpan(name)
		return &binding
	}
	return nil
}
