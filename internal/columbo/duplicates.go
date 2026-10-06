package columbo

import (
	"fmt"
	"sort"

	"github.com/mibk/dupl/suffixtree"
	"github.com/mibk/dupl/syntax"
	"github.com/mibk/dupl/syntax/golang"
)

type duplicateIndex struct {
	engine    *engine
	tree      *suffixtree.STree
	nodes     []*syntax.Node
	files     map[string]*file
	groups    map[string][]Source
	threshold int
}

func (a *engine) duplicates() error {
	if a.config.Severity["duplicate-code"] == "off" {
		return nil
	}
	index := &duplicateIndex{engine: a, tree: suffixtree.New(), files: map[string]*file{}, groups: map[string][]Source{}, threshold: int(a.config.Counts["duplicate-tokens"])}
	if err := index.build(); err != nil {
		return err
	}
	index.find()
	return index.report()
}

func (index *duplicateIndex) build() error {
	for _, f := range index.engine.files {
		if f.included {
			if err := index.addFile(f); err != nil {
				return err
			}
		}
	}
	index.tree.Update(&syntax.Node{Type: -1})
	return nil
}

func (index *duplicateIndex) addFile(f *file) error {
	root, err := golang.Parse(f.path)
	if err != nil {
		return fmt.Errorf("duplicate-code: %w", err)
	}
	index.files[f.path] = f
	index.appendSyntax(root)
	return nil
}

func (index *duplicateIndex) appendSyntax(root *syntax.Node) {
	for _, node := range syntax.Serialize(root) {
		index.nodes = append(index.nodes, node)
		index.tree.Update(node)
	}
}

func (index *duplicateIndex) find() {
	for match := range index.tree.FindDuplOver(index.threshold) {
		units := syntax.FindSyntaxUnits(index.nodes, match, index.threshold)
		for _, fragment := range units.Frags {
			if source, ok := index.source(fragment); ok {
				index.groups[units.Hash] = append(index.groups[units.Hash], source)
			}
		}
	}
}

func (index *duplicateIndex) source(fragment []*syntax.Node) (Source, bool) {
	first, last := fragment[0], fragment[len(fragment)-1]
	f := index.files[first.Filename]
	owner := index.owner(f, first.Pos, last.End)
	if owner == nil {
		return Source{}, false
	}
	tokens := 0
	for _, node := range fragment {
		tokens += node.Owns + 1
	}
	return owner.source("duplicate-fragment", f.tf.Pos(first.Pos), f.tf.Pos(last.End), Detail{Subject: owner.symbol, Value: tokens}), true
}

func (index *duplicateIndex) owner(f *file, start, end int) *declaration {
	for _, d := range index.engine.declarations {
		if d.file == f && f.tf.Offset(d.fn.Pos()) < end && f.tf.Offset(d.fn.End()) > start {
			return d
		}
	}
	return nil
}

func uniqueDuplicateSources(sources []Source) []Source {
	sort.Slice(sources, func(i, j int) bool { return sourceLess(sources[i], sources[j]) })
	unique := []Source{}
	for _, source := range sources {
		if len(unique) == 0 || source.File != unique[len(unique)-1].File || source.StartOffset != unique[len(unique)-1].StartOffset {
			unique = append(unique, source)
		}
	}
	return unique
}

func (index *duplicateIndex) report() error {
	for _, key := range sortedDuplicateKeys(index.groups) {
		sources := uniqueDuplicateSources(index.groups[key])
		if len(sources) >= 2 {
			if err := index.reportGroup(key, sources); err != nil {
				return err
			}
		}
	}
	return nil
}

func sortedDuplicateKeys(groups map[string][]Source) []string {
	keys := make([]string, 0, len(groups))
	for key := range groups {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func (index *duplicateIndex) reportGroup(key string, sources []Source) error {
	first := sources[0]
	id, raw := identity("duplicate-code", first.File, first.Detail.Subject, key)
	if err := index.engine.claimIdentity(id, raw); err != nil {
		return err
	}
	c := caseFromSource("duplicate-code", index.engine.config.Severity["duplicate-code"], first)
	c.ID = id
	c.duplicateEvidence(sources, index.threshold)
	index.engine.report.Cases = append(index.engine.report.Cases, *c)
	return nil
}

func (c *Case) duplicateEvidence(sources []Source, threshold int) {
	c.Receipts = []any{}
	appendSources(c, sources)
	c.Clues = append(c.Clues,
		c.metric("duplicate-tokens", sources[0].Detail.Value).compare(threshold, ">=").supportedBy(sources),
		c.metric("duplicate-fragments", len(sources)).supportedBy(sources))
}
