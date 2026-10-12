package columbo

import (
	"context"
	"fmt"
	"sort"

	"github.com/KellyBennett/Columbo/internal/snapshotdb"
)

type inspectionRangeKey struct {
	path       string
	start, end int
}

type inspectionSavedRange struct {
	id int64
	inspectionSpan
}

type inspectionSnapshotWriter struct {
	*snapshotWriter
	files  map[string]inspectionFile
	types  map[string]int64
	ranges map[inspectionRangeKey]inspectionSavedRange
}

func (w *snapshotWriter) writeInspection(facts *inspectionFacts) {
	if facts == nil || w.err != nil {
		return
	}
	inspection := inspectionSnapshotWriter{snapshotWriter: w, files: map[string]inspectionFile{}, types: map[string]int64{}, ranges: map[inspectionRangeKey]inspectionSavedRange{}}
	facts.writeSnapshot(&inspection)
}

func (facts inspectionFacts) writeSnapshot(w *inspectionSnapshotWriter) {
	w.writeIdentity(facts)
	w.writeEnvironment(facts.Environment)
	w.writePatterns(facts.Patterns)
	w.writeFiles(facts.Files)
	w.writeConfig(facts)
	w.writeTypes(facts.Types)
	w.writeDeclarations(facts.Declarations)
}

func (facts inspectionFacts) snapshotIdentity() (snapshotdb.InsertSourceScanParams, error) {
	if facts.SourceSHA != inspectionIdentity(inspectionOrderedFiles(facts.Files)) {
		return snapshotdb.InsertSourceScanParams{}, fmt.Errorf("inspection source identity does not match source inventory")
	}
	return snapshotdb.InsertSourceScanParams{Root: facts.Root, GoVersion: facts.GoVersion, SourceSha256: facts.SourceSHA}, nil
}

func (w *inspectionSnapshotWriter) writeIdentity(facts inspectionFacts) {
	params, err := facts.snapshotIdentity()
	w.record(err)
	w.exec(func() error { return w.queries.InsertSourceScan(context.Background(), params) })
}

func (w *inspectionSnapshotWriter) writeEnvironment(environment map[string]string) {
	writeInspectionMap(w, environment, func(name, value string) error {
		return w.queries.InsertSourceEnvironment(context.Background(), snapshotdb.InsertSourceEnvironmentParams{Name: name, Value: value})
	})
}

func (w *inspectionSnapshotWriter) writePatterns(patterns []string) {
	for ordinal, pattern := range patterns {
		w.exec(func() error {
			return w.queries.InsertSourcePattern(context.Background(), snapshotdb.InsertSourcePatternParams{Ordinal: int64(ordinal), Pattern: pattern})
		})
	}
}

func inspectionSortedKeys[T any](items map[string]T) []string {
	keys := make([]string, 0, len(items))
	for key := range items {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func inspectionOrderedFiles(files []inspectionFile) []inspectionFile {
	ordered := append([]inspectionFile{}, files...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].Path < ordered[j].Path })
	return ordered
}

func (w *inspectionSnapshotWriter) writeFiles(files []inspectionFile) {
	for _, file := range inspectionOrderedFiles(files) {
		id := w.file(file.Path)
		w.exec(func() error {
			return w.queries.InsertSourceFile(context.Background(), snapshotdb.InsertSourceFileParams{FileID: id, Sha256: file.SHA, Size: file.Size, Package: file.Package, SelectedRuntime: snapshotFlag(file.Selected), OrdinaryTarget: snapshotFlag(file.Target), Generated: snapshotFlag(file.Generated), Scope: file.Scope})
		})
		w.files[file.Path] = file
	}
}

func (w *inspectionSnapshotWriter) sourceFile(span inspectionSpan) int64 {
	file, ok := w.files[span.Path]
	w.require(ok, "inspection source file is absent from inventory: "+span.Path)
	w.record(span.validateFile(file))
	return w.file(span.Path)
}

func (span inspectionSpan) validateFile(file inspectionFile) error {
	if file.SHA != span.FileSHA {
		return fmt.Errorf("inspection range does not match source inventory: %s", span.Path)
	}
	if !file.Selected {
		return fmt.Errorf("inspection range is outside selected runtime: %s", span.Path)
	}
	if span.StartOffset < 0 || span.EndOffset < span.StartOffset || int64(span.EndOffset) > file.Size {
		return fmt.Errorf("inspection range exceeds source file: %s", span.Path)
	}
	return nil
}

func (w *inspectionSnapshotWriter) sourceRange(span inspectionSpan) int64 {
	file := w.sourceFile(span)
	key := inspectionRangeKey{span.Path, span.StartOffset, span.EndOffset}
	if previous, ok := w.ranges[key]; ok {
		w.require(previous.inspectionSpan == span, "conflicting inspection source ranges: "+span.Path)
		return previous.id
	}
	id := w.insert(func() (int64, error) {
		return w.queries.InsertSourceRange(context.Background(), snapshotdb.InsertSourceRangeParams{FileID: file, StartLine: int64(span.StartLine), EndLine: int64(span.EndLine), StartOffset: int64(span.StartOffset), EndOffset: int64(span.EndOffset), Sha256: span.SHA})
	})
	w.ranges[key] = inspectionSavedRange{id, span}
	return id
}

func (w *inspectionSnapshotWriter) optionalRange(span *inspectionSpan) *int64 {
	if span == nil {
		return nil
	}
	id := w.sourceRange(*span)
	return &id
}

func (w *inspectionSnapshotWriter) writeTypes(types []inspectionType) {
	ordered := append([]inspectionType{}, types...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].Key < ordered[j].Key })
	for _, typ := range ordered {
		file, span := w.sourceFile(typ.inspectionSpan), w.sourceRange(typ.inspectionSpan)
		w.types[typ.Key] = w.insert(func() (int64, error) {
			return w.queries.InsertSourceType(context.Background(), snapshotdb.InsertSourceTypeParams{TypeKey: typ.Key, ResolvedKey: typ.ResolvedKey, Alias: snapshotFlag(typ.Alias), FileID: file, RangeID: span})
		})
	}
	for _, typ := range ordered {
		for ordinal, field := range typ.Fields {
			w.writeField(w.types[typ.Key], ordinal, field)
		}
	}
}

func (w *inspectionSnapshotWriter) optionalType(key string) *int64 {
	if id, ok := w.types[key]; ok {
		return &id
	}
	return nil
}

func (w *inspectionSnapshotWriter) writeField(owner int64, ordinal int, field inspectionField) {
	params := snapshotdb.InsertSourceFieldParams{TypeID: owner, FileID: w.sourceFile(field.inspectionSpan), Ordinal: int64(ordinal), RangeID: w.sourceRange(field.inspectionSpan), Embedded: snapshotFlag(field.Embedded), TypeText: field.TypeText, TypeKey: snapshotOptionalText(field.TypeKey), TargetTypeID: w.optionalType(field.TypeKey)}
	id := w.insert(func() (int64, error) { return w.queries.InsertSourceField(context.Background(), params) })
	for ordinal, name := range field.Names {
		w.exec(func() error {
			return w.queries.InsertSourceFieldName(context.Background(), snapshotdb.InsertSourceFieldNameParams{FieldID: id, Ordinal: int64(ordinal), Name: name})
		})
	}
}

func (w *inspectionSnapshotWriter) writeDeclarations(declarations []inspectionDeclaration) {
	ordered := append([]inspectionDeclaration{}, declarations...)
	sort.Slice(ordered, func(i, j int) bool {
		return declarationRefLess(DeclarationRef{File: ordered[i].Path, Symbol: ordered[i].Symbol}, DeclarationRef{File: ordered[j].Path, Symbol: ordered[j].Symbol})
	})
	for _, declaration := range ordered {
		w.writeDeclaration(declaration)
	}
	for _, declaration := range ordered {
		id := w.declaration(DeclarationRef{File: declaration.Path, Symbol: declaration.Symbol})
		for ordinal, call := range declaration.Calls {
			w.writeCall(id, ordinal, call)
		}
	}
}

func (w *inspectionSnapshotWriter) writeDeclaration(declaration inspectionDeclaration) {
	id := w.declaration(DeclarationRef{File: declaration.Path, Symbol: declaration.Symbol})
	file := w.sourceFile(declaration.inspectionSpan)
	w.checkDeclarationRange(id, file, declaration.inspectionSpan)
	receiver := w.optionalType(declaration.ReceiverKey)
	w.require(declaration.Kind != "method" || receiver != nil, "inspection method has no selected source receiver type: "+declaration.Symbol)
	w.exec(func() error {
		return w.queries.InsertSourceDeclaration(context.Background(), snapshotdb.InsertSourceDeclarationParams{DeclarationID: id, FileID: file, Kind: declaration.Kind, ReceiverTypeID: receiver, Sha256: declaration.SHA})
	})
}

func (w *inspectionSnapshotWriter) checkDeclarationRange(id, file int64, span inspectionSpan) {
	if w.err != nil {
		return
	}
	stored, err := w.queries.InspectionDeclarationLocation(context.Background(), snapshotdb.InspectionDeclarationLocationParams{ID: id})
	w.record(err)
	w.require(stored.FileID == file && stored.StartLine == int64(span.StartLine) && stored.EndLine == int64(span.EndLine) && stored.StartOffset == int64(span.StartOffset) && stored.EndOffset == int64(span.EndOffset), "inspection declaration range differs from ordinary evidence")
}

func (w *inspectionSnapshotWriter) writeCall(owner int64, ordinal int, call inspectionCall) {
	params := snapshotdb.InsertSourceCallParams{DeclarationID: owner, FileID: w.sourceFile(call.inspectionSpan), Ordinal: int64(ordinal), RangeID: w.sourceRange(call.inspectionSpan), Expression: call.Expression, Resolution: call.Resolution, ReceiverForm: call.ReceiverForm, ReceiverRangeID: w.optionalRange(call.Receiver), BindingRangeID: w.optionalRange(call.Binding), BindingLimit: call.BindingLimit}
	if call.Target != nil {
		id := w.declaration(DeclarationRef{File: call.Target.Path, Symbol: call.Target.Symbol})
		w.checkDeclarationRange(id, w.sourceFile(call.Target.inspectionSpan), call.Target.inspectionSpan)
		params.TargetDeclarationID = &id
	}
	w.exec(func() error { return w.queries.InsertSourceCall(context.Background(), params) })
}
