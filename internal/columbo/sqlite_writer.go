package columbo

import (
	"database/sql"
	"fmt"
	"math"
	"sort"
)

type snapshotClusterKey struct{ caseID, key string }
type snapshotMemberKey struct {
	caseID string
	ref    MemberRef
}
type snapshotReceiptKey struct{ caseID, key string }

// A snapshotWriter owns the transaction and the first error. Once an operation
// fails, subsequent inserts become no-ops and the transaction is rolled back.
type snapshotWriter struct {
	tx           *sql.Tx
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

func newSnapshotWriter(tx *sql.Tx) *snapshotWriter {
	return &snapshotWriter{tx: tx, files: map[string]int64{}, declarations: map[DeclarationRef]int64{}, dependencies: map[string]int64{}, clusters: map[snapshotClusterKey]int64{}, members: map[snapshotMemberKey]int64{}, receipts: map[snapshotReceiptKey]int64{}, commits: map[string]int64{}, policies: map[string]PolicyReview{}}
}
func (w *snapshotWriter) write(report Report, version string) error {
	w.exec("INSERT INTO report VALUES (1, ?, ?)", SchemaVersion, version)
	w.writeDeclarations(report.Declarations)
	for ordinal, c := range report.Cases {
		w.writeCase(c, ordinal)
	}
	w.writeSuppressions(report.Suppressions)
	w.writeWarnings(report.Warnings)
	return w.err
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
func (w *snapshotWriter) exec(query string, args ...any) {
	if w.err != nil {
		return
	}
	_, err := w.tx.Exec(query, args...)
	w.record(err)
}
func (w *snapshotWriter) insert(query string, args ...any) int64 {
	if w.err != nil {
		return 0
	}
	result, err := w.tx.Exec(query, args...)
	w.record(err)
	if err != nil {
		return 0
	}
	id, err := result.LastInsertId()
	w.record(err)
	return id
}
func (w *snapshotWriter) file(path string) int64 {
	if id, ok := w.files[path]; ok {
		return id
	}
	id := w.insert("INSERT INTO files(path) VALUES (?)", path)
	w.files[path] = id
	return id
}
func (w *snapshotWriter) declaration(ref DeclarationRef) int64 {
	id, ok := w.declarations[ref]
	w.require(ok, "missing typed declaration "+ref.Symbol+" in "+ref.File)
	return id
}
func (w *snapshotWriter) optionalDeclaration(ref *DeclarationRef) any {
	if ref == nil {
		return nil
	}
	return w.declaration(*ref)
}
func (w *snapshotWriter) dependency(identity string) int64 {
	if id, ok := w.dependencies[identity]; ok {
		return id
	}
	id := w.insert("INSERT INTO dependencies(identity) VALUES (?)", identity)
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
	s := d.Source
	w.require(s.File == d.Ref.File, "declaration source file does not match typed reference")
	id := w.insert("INSERT INTO declarations(file_id,symbol,start_line,end_line,start_offset,end_offset) VALUES (?,?,?,?,?,?)", file, d.Ref.Symbol, s.StartLine, s.EndLine, s.StartOffset, s.EndOffset)
	w.declarations[d.Ref] = id
	w.writeDeclarationDependencies(id, d.Dependencies)
}
func (w *snapshotWriter) writeDeclarationDependencies(declarationID int64, evidence []DependencyEvidence) {
	ordered := append([]DependencyEvidence{}, evidence...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].Identity < ordered[j].Identity })
	for _, dep := range ordered {
		dependencyID := w.dependency(dep.Identity)
		w.exec("INSERT INTO declaration_dependencies VALUES (?,?,?)", declarationID, dependencyID, dep.Scored)
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
	w.exec("INSERT INTO cases VALUES (?,?,?,?,?,?,?,?,?,?)", c.ID, ordinal, id, c.Smell, c.Verdict, c.Suppressed, c.StartLine, c.EndLine, c.Why, c.Diagnosis)
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
	w.exec("INSERT OR IGNORE INTO case_declarations VALUES (?,?,?)", caseID, id, role)
}
func (w *snapshotWriter) writeGuidance(c Case) {
	groups := []struct {
		kind  string
		items []string
	}{{"lead", c.Leads}, {"avoid", c.Avoid}}
	for _, group := range groups {
		for ordinal, item := range group.items {
			w.exec("INSERT INTO case_guidance VALUES (?,?,?,?)", c.ID, group.kind, ordinal, item)
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
	id := w.insert("INSERT INTO clusters(case_id,cluster_key,owner_declaration_id,file_id,ordinal) VALUES (?,?,?,?,?)", caseID, cluster.Key, owner, file, ordinal)
	w.clusters[snapshotClusterKey{caseID, cluster.Key}] = id
	for index, member := range cluster.Members {
		w.writeMember(snapshotClusterKey{caseID, cluster.Key}, id, index, member)
	}
}
func (cluster Cluster) snapshotOwner(w *snapshotWriter) any {
	w.require(cluster.OwnerDeclaration != nil, "cluster has no typed owner")
	if cluster.OwnerDeclaration == nil {
		return nil
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
	id := w.insert("INSERT INTO cluster_members(cluster_id,case_id,helper_declaration_id,ordinal,call_offset) VALUES (?,?,?,?,?)", clusterID, cluster.caseID, helper, ordinal, member.CallOffset)
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
func (w *snapshotWriter) writeSource(caseID string, ordinal int, s Source) {
	file := w.file(s.File)
	declarationID := w.optionalDeclaration(s.Declaration)
	numeric := w.number(s.Detail.Value, true)
	nesting, err := snapshotNesting(s.Detail.Nesting)
	w.record(err)
	id := w.insert("INSERT INTO source_receipts(case_id,ordinal,kind,file_id,start_line,end_line,start_offset,end_offset,subject,value_type,value,nesting,source_declaration_id) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)", caseID, ordinal, s.Kind, file, s.StartLine, s.EndLine, s.StartOffset, s.EndOffset, s.Detail.Subject, numeric.kind, numeric.value, nesting, declarationID)
	w.sourceRelations(snapshotReceiptKey{caseID, s.EvidenceKey}, id, s)
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
	declarationID := w.optionalDeclaration(source.Declaration)
	w.require(declarationID != nil, "dependency receipt has no typed declaration")
	dependencyID := w.dependency(source.DependencyIdentity)
	var scored bool
	w.record(w.tx.QueryRow("SELECT scored FROM declaration_dependencies WHERE declaration_id=? AND dependency_id=?", declarationID, dependencyID).Scan(&scored))
	w.require(scored == source.DependencyScored, "dependency receipt disagrees with scored inventory")
	w.exec("INSERT INTO dependency_receipts VALUES (?,?,?,?)", declarationID, dependencyID, id, caseID)
}
func (w *snapshotWriter) writeExpansionDeclarations(receiptID int64, detail Detail) {
	w.require(len(detail.Expansion) == len(detail.ExpansionDeclarations), "expansion declarations lack typed references")
	if w.err != nil {
		return
	}
	for ordinal, ref := range detail.ExpansionDeclarations {
		id := w.declaration(ref)
		w.require(detail.Expansion[ordinal] == ref.Symbol, "expansion name does not match typed declaration")
		w.exec("INSERT INTO receipt_expansion_declarations VALUES (?,?,?,?)", receiptID, ordinal, id, detail.Expansion[ordinal])
	}
}
func (w *snapshotWriter) writeExpansionSites(receiptID int64, sites []Site) {
	for ordinal, site := range sites {
		file := w.file(site.File)
		owner := w.optionalDeclaration(site.Owner)
		w.exec("INSERT INTO receipt_expansion_sites VALUES (?,?,?,?,?)", receiptID, ordinal, file, site.CallOffset, owner)
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
	refs := w.clueReferences(caseID, clue)
	id := w.insert("INSERT INTO clues(case_id,ordinal,kind,subject,value_type,numeric_value,limit_type,limit_value,operator,declaration_id,cluster_id,member_id,pair_left_member_id,pair_right_member_id) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)", caseID, ordinal, clue.Kind, clue.Subject, value.kind, value.value, limit.kind, limit.value, clue.Operator, refs.declaration, refs.cluster, refs.member, refs.left, refs.right)
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
		w.exec("INSERT INTO clue_values VALUES (?,?,?)", id, ordinal, value)
	}
}
func (w *snapshotWriter) writeClueReceipts(caseID string, clueID int64, keys []string) {
	for _, key := range keys {
		receiptID, ok := w.receipts[snapshotReceiptKey{caseID, key}]
		w.require(ok, "clue references missing typed receipt")
		w.exec("INSERT INTO clue_receipts VALUES (?,?,?)", caseID, clueID, receiptID)
	}
}

type snapshotClueReferences struct{ declaration, cluster, member, left, right any }

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
func (w *snapshotWriter) clueCluster(caseID, key string) int64 {
	id, ok := w.clusters[snapshotClusterKey{caseID, key}]
	w.require(ok, "clue references missing typed cluster")
	return id
}
func (w *snapshotWriter) clueMember(caseID string, ref MemberRef) int64 {
	id, ok := w.members[snapshotMemberKey{caseID, ref}]
	w.require(ok, "clue references missing typed cluster member")
	return id
}
func (w *snapshotWriter) cluePair(caseID string, pair []MemberRef) (any, any) {
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
	w.exec("INSERT INTO case_history VALUES (?,?,?,?)", caseID, ordinal, history.Commit, history.Kind)
	for index, path := range history.Files {
		file := w.file(path)
		w.exec("INSERT INTO case_history_files VALUES (?,?,?,?)", caseID, ordinal, index, file)
	}
}
func (w *snapshotWriter) writeCommit(history History) {
	if stamp, ok := w.commits[history.Commit]; ok {
		w.require(stamp == history.CommittedAt, "conflicting commit timestamps")
		return
	}
	w.exec("INSERT INTO commits VALUES (?,?)", history.Commit, history.CommittedAt)
	w.commits[history.Commit] = history.CommittedAt
}
func (w *snapshotWriter) writePolicies(c Case) {
	for ordinal, policy := range c.PolicyReviews {
		w.policy(policy)
		w.exec("INSERT INTO case_policy_reviews VALUES (?,?,?)", c.ID, ordinal, policy.ID)
	}
}
func (w *snapshotWriter) policy(policy PolicyReview) {
	if previous, ok := w.policies[policy.ID]; ok {
		w.require(previous == policy, "conflicting policy review "+policy.ID)
		return
	}
	w.exec("INSERT INTO policy_reviews VALUES (?,?,?)", policy.ID, policy.Note, policy.ReviewPrompt)
	w.policies[policy.ID] = policy
}
func (w *snapshotWriter) writeSuppressions(suppressions []Suppression) {
	for ordinal, suppression := range suppressions {
		suppression.writeSnapshot(w, ordinal)
	}
}
func (s Suppression) writeSnapshot(w *snapshotWriter, ordinal int) {
	file := w.file(s.File)
	w.exec("INSERT INTO suppressions(ordinal,smell,symbol,file_id,line,justification,applied,case_id) VALUES (?,?,?,?,?,?,?,?)", ordinal, s.Smell, s.Symbol, file, s.Line, s.Justification, s.Applied, s.CaseID)
}
func (w *snapshotWriter) writeWarnings(warnings []Warning) {
	for ordinal, warning := range warnings {
		warning.writeSnapshot(w, ordinal)
	}
}
func (warning Warning) writeSnapshot(w *snapshotWriter, ordinal int) {
	var file any
	if warning.File != "" {
		file = w.file(warning.File)
	}
	w.exec("INSERT INTO warnings(ordinal,code,file_id,line,message) VALUES (?,?,?,?,?)", ordinal, warning.Code, file, warning.Line, warning.Message)
}

type snapshotNumeric struct {
	kind, value any
	items       []string
	err         error
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
func snapshotNesting(value any) (any, error) {
	number, err := snapshotNumericValue(value, true)
	if err != nil {
		return nil, err
	}
	if number.kind != nil && number.kind != "integer" {
		return nil, fmt.Errorf("nesting must be an integer")
	}
	return number.value, nil
}
