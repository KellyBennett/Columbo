package columbo

import (
	"context"
	"fmt"
	"github.com/KellyBennett/Columbo/internal/snapshotdb"
	"math"
	"reflect"
	"sort"
)

type snapshotClusterKey struct{ caseID, key string }
type snapshotMemberKey struct {
	caseID string
	ref    MemberRef
}
type snapshotReceiptKey struct{ caseID, key string }

type snapshotWriter struct {
	queries      snapshotdb.Querier
	err          error
	files        map[string]int64
	declarations map[DeclarationRef]int64
	dependencies map[string]int64
	clusters     map[snapshotClusterKey]int64
	members      map[snapshotMemberKey]int64
	receipts     map[snapshotReceiptKey]int64
	commits      map[string]int64
	policies     map[string]PolicyReview
}

func newSnapshotWriter(queries snapshotdb.Querier) *snapshotWriter {
	return &snapshotWriter{queries: queries, files: map[string]int64{}, declarations: map[DeclarationRef]int64{}, dependencies: map[string]int64{}, clusters: map[snapshotClusterKey]int64{}, members: map[snapshotMemberKey]int64{}, receipts: map[snapshotReceiptKey]int64{}, commits: map[string]int64{}, policies: map[string]PolicyReview{}}
}
func (w *snapshotWriter) write(report Report, version string) error {
	w.writeHeader(version)
	w.writeDeclarations(report.Declarations)
	for ordinal, c := range report.Cases {
		w.writeCase(c, ordinal)
	}
	report.writeEvidence(w)
	w.writeSuppressions(report.Suppressions)
	w.writeWarnings(report.Warnings)
	return w.err
}
func (w *snapshotWriter) writeHeader(version string) {
	w.exec(func() error {
		return w.queries.InsertReport(context.Background(), snapshotdb.InsertReportParams{SchemaVersion: int64(SchemaVersion), ColumboVersion: version})
	})
}
func (w *snapshotWriter) record(err error) {
	if w.err == nil {
		w.err = err
	}
}
func (w *snapshotWriter) require(condition bool, message string) {
	if !condition {
		w.record(fmt.Errorf("%s", message))
	}
}
func (w *snapshotWriter) exec(write func() error) {
	if w.err == nil {
		w.record(write())
	}
}
func (w *snapshotWriter) insert(write func() (int64, error)) int64 {
	if w.err != nil {
		return 0
	}
	id, err := write()
	w.record(err)
	return id
}
func (w *snapshotWriter) file(path string) int64 {
	if id, ok := w.files[path]; ok {
		return id
	}
	id := w.insert(func() (int64, error) {
		return w.queries.InsertFile(context.Background(), snapshotdb.InsertFileParams{Path: path})
	})
	w.files[path] = id
	return id
}
func (w *snapshotWriter) declaration(ref DeclarationRef) int64 {
	id, ok := w.declarations[ref]
	w.require(ok, "missing typed declaration "+ref.Symbol+" in "+ref.File)
	return id
}
func (w *snapshotWriter) optionalDeclaration(ref *DeclarationRef) *int64 {
	if ref == nil {
		return nil
	}
	id := w.declaration(*ref)
	return &id
}
func (w *snapshotWriter) dependency(identity string) int64 {
	if id, ok := w.dependencies[identity]; ok {
		return id
	}
	id := w.insert(func() (int64, error) {
		return w.queries.InsertDependency(context.Background(), snapshotdb.InsertDependencyParams{Identity: identity})
	})
	w.dependencies[identity] = id
	return id
}
func (w *snapshotWriter) writeDeclarations(evidence []DeclarationEvidence) {
	ordered := append([]DeclarationEvidence{}, evidence...)
	sort.Slice(ordered, func(i, j int) bool { return declarationRefLess(ordered[i].Ref, ordered[j].Ref) })
	for _, d := range ordered {
		w.writeDeclaration(d)
	}
}
func declarationRefLess(left, right DeclarationRef) bool {
	if left.File != right.File {
		return left.File < right.File
	}
	return left.Symbol < right.Symbol
}
func (w *snapshotWriter) writeDeclaration(d DeclarationEvidence) {
	file := w.file(d.Ref.File)
	params, err := d.snapshotDeclaration(file)
	w.record(err)
	id := w.insert(func() (int64, error) { return w.queries.InsertDeclaration(context.Background(), params) })
	w.registerDeclaration(d, id)
}
func (d DeclarationEvidence) snapshotDeclaration(fileID int64) (snapshotdb.InsertDeclarationParams, error) {
	s := d.Source
	if s.File != d.Ref.File {
		return snapshotdb.InsertDeclarationParams{}, fmt.Errorf("declaration source file does not match typed reference")
	}
	return snapshotdb.InsertDeclarationParams{FileID: fileID, Symbol: d.Ref.Symbol, StartLine: int64(s.StartLine), EndLine: int64(s.EndLine), StartOffset: int64(s.StartOffset), EndOffset: int64(s.EndOffset)}, nil
}
func (w *snapshotWriter) registerDeclaration(d DeclarationEvidence, id int64) {
	w.declarations[d.Ref] = id
	w.writeDeclarationDependencies(id, d.Dependencies)
}
func (w *snapshotWriter) writeDeclarationDependencies(declarationID int64, evidence []DependencyEvidence) {
	ordered := append([]DependencyEvidence{}, evidence...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].Identity < ordered[j].Identity })
	for _, dep := range ordered {
		dependencyID := w.dependency(dep.Identity)
		w.exec(func() error {
			return w.queries.InsertDeclarationDependency(context.Background(), snapshotdb.InsertDeclarationDependencyParams{DeclarationID: int64(declarationID), DependencyID: int64(dependencyID), Scored: snapshotFlag(dep.Scored)})
		})
	}
}
func (w *snapshotWriter) writeCase(c Case, ordinal int) {
	c.writeSnapshot(w, ordinal)
	w.writeCaseEvidence(c)
}
func (c Case) writeSnapshot(w *snapshotWriter, ordinal int) {
	w.require(c.PrimaryDeclaration != nil, "missing typed primary declaration")
	if c.PrimaryDeclaration == nil {
		return
	}
	id := w.declaration(*c.PrimaryDeclaration)
	w.require(c.File == c.PrimaryDeclaration.File && c.Symbol == c.PrimaryDeclaration.Symbol, "case primary declaration does not match location")
	w.exec(func() error {
		return w.queries.InsertCase(context.Background(), snapshotdb.InsertCaseParams{ID: c.ID, Ordinal: int64(ordinal), PrimaryDeclarationID: int64(id), Smell: c.Smell, Verdict: c.Verdict, Suppressed: snapshotFlag(c.Suppressed), StartLine: int64(c.StartLine), EndLine: int64(c.EndLine), Why: c.Why, Diagnosis: c.Diagnosis})
	})
	w.caseDeclaration(c.ID, *c.PrimaryDeclaration, "primary")
}
func (w *snapshotWriter) writeCaseEvidence(c Case) {
	for _, d := range c.SupportingDeclarations {
		w.caseDeclaration(c.ID, d.Declaration, d.Role)
	}
	w.writeGuidance(c)
	w.writeClusters(c)
	w.writeReceipts(c)
	w.writeClues(c)
	w.writePolicies(c)
}
func (w *snapshotWriter) caseDeclaration(caseID string, ref DeclarationRef, role string) {
	id := w.declaration(ref)
	w.exec(func() error {
		return w.queries.LinkCaseDeclaration(context.Background(), snapshotdb.LinkCaseDeclarationParams{CaseID: caseID, DeclarationID: int64(id), Role: role})
	})
}
func (w *snapshotWriter) writeGuidance(c Case) {
	groups := []struct {
		kind  string
		items []string
	}{{"lead", c.Leads}, {"avoid", c.Avoid}}
	for _, group := range groups {
		for ordinal, item := range group.items {
			w.exec(func() error {
				return w.queries.InsertGuidance(context.Background(), snapshotdb.InsertGuidanceParams{CaseID: c.ID, Kind: group.kind, Ordinal: int64(ordinal), Item: item})
			})
		}
	}
}
func (w *snapshotWriter) writeClusters(c Case) {
	for ordinal, cluster := range c.Clusters {
		w.writeCluster(c.ID, ordinal, cluster)
	}
}
func (w *snapshotWriter) writeCluster(caseID string, ordinal int, cluster Cluster) {
	owner := cluster.snapshotOwner(w)
	file := w.file(cluster.File)
	id := w.insert(func() (int64, error) {
		return w.queries.InsertCluster(context.Background(), snapshotdb.InsertClusterParams{CaseID: caseID, ClusterKey: cluster.Key, OwnerDeclarationID: int64(owner), FileID: int64(file), Ordinal: int64(ordinal)})
	})
	w.clusters[snapshotClusterKey{caseID, cluster.Key}] = id
	for index, member := range cluster.Members {
		w.writeMember(snapshotClusterKey{caseID, cluster.Key}, id, index, member)
	}
}
func (cluster Cluster) snapshotOwner(w *snapshotWriter) int64 {
	w.require(cluster.OwnerDeclaration != nil, "cluster has no typed owner")
	if cluster.OwnerDeclaration == nil {
		return 0
	}
	w.require(cluster.Owner == cluster.OwnerDeclaration.Symbol && cluster.File == cluster.OwnerDeclaration.File, "cluster owner does not match typed declaration")
	return w.declaration(*cluster.OwnerDeclaration)
}
func (w *snapshotWriter) writeMember(cluster snapshotClusterKey, clusterID int64, ordinal int, member Member) {
	helper := w.optionalDeclaration(member.Declaration)
	w.require(member.Declaration != nil, "cluster member has no typed helper")
	if member.Declaration == nil {
		return
	}
	w.require(member.Helper == member.Declaration.Symbol, "member helper does not match typed declaration")
	id := w.insert(func() (int64, error) {
		return w.queries.InsertClusterMember(context.Background(), snapshotdb.InsertClusterMemberParams{ClusterID: int64(clusterID), CaseID: cluster.caseID, HelperDeclarationID: *helper, Ordinal: int64(ordinal), CallOffset: int64(member.CallOffset)})
	})
	w.registerMember(cluster, member, id)
}
func (w *snapshotWriter) registerMember(cluster snapshotClusterKey, member Member, id int64) {
	w.members[snapshotMemberKey{cluster.caseID, MemberRef{ClusterKey: cluster.key, Helper: *member.Declaration, CallOffset: member.CallOffset}}] = id
}
func (w *snapshotWriter) writeReceipts(c Case) {
	historyOrdinal := 0
	for ordinal, receipt := range c.Receipts {
		switch r := receipt.(type) {
		case Source:
			w.writeSource(c.ID, ordinal, r)
		case History:
			w.writeHistory(c.ID, historyOrdinal, r)
			historyOrdinal++
		default:
			w.record(fmt.Errorf("unsupported receipt type %T", receipt))
		}
	}
}
func (w *snapshotWriter) writeSource(caseID string, ordinal int, source Source) {
	params, err := source.snapshotSource(caseID, ordinal, w.file(source.File))
	w.record(err)
	params.SourceDeclarationID = w.optionalDeclaration(source.Declaration)
	id := w.insert(func() (int64, error) { return w.queries.InsertSourceReceipt(context.Background(), params) })
	w.sourceRelations(snapshotReceiptKey{caseID, source.EvidenceKey}, id, source)
}
func (s Source) snapshotSource(caseID string, ordinal int, fileID int64) (snapshotdb.InsertSourceReceiptParams, error) {
	numeric, err := snapshotNumericValue(s.Detail.Value, true)
	if err != nil {
		return snapshotdb.InsertSourceReceiptParams{}, err
	}
	nesting, err := snapshotNesting(s.Detail.Nesting)
	if err != nil {
		return snapshotdb.InsertSourceReceiptParams{}, err
	}
	return snapshotdb.InsertSourceReceiptParams{CaseID: caseID, Ordinal: int64(ordinal), Kind: s.Kind, FileID: fileID, StartLine: int64(s.StartLine), EndLine: int64(s.EndLine), StartOffset: int64(s.StartOffset), EndOffset: int64(s.EndOffset), Subject: s.Detail.Subject, Spelling: s.Spelling, ValueType: snapshotOptionalText(numeric.kind), Value: numeric.value, Nesting: nesting}, nil
}
func (w *snapshotWriter) sourceRelations(key snapshotReceiptKey, id int64, source Source) {
	if key.key != "" {
		_, exists := w.receipts[key]
		w.require(!exists, "duplicate source evidence key")
		w.receipts[key] = id
	}
	w.writeExpansionDeclarations(id, source.Detail)
	w.writeExpansionSites(id, source.Detail.ExpansionSites)
	if source.DependencyIdentity != "" {
		w.writeDependencyReceipt(key.caseID, id, source)
	}
}
func (w *snapshotWriter) writeDependencyReceipt(caseID string, id int64, source Source) {
	params := source.snapshotDependency(w)
	w.validateDependency(params, source.DependencyScored)
	w.exec(func() error {
		return w.queries.InsertDependencyReceipt(context.Background(), snapshotdb.InsertDependencyReceiptParams{DeclarationID: params.DeclarationID, DependencyID: params.DependencyID, ReceiptID: id, CaseID: caseID})
	})
}
func (source Source) snapshotDependency(w *snapshotWriter) snapshotdb.DependencyScoredParams {
	declarationID := w.optionalDeclaration(source.Declaration)
	w.require(declarationID != nil, "dependency receipt has no typed declaration")
	dependencyID := w.dependency(source.DependencyIdentity)
	if declarationID == nil {
		return snapshotdb.DependencyScoredParams{}
	}
	return snapshotdb.DependencyScoredParams{DeclarationID: *declarationID, DependencyID: dependencyID}
}
func (w *snapshotWriter) validateDependency(params snapshotdb.DependencyScoredParams, expected bool) {
	if w.err != nil {
		return
	}
	scored, err := w.queries.DependencyScored(context.Background(), params)
	w.record(err)
	w.require((scored != 0) == expected, "dependency receipt disagrees with scored inventory")
}
func (w *snapshotWriter) writeExpansionDeclarations(receiptID int64, detail Detail) {
	w.require(len(detail.Expansion) == len(detail.ExpansionDeclarations), "expansion declarations lack typed references")
	if w.err != nil {
		return
	}
	for ordinal, ref := range detail.ExpansionDeclarations {
		id := w.declaration(ref)
		w.require(detail.Expansion[ordinal] == ref.Symbol, "expansion name does not match typed declaration")
		w.exec(func() error {
			return w.queries.InsertExpansionDeclaration(context.Background(), snapshotdb.InsertExpansionDeclarationParams{ReceiptID: int64(receiptID), Ordinal: int64(ordinal), DeclarationID: int64(id), Symbol: detail.Expansion[ordinal]})
		})
	}
}
func (w *snapshotWriter) writeExpansionSites(receiptID int64, sites []Site) {
	for ordinal, site := range sites {
		file := w.file(site.File)
		owner := w.optionalDeclaration(site.Owner)
		w.exec(func() error {
			return w.queries.InsertExpansionSite(context.Background(), snapshotdb.InsertExpansionSiteParams{ReceiptID: int64(receiptID), Ordinal: int64(ordinal), FileID: int64(file), CallOffset: int64(site.CallOffset), OwnerDeclarationID: owner})
		})
	}
}
func (w *snapshotWriter) writeClues(c Case) {
	for ordinal, clue := range c.Clues {
		w.writeClue(c.ID, ordinal, clue)
	}
}
func (w *snapshotWriter) writeClue(caseID string, ordinal int, clue Clue) {
	value, err := snapshotClueValue(clue.Value)
	w.record(err)
	limit := w.number(clue.Limit, true)
	operator := w.nullableText(clue.Operator)
	refs := w.clueReferences(caseID, clue)
	id := w.insert(func() (int64, error) {
		return w.queries.InsertClue(context.Background(), snapshotdb.InsertClueParams{CaseID: caseID, Ordinal: int64(ordinal), Kind: clue.Kind, Subject: clue.Subject, ValueType: value.kind, NumericValue: value.value, LimitType: snapshotOptionalText(limit.kind), LimitValue: limit.value, Operator: operator, DeclarationID: refs.declaration, ClusterID: refs.cluster, MemberID: refs.member, PairLeftMemberID: refs.left, PairRightMemberID: refs.right})
	})
	w.writeClueValues(id, value.items)
	w.writeClueReceipts(caseID, id, clue.SupportingReceipts)
}
func (w *snapshotWriter) number(value any, nullable bool) snapshotNumeric {
	number, err := snapshotNumericValue(value, nullable)
	w.record(err)
	return number
}
func (w *snapshotWriter) writeClueValues(id int64, values []string) {
	for ordinal, value := range values {
		w.exec(func() error {
			return w.queries.InsertClueValue(context.Background(), snapshotdb.InsertClueValueParams{ClueID: int64(id), Ordinal: int64(ordinal), Value: value})
		})
	}
}
func (w *snapshotWriter) writeClueReceipts(caseID string, clueID int64, keys []string) {
	for _, key := range keys {
		receiptID, ok := w.receipts[snapshotReceiptKey{caseID, key}]
		w.require(ok, "clue references missing typed receipt")
		w.exec(func() error {
			return w.queries.LinkClueReceipt(context.Background(), snapshotdb.LinkClueReceiptParams{CaseID: caseID, ClueID: int64(clueID), ReceiptID: int64(receiptID)})
		})
	}
}

type snapshotClueReferences struct{ declaration, cluster, member, left, right *int64 }

func (w *snapshotWriter) clueReferences(caseID string, clue Clue) snapshotClueReferences {
	refs := snapshotClueReferences{declaration: w.optionalDeclaration(clue.Declaration)}
	if clue.ClusterKey != "" {
		refs.cluster = w.clueCluster(caseID, clue.ClusterKey)
	}
	if clue.Member != nil {
		refs.member = w.clueMember(caseID, *clue.Member)
	}
	refs.left, refs.right = w.cluePair(caseID, clue.Pair)
	return refs
}
func (w *snapshotWriter) clueCluster(caseID, key string) *int64 {
	id, ok := w.clusters[snapshotClusterKey{caseID, key}]
	w.require(ok, "clue references missing typed cluster")
	return &id
}
func (w *snapshotWriter) clueMember(caseID string, ref MemberRef) *int64 {
	id, ok := w.members[snapshotMemberKey{caseID, ref}]
	w.require(ok, "clue references missing typed cluster member")
	return &id
}
func (w *snapshotWriter) cluePair(caseID string, pair []MemberRef) (*int64, *int64) {
	if len(pair) == 0 {
		return nil, nil
	}
	w.require(len(pair) == 2, "pair overlap requires two typed members")
	if len(pair) != 2 {
		return nil, nil
	}
	return w.clueMember(caseID, pair[0]), w.clueMember(caseID, pair[1])
}
func (w *snapshotWriter) writeHistory(caseID string, ordinal int, history History) {
	w.writeCommit(history)
	w.exec(func() error {
		return w.queries.InsertHistory(context.Background(), snapshotdb.InsertHistoryParams{CaseID: caseID, Ordinal: int64(ordinal), CommitHash: history.Commit, Kind: history.Kind})
	})
	for index, path := range history.Files {
		file := w.file(path)
		w.exec(func() error {
			return w.queries.InsertHistoryFile(context.Background(), snapshotdb.InsertHistoryFileParams{CaseID: caseID, HistoryOrdinal: int64(ordinal), Ordinal: int64(index), FileID: int64(file)})
		})
	}
}
func (w *snapshotWriter) writeCommit(history History) {
	if stamp, ok := w.commits[history.Commit]; ok {
		w.require(stamp == history.CommittedAt, "conflicting commit timestamps")
		return
	}
	w.exec(func() error {
		return w.queries.InsertCommit(context.Background(), snapshotdb.InsertCommitParams{Hash: history.Commit, CommittedAt: int64(history.CommittedAt)})
	})
	w.commits[history.Commit] = history.CommittedAt
}
func (w *snapshotWriter) writePolicies(c Case) {
	for ordinal, policy := range c.PolicyReviews {
		w.policy(policy)
		w.exec(func() error {
			return w.queries.LinkPolicy(context.Background(), snapshotdb.LinkPolicyParams{CaseID: c.ID, Ordinal: int64(ordinal), PolicyID: policy.ID})
		})
	}
}
func (w *snapshotWriter) policy(policy PolicyReview) {
	if previous, ok := w.policies[policy.ID]; ok {
		w.require(previous == policy, "conflicting policy review "+policy.ID)
		return
	}
	w.exec(func() error {
		return w.queries.InsertPolicy(context.Background(), snapshotdb.InsertPolicyParams{ID: policy.ID, Note: policy.Note, ReviewPrompt: policy.ReviewPrompt})
	})
	w.policies[policy.ID] = policy
}
func (w *snapshotWriter) writeSuppressions(suppressions []Suppression) {
	for ordinal, suppression := range suppressions {
		suppression.writeSnapshot(w, ordinal)
	}
}
func (s Suppression) writeSnapshot(w *snapshotWriter, ordinal int) {
	file := w.file(s.File)
	caseID := w.nullableText(s.CaseID)
	w.exec(func() error {
		return w.queries.InsertSuppression(context.Background(), snapshotdb.InsertSuppressionParams{Ordinal: int64(ordinal), Smell: s.Smell, Symbol: s.Symbol, FileID: int64(file), Line: int64(s.Line), Justification: s.Justification, Applied: snapshotFlag(s.Applied), CaseID: caseID})
	})
}
func (w *snapshotWriter) writeWarnings(warnings []Warning) {
	for ordinal, warning := range warnings {
		warning.writeSnapshot(w, ordinal)
	}
}
func (warning Warning) writeSnapshot(w *snapshotWriter, ordinal int) {
	var file *int64
	if warning.File != "" {
		id := w.file(warning.File)
		file = &id
	}
	w.exec(func() error {
		return w.queries.InsertWarning(context.Background(), snapshotdb.InsertWarningParams{Ordinal: int64(ordinal), Code: warning.Code, FileID: file, Line: int64(warning.Line), Message: warning.Message})
	})
}

type snapshotNumeric struct {
	kind  string
	value any
	items []string
	err   error
}

func snapshotNumericValue(value any, nullable bool) (snapshotNumeric, error) {
	number := snapshotNumeric{value: value}
	number.classify(nullable)
	return number, number.err
}
func (n *snapshotNumeric) classify(nullable bool) {
	switch n.value.(type) {
	case nil:
		n.allowNull(nullable)
	case int, int8, int16, int32, int64:
		n.kind = "integer"
	case float32, float64:
		n.kind = "real"
		n.err = finiteSnapshotNumber(n.value)
	default:
		n.err = fmt.Errorf("unsupported numeric value %T", n.value)
	}
}
func (n *snapshotNumeric) allowNull(nullable bool) {
	if !nullable {
		n.err = fmt.Errorf("null numeric value")
	}
}
func finiteSnapshotNumber(value any) error {
	var number float64
	switch v := value.(type) {
	case float32:
		number = float64(v)
	case float64:
		number = v
	}
	if math.IsNaN(number) || math.IsInf(number, 0) {
		return fmt.Errorf("non-finite numeric value")
	}
	return nil
}
func snapshotClueValue(value any) (snapshotNumeric, error) {
	if items, ok := value.([]string); ok {
		return snapshotNumeric{kind: "list", items: items}, nil
	}
	return snapshotNumericValue(value, false)
}
func snapshotNesting(value any) (*int64, error) {
	number, err := snapshotNumericValue(value, true)
	if err != nil {
		return nil, err
	}
	if number.kind != "" && number.kind != "integer" {
		return nil, fmt.Errorf("nesting must be an integer")
	}
	if number.kind == "" {
		return nil, nil
	}
	return snapshotIntegerPointer(number.value), nil
}

func snapshotFlag(value bool) int64 {
	if value {
		return 1
	}
	return 0
}
func snapshotOptionalText(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
func (w *snapshotWriter) nullableText(value any) *string {
	text, err := snapshotNullableText(value)
	w.record(err)
	return text
}
func snapshotNullableText(value any) (*string, error) {
	if value == nil {
		return nil, nil
	}
	text, ok := value.(string)
	if !ok {
		return nil, fmt.Errorf("unsupported snapshot text %T", value)
	}
	return &text, nil
}

func snapshotIntegerPointer(value any) *int64 {
	integer := reflect.ValueOf(value).Int()
	return &integer
}

func (report Report) writeEvidence(w *snapshotWriter) {
	report.writeAdvisories(w)
	report.writeStages(w)
	report.writeRoles(w)
	report.writeCorrelations(w)
}
