package files

import (
	"bytes"
	"errors"
	"io"
	"slices"
	"sort"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
)

// The YAML parser decodes a file with go.yaml.in/yaml/v3 into a yaml.Node
// tree, never into Go values, and places versions with the TOML parser's probe
// (toml.go): every token the text parser finds is replaced by a placeholder of
// its own, the copy is decoded once, and a token whose placeholder is now a
// value's whole text is a candidate at that value's key.
//
// A candidate is a scalar that is a value, never a mapping key, written plain,
// single-quoted, or double-quoted, whose tag is !!str, implicit or written
// (`!!str 1.2.3` counts, `!semver 1.2.3` does not), and whose decoded text is
// exactly one placeholder. A placeholder inside a longer scalar (an image
// reference such as ghcr.io/me/app:1.4.2, a range such as ~17.11.3) is not a
// candidate, nor is one in a | or > block scalar however little it holds, a
// comment, or a key. A placeholder is digits and dots like the token it
// stands for, and a scalar holding one has at least two dots, which no YAML
// number or date has, so it resolves to !!str just as the version does: the
// probe never changes a value's type. A value under a key that is not a scalar
// (`? [a, b]`) has no path a key could name and is never a candidate.
//
// Decoding into a yaml.Node keeps an alias as an alias node rather than a copy
// of what it names, so a billion-laughs document decodes at once, and an alias
// is never a candidate: it has no bytes of its own to rewrite. An anchored
// version (`base: &v 1.2.3`) is one candidate at the anchor's key, and
// rewriting it changes every alias that reads it, directly or through an
// anchored mapping or sequence holding it; a merge key (`<<: *defaults`) is an
// alias too. Located.Aliases names them for discover's dry run, Rewrite's
// check compares an alias as the anchor it names so their change counts as
// planned, and a key that names an alias, or a path that only exists through
// one, is an error naming where the value is written (Located.Named).
//
// A file is one YAML document. A stream of several, such as a Kubernetes
// manifest, is refused with a *ParseError saying how many, since a key path
// cannot say which document it means. A file with no document at all, empty
// or only comments, holds no candidates.
//
// As in TOML, the probe copy must decode, and once every placeholder is put
// back it must hold exactly the file's nodes (agreesYAML); a copy that does
// not is refused as YAML whose versions incrmit cannot place. The decoder
// limits an implicit key to 1,024 characters, and a placeholder is wider than
// most tokens, so a key holding several tokens can pass the limit in the copy
// alone.

func locateYAML(data []byte) (*Located, error) {
	root, err := decodeYAML(data, false)
	if err != nil {
		return nil, err
	}
	loc := &Located{}
	toks := TextCandidates(data)
	if len(toks) == 0 {
		walkYAML(root, func(path KeyPath, n *yaml.Node, role yamlRole, _ *yaml.Node) {
			if s, ok := yamlString(n, role); ok && isVersion(s) {
				loc.Skipped = append(loc.Skipped, Skipped{Key: slices.Clone(path), Line: n.Line, Value: s})
			}
		})
		return loc, nil
	}

	pr := newProbe(data, yamlTexts(root), toks)
	proot, err := decodeYAML(pr.data, true)
	if err != nil {
		return nil, err
	}
	if !pr.agreesYAML(root, proot) {
		return nil, &ParseError{Parser: ParserYAML, Probe: true, Msg: "masking them changes what the file holds"}
	}
	g := &aliasGraph{
		outer:   make(map[*yaml.Node]*yaml.Node),
		aliases: make(map[*yaml.Node][]yamlAlias),
		at:      make(map[int]*yaml.Node),
		memo:    make(map[*yaml.Node][]KeyPath),
	}
	walkYAML(proot, func(path KeyPath, n *yaml.Node, role yamlRole, outer *yaml.Node) {
		g.add(pr, path, n, outer)
		s, ok := yamlString(n, role)
		if !ok {
			return
		}
		if i, ok := pr.whole[s]; ok {
			c := toks[i]
			c.Key = pr.unmap(path)
			loc.Candidates = append(loc.Candidates, c)
			g.at[c.Start] = outer
			if n.Anchor != "" {
				g.at[c.Start] = n
			}
			return
		}
		if strings.Contains(s, pr.prefix) {
			return // a longer string holding a token
		}
		if isVersion(s) {
			loc.Skipped = append(loc.Skipped, Skipped{Key: pr.unmap(path), Line: n.Line, Value: s})
		}
	})
	sort.Slice(loc.Candidates, func(i, j int) bool { return loc.Candidates[i].Start < loc.Candidates[j].Start })
	if g.order > 0 {
		loc.aliases = g
	}
	return loc, nil
}

// decodeYAML decodes data as one YAML document and returns the document's root
// node, or nil for a stream with no document (an empty file, or one of
// comments). A stream of more than one document is a *ParseError with Docs
// set. probe marks an error as the probe copy's.
func decodeYAML(data []byte, probe bool) (*yaml.Node, error) {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	var root *yaml.Node
	docs := 0
	for {
		var doc yaml.Node
		err := dec.Decode(&doc)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, yamlParseError(err, probe)
		}
		docs++
		if docs == 1 && len(doc.Content) > 0 {
			root = doc.Content[0]
		}
	}
	if docs > 1 {
		return nil, &ParseError{Parser: ParserYAML, Docs: docs, Probe: probe}
	}
	return root, nil
}

// yamlParseError renders a decoding error as a *ParseError. The decoder's
// errors are strings such as "yaml: line 3: did not find expected key", whose
// line is where the construct around the problem began, at times the line
// before the problem itself, and which a problem on the first line has none
// of. A placeholder holds no line break, so a probe copy's lines are the
// file's.
func yamlParseError(err error, probe bool) *ParseError {
	msg := strings.TrimPrefix(err.Error(), "yaml: ")
	pe := &ParseError{Parser: ParserYAML, Msg: msg, Probe: probe}
	if rest, ok := strings.CutPrefix(msg, "line "); ok {
		if n, m, ok := strings.Cut(rest, ": "); ok {
			if line, err := strconv.Atoi(n); err == nil && line > 0 {
				pe.Line, pe.Msg = line, m
			}
		}
	}
	return pe
}

// yamlRole is where walkYAML found a node.
type yamlRole int

const (
	// yamlValue is a node a key path names: the document's root, a
	// sequence's item, or a mapping's value under a scalar key.
	yamlValue yamlRole = iota
	// yamlKey is a mapping's key. A scalar key is a step of its value's
	// path.
	yamlKey
	// yamlUnnamed is a node no key path names: one inside a key that is not
	// a scalar, or a value under such a key.
	yamlUnnamed
)

// walkYAML calls visit for root and every node below it in document order,
// keys included, each mapping's key before its value. An alias is visited and
// not followed, so a document of aliases costs what its text does. outer is
// the innermost anchored node holding n, not counting n itself, or nil. A
// scalar key adds its text to its value's path, and a node that is not a
// yamlValue gets the path of the mapping holding it. The path is reused
// between calls: visit copies it to keep it. The decoder refuses nesting past
// 10,000 levels, which bounds the recursion.
func walkYAML(root *yaml.Node, visit func(path KeyPath, n *yaml.Node, role yamlRole, outer *yaml.Node)) {
	var walk func(n *yaml.Node, path KeyPath, role yamlRole, outer *yaml.Node)
	walk = func(n *yaml.Node, path KeyPath, role yamlRole, outer *yaml.Node) {
		visit(path, n, role, outer)
		if n.Anchor != "" {
			outer = n
		}
		named := role == yamlValue
		switch n.Kind {
		case yaml.MappingNode:
			for i := 0; i+1 < len(n.Content); i += 2 {
				k, v := n.Content[i], n.Content[i+1]
				switch {
				case !named:
					walk(k, path, yamlUnnamed, outer)
					walk(v, path, yamlUnnamed, outer)
				case k.Kind == yaml.ScalarNode:
					walk(k, path, yamlKey, outer)
					walk(v, append(path, keyStep(k.Value)), yamlValue, outer)
				default:
					walk(k, path, yamlKey, outer)
					walk(v, path, yamlUnnamed, outer)
				}
			}
		case yaml.SequenceNode:
			for i, item := range n.Content {
				if named {
					walk(item, append(path, indexStep(i)), yamlValue, outer)
				} else {
					walk(item, path, yamlUnnamed, outer)
				}
			}
		}
	}
	if root != nil {
		walk(root, nil, yamlValue, nil)
	}
}

// yamlString returns the text of a node that has the shape of a candidate: a
// value, a scalar written plain or in quotes, tagged !!str.
func yamlString(n *yaml.Node, role yamlRole) (string, bool) {
	if role != yamlValue || n.Kind != yaml.ScalarNode || isBlockScalar(n) || n.ShortTag() != "!!str" {
		return "", false
	}
	return n.Value, true
}

func isBlockScalar(n *yaml.Node) bool {
	return n.Style&(yaml.LiteralStyle|yaml.FoldedStyle) != 0
}

// yamlTexts returns every scalar and tag a YAML document decodes to, keys
// included, for absentDigits: an escape in a double-quoted scalar or a tag
// can decode to digits the file does not hold.
func yamlTexts(root *yaml.Node) []string {
	var texts []string
	walkYAML(root, func(_ KeyPath, n *yaml.Node, _ yamlRole, _ *yaml.Node) {
		texts = append(texts, n.Value, n.Tag)
	})
	return texts
}

// The values yamlLeaves gives a node besides a string, which is a value with
// the shape of a candidate, and a container.
type (
	// yamlScalar is any other scalar.
	yamlScalar struct{ tag, value string }
	// yamlKeyLeaf is a scalar key, whose text is the last step of the
	// leaf's path.
	yamlKeyLeaf struct{ tag string }
	// yamlAliasLeaf is an alias, compared as the leaf of the node it names,
	// by that leaf's index.
	yamlAliasLeaf struct{ at int }
)

// yamlLeaves calls emit for every node of a YAML document in walkYAML's order,
// with the path a flattened leaf names it by and what it holds, for Rewrite's
// check (flattenYAML) and the probe's (agreesYAML). The leaf's path is path,
// with key appended when key is not nil, which a scalar key's leaf has; path
// is reused between calls. A value with the shape of a candidate holds its
// string, so the check can match it with a planned rewrite. An alias holds the
// index of the leaf it names rather than that node's value, so the aliases of
// a rewritten anchor change with it as planned, and a document of aliases
// flattens to what its text holds. text is applied to every scalar and tag,
// to put back a probe copy's placeholders; the path is left as decoded.
func yamlLeaves(root *yaml.Node, text func(string) string, emit func(path KeyPath, key *KeyStep, v any)) {
	n := 0
	at := make(map[*yaml.Node]int)
	walkYAML(root, func(path KeyPath, node *yaml.Node, role yamlRole, _ *yaml.Node) {
		var key *KeyStep
		var v any
		switch {
		case node.Kind == yaml.MappingNode:
			v = container{kind: "{", n: len(node.Content) / 2}
		case node.Kind == yaml.SequenceNode:
			v = container{kind: "[", n: len(node.Content)}
		case node.Kind == yaml.AliasNode:
			// An anchor comes before every alias of it, so the walk has
			// recorded it.
			v = yamlAliasLeaf{at: at[node.Alias]}
		case role == yamlKey:
			step := keyStep(node.Value)
			key = &step
			v = yamlKeyLeaf{tag: text(node.ShortTag())}
		default:
			if s, ok := yamlString(node, role); ok {
				v = text(s)
			} else {
				v = yamlScalar{tag: text(node.ShortTag()), value: text(node.Value)}
			}
		}
		if node.Anchor != "" {
			at[node] = n
		}
		n++
		emit(path, key, v)
	})
}

// flattenYAML decodes data and flattens it for Rewrite's check.
func flattenYAML(data []byte) ([]leaf, error) {
	root, err := decodeYAML(data, false)
	if err != nil {
		return nil, err
	}
	var leaves []leaf
	yamlLeaves(root, func(s string) string { return s }, func(path KeyPath, key *KeyStep, v any) {
		if key != nil {
			path = append(slices.Clip(path), *key)
		}
		leaves = append(leaves, leaf{path: path.String(), value: v})
	})
	return leaves, nil
}

// yamlShape is a node as agreesYAML compares it: the length of its path, the
// path's last step, and what it holds. Nodes come in walkYAML's order, in
// which a node's path is its last step appended to the path of the nearest
// node before it one step shorter, so two documents whose shapes agree
// node for node agree on every path, and comparing them costs nothing per
// step of depth.
type yamlShape struct {
	depth int
	last  KeyStep
	value any
}

func yamlShapes(root *yaml.Node, text func(string) string) []yamlShape {
	var shapes []yamlShape
	yamlLeaves(root, text, func(path KeyPath, key *KeyStep, v any) {
		sh := yamlShape{depth: len(path), value: v}
		if len(path) > 0 {
			sh.last = path[len(path)-1]
		}
		if key != nil {
			sh.depth, sh.last = sh.depth+1, *key
		}
		sh.last.Name = text(sh.last.Name)
		shapes = append(shapes, sh)
	})
	return shapes
}

// agreesYAML reports whether proot, the decoded copy, holds exactly root's
// nodes, in the same order and at the same paths, once every placeholder is
// put back, in keys, scalars, and tags.
func (pr *probe) agreesYAML(root, proot *yaml.Node) bool {
	back := func(s string) string {
		if strings.Contains(s, pr.prefix) {
			return pr.back.Replace(s)
		}
		return s
	}
	return slices.EqualFunc(yamlShapes(root, func(s string) string { return s }), yamlShapes(proot, back), func(a, b yamlShape) bool {
		return a.depth == b.depth && a.last == b.last && leafEqual(a.value, b.value)
	})
}

// aliasGraph is what Located.Aliases reads in a YAML file: every anchored node
// with the innermost anchored node holding it, the aliases naming each, and
// the innermost anchored node holding each candidate, the candidate itself
// included.
type aliasGraph struct {
	outer   map[*yaml.Node]*yaml.Node
	aliases map[*yaml.Node][]yamlAlias
	at      map[int]*yaml.Node // by the candidate's Start
	memo    map[*yaml.Node][]KeyPath
	order   int // the aliases seen so far
}

// yamlAlias is one alias: its path, its place in document order, and the
// innermost anchored node holding it.
type yamlAlias struct {
	path  KeyPath
	order int
	outer *yaml.Node
}

// add records n, which walkYAML found at path inside outer, in a probe copy.
func (g *aliasGraph) add(pr *probe, path KeyPath, n *yaml.Node, outer *yaml.Node) {
	if n.Anchor != "" {
		g.outer[n] = outer
	}
	if n.Kind == yaml.AliasNode {
		g.aliases[n.Alias] = append(g.aliases[n.Alias], yamlAlias{path: pr.unmap(path), order: g.order, outer: outer})
		g.order++
	}
}

// Aliases returns the paths of the aliases that read c's value, in document
// order: in a YAML file, every alias of c's own anchor or of an anchored node
// holding c, and every alias of an anchored node holding one of those, and so
// on, since rewriting c changes what each of them reads. An alias in a key, or
// under a key that is not a scalar, is named by the path of the mapping
// holding it. It is nil when no alias reads c, and in any other format.
func (l *Located) Aliases(c Candidate) []KeyPath {
	if l.aliases == nil {
		return nil
	}
	return l.aliases.of(c.Start)
}

// of returns the paths of the aliases that read the candidate starting at
// start. It climbs from the innermost anchored node holding the candidate
// through the anchored nodes holding that one, collecting each one's aliases,
// and from each alias climbs the same way. A climb stops at the first node an
// earlier climb has passed, since that one went on from there to the top, so
// every anchored node is climbed past once and each alias found once: the work
// is what the answer holds, however the aliases nest or cycle.
func (g *aliasGraph) of(start int) []KeyPath {
	from := g.at[start]
	if from == nil {
		return nil
	}
	if paths, ok := g.memo[from]; ok {
		return paths
	}
	seen := make(map[*yaml.Node]bool)
	var found []yamlAlias
	climbs := []*yaml.Node{from}
	for len(climbs) > 0 {
		n := climbs[len(climbs)-1]
		climbs = climbs[:len(climbs)-1]
		for ; n != nil && !seen[n]; n = g.outer[n] {
			seen[n] = true
			for _, a := range g.aliases[n] {
				found = append(found, a)
				climbs = append(climbs, a.outer)
			}
		}
	}
	sort.Slice(found, func(i, j int) bool { return found[i].order < found[j].order })
	var paths []KeyPath
	for _, a := range found {
		paths = append(paths, a.path)
	}
	g.memo[from] = paths
	return paths
}

// yamlKeyError returns a *KeyError when k, in a YAML file, names an alias or a
// path the document only has through an alias or a merge key, as a reader of
// its data would follow them, and nil otherwise. values are what k's path
// names without following anything. Either way the value has no bytes of its
// own at k: a bump rewrites a value where it is written, which the error
// names.
func yamlKeyError(root *yaml.Node, k Key, values []any) error {
	var target *yaml.Node
	via := "an alias"
	switch len(values) {
	case 0:
		target, via = resolveYAML(root, k.path)
		if target == nil || via == "" {
			return nil
		}
	case 1:
		n, ok := values[0].(*yaml.Node)
		if !ok || n.Kind != yaml.AliasNode {
			return nil
		}
		target = n.Alias
	default:
		return nil
	}
	where, ok := yamlPathOf(root, target)
	if !ok {
		return &KeyError{Key: k, Msg: "reads its value through " + via + ", from a node no key can name; incrmit rewrites a value where it is written"}
	}
	return &KeyError{Key: k, Msg: "reads " + displayPath(where.String()) + " through " + via + ", and incrmit rewrites a value where it is written; name " + displayPath(where.String()) + " instead"}
}

// resolveYAML follows path from root as a reader of the document's data would,
// through aliases and merge keys, and returns the node it ends at and what it
// went through: "an alias" when it followed one, else "a merge key" when it
// took a key from one, else "". It returns nil when the data holds nothing at
// path.
func resolveYAML(root *yaml.Node, path KeyPath) (*yaml.Node, string) {
	n, via := root, ""
	deref := func() {
		if n != nil && n.Kind == yaml.AliasNode {
			n, via = n.Alias, "an alias"
		}
	}
	for _, s := range path {
		deref()
		switch {
		case n == nil:
			return nil, ""
		case s.Index >= 0:
			if n.Kind != yaml.SequenceNode || s.Index >= len(n.Content) {
				return nil, ""
			}
			n = n.Content[s.Index]
		case n.Kind == yaml.MappingNode:
			var merged string
			n, merged = yamlLookup(n, s.Name, make(map[*yaml.Node]bool))
			if merged != "" && via != "an alias" {
				via = merged
			}
		default:
			return nil, ""
		}
	}
	deref()
	if n == nil {
		return nil, ""
	}
	return n, via
}

// yamlLookup returns the value of key name in mapping m: m's own, or else the
// first one its merge keys bring in, as YAML's merge key type orders them,
// and "an alias" or "a merge key" for how it was brought in. seen stops a
// merge that leads back to a mapping already searched.
func yamlLookup(m *yaml.Node, name string, seen map[*yaml.Node]bool) (*yaml.Node, string) {
	if seen[m] {
		return nil, ""
	}
	seen[m] = true
	for i := 0; i+1 < len(m.Content); i += 2 {
		if k := m.Content[i]; k.Kind == yaml.ScalarNode && k.Value == name {
			return m.Content[i+1], ""
		}
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		if k := m.Content[i]; k.Kind != yaml.ScalarNode || k.ShortTag() != "!!merge" {
			continue
		}
		srcs := []*yaml.Node{m.Content[i+1]}
		if srcs[0].Kind == yaml.SequenceNode {
			srcs = srcs[0].Content
		}
		for _, src := range srcs {
			via := "a merge key"
			if src.Kind == yaml.AliasNode {
				src, via = src.Alias, "an alias"
			}
			if src.Kind != yaml.MappingNode {
				continue
			}
			if v, inner := yamlLookup(src, name, seen); v != nil {
				if inner == "an alias" {
					via = inner
				}
				return v, via
			}
		}
	}
	return nil, ""
}

// yamlPathOf returns the path of target in the document root, if target is a
// node a key path names.
func yamlPathOf(root, target *yaml.Node) (KeyPath, bool) {
	var found KeyPath
	ok := false
	walkYAML(root, func(path KeyPath, n *yaml.Node, role yamlRole, _ *yaml.Node) {
		if !ok && n == target && role == yamlValue {
			found, ok = slices.Clone(path), true
		}
	})
	return found, ok
}

// describeYAML says what a YAML value that is not a candidate holds, for a
// KeyError.
func describeYAML(n *yaml.Node) string {
	switch n.Kind {
	case yaml.MappingNode:
		return "holds a mapping, not a version"
	case yaml.SequenceNode:
		return "holds a sequence, not a version"
	}
	switch tag := n.ShortTag(); {
	case tag == "!!str" && isBlockScalar(n):
		return "holds a block scalar; incrmit rewrites a version written plain or in quotes"
	case tag == "!!str":
		return "holds " + quoteClipped(n.Value) + ", which is not a version"
	case tag == "!!int" || tag == "!!float":
		return "holds a number; a version is a string"
	case tag == "!!bool":
		return "holds a boolean, not a version"
	case tag == "!!null":
		return "holds null, not a version"
	case tag == "!!timestamp":
		return "holds a date or time, not a version"
	default:
		return "holds " + quoteClipped(n.Value) + " tagged " + tag + "; a version is a string, untagged or tagged !!str"
	}
}
