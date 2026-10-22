// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Marek Cingel
package opentlv

import (
	"fmt"
	"github.com/OpenTLV/opentlv-go/internal/capi"
	"runtime"
	"sync"
)

// QueryType identifies a closed native expression or binding category.
type QueryType int

const (
	QueryNodes QueryType = iota
	QueryBoolean
	QueryInteger
	QueryBytes
	QueryString
)

// ProgramError preserves the Query category, related expression span, named limit
// and the conversion or Reader cause. Each value comes from exactly one C field:
// Diagnostic holds the common evidence, including the Reader detail when HasReader
// is true, and Location is the primary location. Expression is the related
// expression span when another location is primary; otherwise its Kind is
// LocationUnknown, because the primary Location already is that span.
type ProgramError struct {
	Diagnostic
	HasReader bool
	StatusError
	Kind        QueryErrorKind
	Expression  Location
	Configured  uint64
	Limit       string
	CodecDetail *CodecDetail
}

// Error renders the canonical result, Query category and primary evidence.
func (e *ProgramError) Error() string {
	return fmt.Sprintf("%s (%s) at %s", e.StatusError.Error(), e.Kind, e.Location)
}

func (e *ProgramError) Unwrap() error { return e.StatusError }
func programError(code capi.Code, d capi.ProgramDiagnostic) error {
	if code == capi.OK {
		return nil
	}
	if code == capi.InvalidState {
		d.Kind = capi.QueryErrorState
	}
	d.Common.Code = code
	e := d.Expression
	return &ProgramError{Diagnostic: publicDiagnostic(d.Common), HasReader: d.HasReader, StatusError: StatusError{code: code}, Kind: QueryErrorKind(d.Kind),
		Expression: Location{Domain: LocationDomain(e.Domain), Kind: LocationKind(e.Kind), Begin: e.Begin, End: e.End},
		Configured: d.Configured, Limit: d.Limit, CodecDetail: publicCodecDetail(d.CodecDetail)}
}

// SourceOffset projects a known primary input location; ok is false for any
// other domain, such as an expression span or a Value-local location.
func (e *ProgramError) SourceOffset() (offset uint64, ok bool) {
	if e.Location.Domain != LocationInput || e.Location.Kind == LocationUnknown {
		return 0, false
	}
	return e.Location.Begin, true
}

// ProgramOptions declares immutable compile-time names, types and explicit bounds.
// Format must be valid; Optimize is an explicit choice, including in a zero options value.
type ProgramOptions struct {
	Format                                                                Format
	Variables                                                             map[string]QueryType
	Names                                                                 map[string][]byte
	Providers                                                             map[QueryConversion]QueryProvider
	Tags                                                                  *QueryTagAdapter
	Resolver                                                              QueryResolver
	Optimize                                                              bool
	MaxText, MaxTokens, MaxNesting, MaxStates, MaxPattern, MaxResolvedTag int
}

// QueryConversion selects one of the four closed native conversion functions.
type QueryConversion int

const (
	QueryNum QueryConversion = iota
	QueryBCD
	QueryText
	QueryDate
)

// QueryMetadata copies candidate metadata; nil identifies a literal/binding conversion.
type QueryMetadata struct {
	Tag, Value    []byte
	Kind          int
	Depth, Offset uint64
}

// QueryProvider replaces one builtin with a stable nonzero ID and bounded UTF-8 scratch.
// Decode returns QueryInteger or QueryString and a shared OpenTLV error (nil for success).
// Callbacks must be safe for concurrent independent executions. Their allocations
// and work are outside C engine contracts; panics become Callback diagnostics.
type QueryProvider struct {
	ID             uint32
	MaxResultBytes int
	Decode         func([]byte, *QueryMetadata) (QueryValue, error)
}

// QueryRule composes an immutable context selector and relative boolean assertion.
type QueryRule struct {
	Context, Assertion *QueryProgram
	Name               string
}

// QuerySchemaLimits bounds retained descriptors, selected contexts and native work.
// ValueCapacity=-1 measures a Document snapshot; nonnegative values are hard bounds.
type QuerySchemaLimits struct{ Depth, Nodes, Work, Contexts, ValueCapacity int }

// DefaultQuerySchemaLimits supplies finite owning-facade convenience bounds.
func DefaultQuerySchemaLimits() QuerySchemaLimits {
	return QuerySchemaLimits{Depth: 64, Nodes: 1024, Work: 100000000, Contexts: 1024, ValueCapacity: -1}
}

// QuerySchemaDetail is the Schema detail of a false contextual assertion.
type QuerySchemaDetail struct {
	Kind  SchemaIssue
	Field string
	Tag   []byte
}

// QuerySchemaError owns a rule-aware native Query/Schema failure. Schema is the
// Schema detail of a failed assertion and nil for any other failure; the common
// evidence, such as the location, path and expected text, stays in Failure.
type QuerySchemaError struct {
	Rule    int
	Schema  *QuerySchemaDetail
	Failure *ProgramError
}

func (e *QuerySchemaError) Error() string { return e.Failure.Error() }
func (e *QuerySchemaError) Unwrap() error { return e.Failure }

func retainQueryRules(rules []QueryRule) ([]capi.QueryRule, func(), error) {
	type pin struct {
		state  *programState
		native *capi.Program
	}
	var pins []pin
	release := func() {
		for _, item := range pins {
			item.state.mutex.Lock()
			item.native.Close()
			item.state.mutex.Unlock()
		}
	}
	result := make([]capi.QueryRule, 0, len(rules))
	for _, rule := range rules {
		var owners [2]*capi.Program
		for index, p := range []*QueryProgram{rule.Context, rule.Assertion} {
			if err := p.lock(); err != nil {
				release()
				return nil, func() {}, err
			}
			owners[index] = p.state.native.Retain()
			pins = append(pins, pin{p.state, owners[index]})
			p.state.mutex.Unlock()
		}
		result = append(result, capi.QueryRule{Context: owners[0], Assertion: owners[1], Name: rule.Name})
	}
	return result, release, nil
}
func validateQuerySchema(rules []QueryRule, input []byte, document *Document, format Format,
	limits QuerySchemaLimits) error {
	if !format.Valid() {
		return programError(capi.InvalidArg, capi.ProgramDiagnostic{})
	}
	retained, release, err := retainQueryRules(rules)
	if err != nil {
		return err
	}
	defer release()
	var nativeDocument *capi.Document
	if document != nil {
		if !document.valid() {
			return programError(capi.InvalidState, capi.ProgramDiagnostic{})
		}
		document.queryActive++
		defer func() { document.queryActive-- }()
		nativeDocument = document.native
	}
	code, detail := capi.ValidateQuerySchema(retained, input, nativeDocument, format.native,
		capi.QuerySchemaLimits{Depth: limits.Depth, Nodes: limits.Nodes, Work: limits.Work,
			Contexts: limits.Contexts, ValueCapacity: limits.ValueCapacity})
	if code == capi.OK {
		return nil
	}
	failure := programError(code, detail.Query).(*ProgramError)
	result := &QuerySchemaError{Rule: detail.Rule, Failure: failure}
	if detail.HasSchema {
		result.Schema = &QuerySchemaDetail{Kind: SchemaIssue(detail.Kind), Field: detail.Field, Tag: detail.Tag}
	}
	return result
}

// ValidateQueryBuffer delegates contextual assertions to C over a copied complete
// input. D-only rules are rejected before traversal; empty selections succeed.
func ValidateQueryBuffer(rules []QueryRule, input []byte, format Format, limits QuerySchemaLimits) error {
	return validateQuerySchema(rules, input, nil, format, limits)
}

// ValidateQueryDocument validates an immutable Document revision, including D.
// Close and edits during providers are rejected; programs stay retained for the call.
func ValidateQueryDocument(rules []QueryRule, document *Document, limits QuerySchemaLimits) error {
	if !document.valid() {
		return programError(capi.InvalidState, capi.ProgramDiagnostic{})
	}
	return validateQuerySchema(rules, nil, document, document.format, limits)
}

type programState struct {
	mutex  sync.Mutex
	native *capi.Program
	format Format
}

// QueryProgram owns an immutable C program. Close prevents new executions; existing
// independent executions retain their own native reference. Copies share Close state.
type QueryProgram struct{ state *programState }

// CompileQuery delegates full syntax, typing and optimization to C, including embedded NUL diagnostics.
func CompileQuery(text string, options ProgramOptions) (*QueryProgram, error) {
	return buildProgram([]byte(text), options, false)
}

// LoadQuery validates and owns a copy of a same-release internal image with original compile options.
func LoadQuery(image []byte, options ProgramOptions) (*QueryProgram, error) {
	return buildProgram(image, options, true)
}
func buildProgram(data []byte, options ProgramOptions, image bool) (*QueryProgram, error) {
	if !options.Format.Valid() {
		return nil, programError(capi.InvalidArg, capi.ProgramDiagnostic{})
	}
	variables := map[string]int{}
	for name, kind := range options.Variables {
		variables[name] = int(kind)
	}
	providers := map[int]capi.QueryProvider{}
	for selector, provider := range options.Providers {
		if provider.Decode == nil {
			return nil, programError(capi.InvalidArg, capi.ProgramDiagnostic{})
		}
		decode := provider.Decode
		providers[int(selector)] = capi.QueryProvider{ID: provider.ID, Capacity: provider.MaxResultBytes,
			Decode: func(input []byte, metadata *capi.QueryMetadata) (capi.ProviderResult, int) {
				var public *QueryMetadata
				if metadata != nil {
					public = &QueryMetadata{Tag: metadata.Tag, Value: metadata.Value, Kind: metadata.Kind, Depth: metadata.Depth, Offset: metadata.Offset}
				}
				value, status := decode(input, public)
				return capi.ProviderResult{Type: int(value.Type), Integer: value.Integer, Text: value.String}, int(queryCallbackCode(status))
			}}
	}
	var tags *capi.QueryTagAdapter
	if options.Tags != nil {
		tags = &capi.QueryTagAdapter{ID: options.Tags.ID}
		if callback := options.Tags.Class; callback != nil {
			tags.Class = func(tag []byte) (int64, capi.Code) { value, err := callback(tag); return value, queryCallbackCode(err) }
		}
		if callback := options.Tags.Number; callback != nil {
			tags.Number = func(tag []byte) (int64, capi.Code) { value, err := callback(tag); return value, queryCallbackCode(err) }
		}
	}
	var resolver capi.QueryResolver
	if callback := options.Resolver; callback != nil {
		resolver = func(namespace, name string) ([]byte, capi.Code) {
			tag, err := callback(namespace, name)
			return tag, queryCallbackCode(err)
		}
	}
	native, code, diag := options.Format.native.CompileProgram(data, capi.ProgramOptions{Variables: variables, Names: options.Names, Tags: tags, Resolver: resolver,
		Providers: providers,
		Optimize:  options.Optimize, MaxText: options.MaxText, MaxTokens: options.MaxTokens, MaxNesting: options.MaxNesting,
		MaxStates: options.MaxStates, MaxPattern: options.MaxPattern, MaxResolvedTag: options.MaxResolvedTag}, image)
	if err := programError(code, diag); err != nil {
		return nil, err
	}
	result := &QueryProgram{state: &programState{native: native, format: options.Format}}
	return result, nil
}

// Close releases this program reference, once. It is nil-safe.
func (p *QueryProgram) Close() {
	if p == nil || p.state == nil {
		return
	}
	p.state.mutex.Lock()
	defer p.state.mutex.Unlock()
	if p.state.native != nil {
		p.state.native.Close()
		p.state.native = nil
	}
	runtime.SetFinalizer(p, nil)
}
func (p *QueryProgram) lock() error {
	if p == nil || p.state == nil {
		return programError(capi.InvalidState, capi.ProgramDiagnostic{})
	}
	p.state.mutex.Lock()
	if p.state.native == nil {
		p.state.mutex.Unlock()
		return programError(capi.InvalidState, capi.ProgramDiagnostic{})
	}
	return nil
}

// Info copies all native compiler resource requirements; its map can be modified independently.
func (p *QueryProgram) Info() (map[string]uint64, error) {
	if err := p.lock(); err != nil {
		return nil, err
	}
	defer p.state.mutex.Unlock()
	return p.state.native.Info(), nil
}

// Variables copies unique referenced names/types; unused declarations are excluded.
func (p *QueryProgram) Variables() (map[string]QueryType, error) {
	if err := p.lock(); err != nil {
		return nil, err
	}
	defer p.state.mutex.Unlock()
	values, code := p.state.native.Variables()
	result := map[string]QueryType{}
	for name, kind := range values {
		result[name] = QueryType(kind)
	}
	return result, programError(code, capi.ProgramDiagnostic{})
}

// Format returns canonical Query spelling from C.
func (p *QueryProgram) Format() (string, error) { return p.render(false) }

// Explain returns implementation-specific native plan details.
func (p *QueryProgram) Explain() (string, error) { return p.render(true) }
func (p *QueryProgram) render(explain bool) (string, error) {
	if err := p.lock(); err != nil {
		return "", err
	}
	defer p.state.mutex.Unlock()
	value, code := p.state.native.Render(explain)
	return value, programError(code, capi.ProgramDiagnostic{})
}

// Image returns a copied version-limited native-layout image, not persistent serialization.
func (p *QueryProgram) Image() ([]byte, error) {
	if err := p.lock(); err != nil {
		return nil, err
	}
	defer p.state.mutex.Unlock()
	return p.state.native.Image(), nil
}

// QueryLimits bounds all published nodes, nesting and charged native work.
type QueryLimits struct{ Depth, Nodes, Work int }

// DefaultQueryLimits returns bounded wrapper convenience limits; callers can replace them explicitly.
func DefaultQueryLimits() QueryLimits { return QueryLimits{Depth: 64, Nodes: 1024, Work: 100000000} }

// QueryExecution is mutable exclusive state. Serialize access; it copies windows and
// bindings into C memory, and returns owned Go snapshots. Close is deterministic.
type QueryExecution struct {
	native     *capi.ProgramExecution
	program    *programState
	document   *Document
	busy       bool
	fed        bool
	resultType QueryType
}

// Execution chooses bounded S0/S1 or explicit retained S0-S2/D storage, without backend fallback.
func (p *QueryProgram) Execution(limits QueryLimits, retained bool) (*QueryExecution, error) {
	if err := p.lock(); err != nil {
		return nil, err
	}
	defer p.state.mutex.Unlock()
	native, code := p.state.native.Execution(limits.Depth, limits.Nodes, limits.Work, retained)
	if code != capi.OK {
		return nil, programError(code, capi.ProgramDiagnostic{})
	}
	q := &QueryExecution{native: native, program: p.state, resultType: QueryType(p.state.native.Info()["result_kind"])}
	runtime.SetFinalizer(q, func(value *QueryExecution) { _ = value.Close() })
	return q, nil
}
func (q *QueryExecution) check() error {
	if q == nil || q.native == nil || q.busy || (q.document != nil && !q.document.valid()) {
		return programError(capi.InvalidState, capi.ProgramDiagnostic{})
	}
	return nil
}

// Close releases workspace, pinned C windows and native program reference. Callback reentry is rejected.
func (q *QueryExecution) Close() error {
	if q == nil || q.native == nil {
		return nil
	}
	if q.busy {
		return programError(capi.InvalidState, capi.ProgramDiagnostic{})
	}
	q.program.mutex.Lock()
	defer q.program.mutex.Unlock()
	q.native.Close()
	q.native = nil
	q.document = nil
	runtime.SetFinalizer(q, nil)
	return nil
}

// Reset clears terminal/suspended state, bindings and retained windows/snapshot.
func (q *QueryExecution) Reset() error {
	if q == nil || q.native == nil || q.busy {
		return programError(capi.InvalidState, capi.ProgramDiagnostic{})
	}
	code := q.native.Reset()
	if code == capi.OK {
		q.document = nil
		q.fed = false
	}
	return programError(code, capi.ProgramDiagnostic{})
}

// SetInput copies an initial or legally replaced complete-extent window. Old bytes remain pinned until reset/close.
func (q *QueryExecution) SetInput(data []byte, discard int, final bool) error {
	if err := q.check(); err != nil {
		return err
	}
	return programError(q.native.SetInput(data, discard, final), capi.ProgramDiagnostic{})
}

// QueryValue is a tagged owning scalar. Type distinguishes false, zero and empty spans.
type QueryValue struct {
	Type    QueryType
	Boolean bool
	Integer int64
	Bytes   []byte
	String  string
}

// Bind copies a typed integer/bytes/UTF-8 string before input; it never interpolates text.
func (q *QueryExecution) Bind(name string, value QueryValue) error {
	if err := q.check(); err != nil {
		return err
	}
	data := value.Bytes
	if value.Type == QueryString {
		data = []byte(value.String)
	}
	code, diag := q.native.Bind(name, int(value.Type), value.Integer, data)
	return programError(code, diag)
}

// QueryMatch owns copied node bytes and traversal metadata.
type QueryMatch struct {
	Element       Element
	Depth, Offset uint64
	Constructed   bool
}

// QueryEventKind identifies a canonical structural publication.
type QueryEventKind int

const (
	QueryBegin QueryEventKind = iota
	QueryElement
	QueryEnd
)

// QueryEvent carries a complete source-less node. END has no Tag/Value.
// Feed copies its bytes; Source-dependent expressions report missing metadata.
type QueryEvent struct {
	Kind          QueryEventKind
	Tag, Value    []byte
	Depth, Offset int
	Skipped       bool
}

// Feed publishes canonical events without a Reader. Immediate selections return
// a match; retained results become available through Next after Finish.
func (q *QueryExecution) Feed(event QueryEvent) (*QueryMatch, error) {
	if err := q.check(); err != nil {
		return nil, err
	}
	if event.Depth < 0 || event.Offset < 0 || event.Kind < QueryBegin || event.Kind > QueryEnd {
		return nil, programError(capi.InvalidArg, capi.ProgramDiagnostic{})
	}
	q.busy = true
	defer func() { q.busy = false }()
	item, code, diagnostic := q.native.Feed(int(event.Kind), event.Tag, event.Value, event.Depth, event.Offset, event.Skipped)
	if err := programError(code, diagnostic); err != nil {
		return nil, err
	}
	q.fed = true
	if item == nil {
		return nil, nil
	}
	return &QueryMatch{Element: NewElement(item.Tag, item.Value), Depth: item.Depth, Offset: item.Offset, Constructed: item.Constructed}, nil
}

// FeedEncoded copies and decodes exactly one encoded node using this program's
// Format, preserving complete C Source metadata. Constructed nodes open a scope;
// publish descendants and close it with Feed(QueryEnd) in canonical order.
func (q *QueryExecution) FeedEncoded(data []byte, depth, offset int) (*QueryMatch, error) {
	if err := q.check(); err != nil {
		return nil, err
	}
	if depth < 0 || offset < 0 {
		return nil, programError(capi.InvalidArg, capi.ProgramDiagnostic{})
	}
	q.busy = true
	defer func() { q.busy = false }()
	item, code, diagnostic := q.native.FeedEncoded(data, depth, offset)
	if err := programError(code, diagnostic); err != nil {
		return nil, err
	}
	q.fed = true
	if item == nil {
		return nil, nil
	}
	return &QueryMatch{Element: NewElement(item.Tag, item.Value), Depth: item.Depth, Offset: item.Offset, Constructed: item.Constructed}, nil
}

// Finish drains Reader input or finalizes balanced raw events through C.
func (q *QueryExecution) Finish() error {
	if err := q.check(); err != nil {
		return err
	}
	q.busy = true
	defer func() { q.busy = false }()
	code, diagnostic := q.native.Finish()
	return programError(code, diagnostic)
}

// QueryVisit controls callback continuation through the C engine.
type QueryVisit int

const (
	QueryContinue QueryVisit = iota
	QueryStop
	QueryVisitorError
)

// Visit emits ordered matches; STOP/NEED_MORE_DATA preserve continuation. Panics resume only after C returns.
func (q *QueryExecution) Visit(visitor func(QueryMatch) QueryVisit) error {
	if err := q.check(); err != nil {
		return err
	}
	if visitor == nil {
		return programError(capi.NullArg, capi.ProgramDiagnostic{})
	}
	q.busy = true
	defer func() { q.busy = false }()
	code, diag := q.native.Visit(func(match capi.ProgramMatch) int {
		return int(visitor(QueryMatch{Element: NewElement(match.Tag, match.Value), Depth: match.Depth, Offset: match.Offset, Constructed: match.Constructed}))
	})
	return programError(code, diag)
}

// Next pulls one owned Tree match, returning ErrEnd at final exhaustion.
func (q *QueryExecution) Next() (QueryMatch, error) {
	if q == nil || q.resultType != QueryNodes {
		return QueryMatch{}, programError(capi.InvalidArg, capi.ProgramDiagnostic{})
	}
	if q.fed {
		if err := q.check(); err != nil {
			return QueryMatch{}, err
		}
		item, code := q.native.NextResult()
		return QueryMatch{Element: NewElement(item.Tag, item.Value), Depth: item.Depth, Offset: item.Offset, Constructed: item.Constructed}, programError(code, capi.ProgramDiagnostic{})
	}
	var result QueryMatch
	found := false
	err := q.Visit(func(match QueryMatch) QueryVisit { result = match; found = true; return QueryStop })
	if err == nil && !found {
		err = programError(capi.End, capi.ProgramDiagnostic{})
	}
	return result, err
}

// NextResultWithOrdinal pulls a finalized retained Tree result and its original
// preorder identity, which Context accepts on the same input in another execution.
// It shares the native result cursor with Next; call Finish before using it.
func (q *QueryExecution) NextResultWithOrdinal() (QueryMatch, uint64, error) {
	if err := q.check(); err != nil {
		return QueryMatch{}, 0, err
	}
	if q.document != nil || q.resultType != QueryNodes {
		return QueryMatch{}, 0, programError(capi.InvalidArg, capi.ProgramDiagnostic{})
	}
	item, ordinal, code := q.native.NextResultWithOrdinal()
	return QueryMatch{Element: NewElement(item.Tag, item.Value), Depth: item.Depth, Offset: item.Offset, Constructed: item.Constructed}, ordinal, programError(code, capi.ProgramDiagnostic{})
}

// Exists drains by default; early success leaves explicit partial validation coverage.
func (q *QueryExecution) Exists(early bool) (bool, error) {
	if err := q.check(); err != nil {
		return false, err
	}
	q.busy = true
	defer func() { q.busy = false }()
	found, code, diag := q.native.Exists(early)
	return found, programError(code, diag)
}

// Info reports deterministic work counters and structural validation coverage.
func (q *QueryExecution) Info() (map[string]uint64, error) {
	if err := q.check(); err != nil {
		return nil, err
	}
	value, code := q.native.Info()
	return value, programError(code, capi.ProgramDiagnostic{})
}

// Context selects a relative zero-based preorder identity on fresh execution.
func (q *QueryExecution) Context(ordinal int) error {
	if err := q.check(); err != nil {
		return err
	}
	if ordinal < 0 {
		return programError(capi.InvalidArg, capi.ProgramDiagnostic{})
	}
	return programError(q.native.Control(ordinal, false), capi.ProgramDiagnostic{})
}

// Pruning explicitly permits proven skipped subtrees with partial validation coverage.
func (q *QueryExecution) Pruning(enabled bool) error {
	if err := q.check(); err != nil {
		return err
	}
	return programError(q.native.Control(-1, enabled), capi.ProgramDiagnostic{})
}

// Result copies a finalized scalar; node programs use Next/NextDocument.
func (q *QueryExecution) Result() (QueryValue, error) {
	if err := q.check(); err != nil {
		return QueryValue{}, err
	}
	kind, number, data, code := q.native.Result()
	if code == capi.OK && QueryType(kind) == QueryNodes {
		code = capi.InvalidArg
	}
	return QueryValue{Type: QueryType(kind), Boolean: number != 0, Integer: number, Bytes: data, String: string(data)}, programError(code, capi.ProgramDiagnostic{})
}

// EvaluateDocument finalizes a retained execution against a Document revision.
// Negative capacity explicitly measures the canonical snapshot; nonnegative capacity is a hard bound.
func (q *QueryExecution) EvaluateDocument(document *Document, context Node, valueCapacity int) error {
	if err := q.check(); err != nil {
		return err
	}
	q.busy = true
	defer func() { q.busy = false }()
	if !document.valid() {
		return programError(capi.InvalidState, capi.ProgramDiagnostic{})
	}
	if context.owner != nil && (!context.Valid() || context.owner.documentState != document.documentState) {
		return programError(capi.InvalidArg, capi.ProgramDiagnostic{})
	}
	if valueCapacity < 0 {
		var code capi.Code
		valueCapacity, code = document.native.Encode(document.format.native, nil, true)
		if code != capi.OK {
			return programError(code, capi.ProgramDiagnostic{})
		}
	}
	q.document = document
	document.queryActive++
	defer func() { document.queryActive-- }()
	code, diag := q.native.Document(document.native, context.native, valueCapacity)
	return programError(code, diag)
}

// QueryEditKind selects a completed-selection edit with native ancestor dominance.
type QueryEditKind int

const (
	QueryRemove QueryEditKind = iota
	QueryReplace
	QueryInsertAfter
)

// EditDocument mutates an already finalized selection with explicit target capacity.
// Return applied even on failure: commit-time allocation errors may be partial.
// A short target array preserves the native selection for retry; successful edits
// invalidate existing Query results and erased checked Nodes.
func (q *QueryExecution) EditDocument(kind QueryEditKind, tag, value []byte, targetCapacity int) (int, error) {
	if err := q.check(); err != nil {
		return 0, err
	}
	if q.document == nil || q.document.queryActive > 0 {
		return 0, programError(capi.InvalidState, capi.ProgramDiagnostic{})
	}
	if targetCapacity < 0 {
		return 0, programError(capi.InvalidArg, capi.ProgramDiagnostic{})
	}
	q.busy = true
	defer func() { q.busy = false }()
	applied, code, diag := q.native.EditDocument(q.document.native, int(kind), tag, value, targetCapacity)
	if applied > 0 {
		q.document.changed()
	}
	return applied, programError(code, diag)
}

// NextDocument returns a checked snapshot handle; edits are rejected by native revision validation.
func (q *QueryExecution) NextDocument() (Node, error) {
	if err := q.check(); err != nil {
		return Node{}, err
	}
	if q.document == nil {
		return Node{}, programError(capi.InvalidArg, capi.ProgramDiagnostic{})
	}
	node, code, diag := q.native.NextDocument()
	if code != capi.OK {
		return Node{}, programError(code, diag)
	}
	return q.document.wrap(node), nil
}
